package security

import (
	"net/http"
	"strings"
	"testing"
)

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errType string
	}{
		// Valid paths
		{"valid simple", "file.txt", false, ""},
		{"valid nested", "src/main.go", false, ""},
		{"valid deep", "a/b/c/d/e/f.txt", false, ""},
		{"valid with dots", "file.test.go", false, ""},
		{"valid hidden", ".gitignore", false, ""},
		{"valid current dir", "./file.txt", false, ""},

		// Invalid paths
		{"empty", "", true, "empty"},
		{"null byte", "file\x00.txt", true, "null byte"},
		{"traversal simple", "../file.txt", true, "traversal"},
		{"traversal nested", "src/../../../etc/passwd", true, "traversal"},
		{"traversal hidden", "src/.../file.txt", false, ""}, // ... is not traversal
		{"absolute unix", "/etc/passwd", true, "absolute"},
		{"absolute windows", "C:\\Windows\\System32", true, "absolute"},
		{"reserved con", "con.txt", true, "reserved"},
		{"reserved prn", "folder/prn.doc", true, "reserved"},
		{"reserved aux", "aux", true, "reserved"},
		{"reserved lpt1", "lpt1.txt", true, "reserved"},
		{"too long", strings.Repeat("a", 2000), true, "length"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
			if err != nil && tt.errType != "" {
				if !strings.Contains(err.Error(), tt.errType) {
					t.Errorf("ValidatePath(%q) error = %v, should contain %q", tt.path, err, tt.errType)
				}
			}
		})
	}
}

func TestSanitizeForLog(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"simple", "hello world", "hello world"},
		{"newline", "line1\nline2", "line1\\nline2"},
		{"carriage return", "line1\rline2", "line1\\rline2"},
		{"tab", "col1\tcol2", "col1\\tcol2"},
		{"mixed", "a\nb\rc\td", "a\\nb\\rc\\td"},
		{"control chars", "hello\x00\x01\x02world", "helloworld"},
		{"long string", strings.Repeat("a", 300), strings.Repeat("a", 200) + "..."},
		{"unicode", "hello 世界", "hello 世界"},
		{"log injection", "user\nERROR: fake error", "user\\nERROR: fake error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForLog(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeForLog(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskSensitiveHeaders(t *testing.T) {
	headers := http.Header{
		"Content-Type":  []string{"application/json"},
		"Authorization": []string{"Bearer secret123"},
		"X-Api-Key":     []string{"key123"},
		"X-Request-Id":  []string{"req-456"},
		"Cookie":        []string{"session=abc"},
		"X-Custom-Auth": []string{"should-be-masked"},
	}

	masked := MaskSensitiveHeaders(headers)

	// Check non-sensitive headers are preserved
	if masked.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type should not be masked")
	}
	if masked.Get("X-Request-Id") != "req-456" {
		t.Errorf("X-Request-Id should not be masked")
	}

	// Check sensitive headers are masked
	sensitiveKeys := []string{"Authorization", "X-Api-Key", "Cookie", "X-Custom-Auth"}
	for _, key := range sensitiveKeys {
		if masked.Get(key) != "[REDACTED]" {
			t.Errorf("%s should be masked, got %q", key, masked.Get(key))
		}
	}

	// Check original headers are not modified
	if headers.Get("Authorization") != "Bearer secret123" {
		t.Errorf("Original headers should not be modified")
	}
}

func TestMaskSensitiveMap(t *testing.T) {
	m := map[string]string{
		"username":     "john",
		"password":     "secret123",
		"api_key":      "key123",
		"database_url": "postgres://...",
		"secret_token": "tok123",
	}

	masked := MaskSensitiveMap(m)

	// Check non-sensitive values are preserved
	if masked["username"] != "john" {
		t.Errorf("username should not be masked")
	}
	if masked["database_url"] != "postgres://..." {
		t.Errorf("database_url should not be masked")
	}

	// Check sensitive values are masked
	sensitiveKeys := []string{"password", "api_key", "secret_token"}
	for _, key := range sensitiveKeys {
		if masked[key] != "[REDACTED]" {
			t.Errorf("%s should be masked, got %q", key, masked[key])
		}
	}

	// Check original map is not modified
	if m["password"] != "secret123" {
		t.Errorf("Original map should not be modified")
	}
}

func TestSanitizeQuery(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"simple", "hello world", "hello world"},
		{"with spaces", "  hello  world  ", "hello  world"},
		{"with newline", "hello\nworld", "hello\nworld"},
		{"with tab", "hello\tworld", "hello\tworld"},
		{"control chars", "hello\x00\x01world", "helloworld"},
		{"unicode", "搜索 query", "搜索 query"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeQuery(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeQuery(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		maxSize int
		wantErr bool
	}{
		{"valid small", "hello", 1000, false},
		{"valid at limit", strings.Repeat("a", 100), 100, false},
		{"exceeds limit", strings.Repeat("a", 101), 100, true},
		{"invalid utf8", "hello\xff\xfeworld", 1000, true},
		{"valid unicode", "hello 世界 🌍", 1000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContent(tt.content, tt.maxSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsBinaryContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty", "", false},
		{"text", "Hello, World!", false},
		{"code", "func main() {\n\treturn\n}", false},
		{"with nulls", "hello\x00\x00\x00\x00world", true},
		{"binary data", string(append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 100)...)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBinaryContent(tt.content); got != tt.want {
				t.Errorf("IsBinaryContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaskSensitiveHeaders_Nil(t *testing.T) {
	result := MaskSensitiveHeaders(nil)
	if result != nil {
		t.Errorf("MaskSensitiveHeaders(nil) should return nil")
	}
}

func TestMaskSensitiveMap_Nil(t *testing.T) {
	result := MaskSensitiveMap(nil)
	if result != nil {
		t.Errorf("MaskSensitiveMap(nil) should return nil")
	}
}

func BenchmarkSanitizeForLog(b *testing.B) {
	input := strings.Repeat("hello\nworld\t", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SanitizeForLog(input)
	}
}

func BenchmarkValidatePath(b *testing.B) {
	path := "src/internal/pkg/security/security.go"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidatePath(path)
	}
}

func BenchmarkMaskSensitiveHeaders(b *testing.B) {
	headers := http.Header{
		"Content-Type":  []string{"application/json"},
		"Authorization": []string{"Bearer secret123"},
		"X-Api-Key":     []string{"key123"},
		"X-Request-Id":  []string{"req-456"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MaskSensitiveHeaders(headers)
	}
}
