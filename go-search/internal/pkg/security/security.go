// Package security provides security utilities for input validation,
// sanitization, and sensitive data masking.
package security

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Path validation errors.
var (
	ErrPathEmpty        = &PathError{Reason: "path is empty"}
	ErrPathNullByte     = &PathError{Reason: "path contains null byte"}
	ErrPathTraversal    = &PathError{Reason: "path traversal detected"}
	ErrPathAbsolute     = &PathError{Reason: "absolute path not allowed"}
	ErrPathTooLong      = &PathError{Reason: "path exceeds maximum length"}
	ErrPathInvalidChars = &PathError{Reason: "path contains invalid characters"}
	ErrPathReservedName = &PathError{Reason: "path contains reserved name"}
)

// PathError represents a path validation error.
type PathError struct {
	Reason string
	Path   string
}

func (e *PathError) Error() string {
	if e.Path != "" {
		return e.Reason + ": " + e.Path
	}
	return e.Reason
}

// MaxPathLength is the maximum allowed path length.
const MaxPathLength = 1024

// reservedNames are Windows reserved device names that should not be used as filenames.
var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true,
	"com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true,
	"lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ValidatePath validates a file path for security issues.
// It checks for:
// - Empty paths
// - Null bytes (path injection)
// - Path traversal attempts (../)
// - Absolute paths
// - Maximum length
// - Reserved names (Windows compatibility)
func ValidatePath(path string) error {
	// Check for empty path
	if path == "" {
		return ErrPathEmpty
	}

	// Check for null bytes (path injection attack)
	if strings.Contains(path, "\x00") {
		return &PathError{Reason: ErrPathNullByte.Reason, Path: "[contains null byte]"}
	}

	// Check for maximum length
	if len(path) > MaxPathLength {
		return &PathError{Reason: ErrPathTooLong.Reason, Path: path[:50] + "..."}
	}

	// Check for absolute paths (cross-platform)
	// filepath.IsAbs only works for the current OS, so check both styles
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return &PathError{Reason: ErrPathAbsolute.Reason, Path: SanitizeForLog(path)}
	}

	// Clean and check for path traversal
	cleaned := filepath.Clean(path)

	// After cleaning, check if path tries to go above root
	if strings.HasPrefix(cleaned, "..") {
		return &PathError{Reason: ErrPathTraversal.Reason, Path: SanitizeForLog(path)}
	}

	// Check each component for traversal and reserved names
	parts := strings.Split(filepath.ToSlash(cleaned), "/")
	for _, part := range parts {
		// Skip empty parts
		if part == "" {
			continue
		}

		// Check for path traversal in any component
		if part == ".." {
			return &PathError{Reason: ErrPathTraversal.Reason, Path: SanitizeForLog(path)}
		}

		// Check for reserved names (case-insensitive)
		baseName := strings.ToLower(part)
		// Remove extension for reserved name check
		if idx := strings.Index(baseName, "."); idx > 0 {
			baseName = baseName[:idx]
		}
		if reservedNames[baseName] {
			return &PathError{Reason: ErrPathReservedName.Reason, Path: SanitizeForLog(path)}
		}
	}

	return nil
}

// ValidatePathStrict performs strict path validation, also checking for
// characters that might cause issues on some file systems.
func ValidatePathStrict(path string) error {
	// First do basic validation
	if err := ValidatePath(path); err != nil {
		return err
	}

	// Check for potentially problematic characters
	for _, r := range path {
		// Allow alphanumeric, common punctuation, and path separators
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) &&
			r != '/' && r != '\\' && r != '.' && r != '-' && r != '_' &&
			r != ' ' && r != '@' && r != '+' && r != '=' && r != '(' && r != ')' {
			// Check if it's a control character
			if unicode.IsControl(r) {
				return &PathError{Reason: ErrPathInvalidChars.Reason, Path: SanitizeForLog(path)}
			}
		}
	}

	return nil
}

// SanitizeForLog sanitizes a string for safe logging.
// It prevents log injection by:
// - Replacing newlines with escaped versions
// - Replacing carriage returns
// - Removing other control characters
// - Truncating to a maximum length
func SanitizeForLog(s string) string {
	return SanitizeForLogWithLength(s, 200)
}

// SanitizeForLogWithLength sanitizes a string for logging with a custom max length.
func SanitizeForLogWithLength(s string, maxLen int) string {
	if s == "" {
		return ""
	}

	// Use a builder for efficiency
	var b strings.Builder
	b.Grow(minInt(len(s), maxLen+10))

	count := 0
	for _, r := range s {
		if count >= maxLen {
			b.WriteString("...")
			break
		}

		switch r {
		case '\n':
			b.WriteString("\\n")
			count += 2
		case '\r':
			b.WriteString("\\r")
			count += 2
		case '\t':
			b.WriteString("\\t")
			count += 2
		default:
			// Remove other control characters, keep printable
			if !unicode.IsControl(r) || r == ' ' {
				b.WriteRune(r)
				count++
			}
		}
	}

	return b.String()
}

// sensitiveHeaders are HTTP header names that contain sensitive data.
// These should be masked in logs.
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"x-api-key":           true,
	"api-key":             true,
	"x-auth-token":        true,
	"cookie":              true,
	"set-cookie":          true,
	"x-csrf-token":        true,
	"x-xsrf-token":        true,
	"proxy-authorization": true,
}

// sensitiveFieldPatterns are patterns in header names that indicate sensitive data.
var sensitiveFieldPatterns = []string{
	"password",
	"secret",
	"token",
	"key",
	"credential",
	"auth",
}

// MaskSensitiveHeaders creates a copy of headers with sensitive values masked.
// This is safe to use for logging.
func MaskSensitiveHeaders(headers http.Header) http.Header {
	if headers == nil {
		return nil
	}

	masked := make(http.Header, len(headers))
	for key, values := range headers {
		if isSensitiveHeader(key) {
			masked[key] = []string{"[REDACTED]"}
		} else {
			// Copy values
			masked[key] = append([]string(nil), values...)
		}
	}
	return masked
}

// MaskSensitiveMap masks sensitive values in a string map.
// Useful for logging request parameters or config values.
func MaskSensitiveMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	masked := make(map[string]string, len(m))
	for key, value := range m {
		if isSensitiveKey(key) {
			masked[key] = "[REDACTED]"
		} else {
			masked[key] = value
		}
	}
	return masked
}

// isSensitiveHeader checks if a header name contains sensitive data.
func isSensitiveHeader(name string) bool {
	lower := strings.ToLower(name)

	// Check exact matches
	if sensitiveHeaders[lower] {
		return true
	}

	// Check patterns
	return isSensitiveKey(lower)
}

// isSensitiveKey checks if a key name likely contains sensitive data.
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, pattern := range sensitiveFieldPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// SanitizeQuery sanitizes a search query string.
// It removes control characters while preserving normal whitespace.
func SanitizeQuery(query string) string {
	if query == "" {
		return ""
	}

	// Use strings.Map for efficiency
	sanitized := strings.Map(func(r rune) rune {
		// Keep newlines and tabs (may be intentional in code search)
		if r == '\n' || r == '\t' {
			return r
		}
		// Remove other control characters
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, query)

	return strings.TrimSpace(sanitized)
}

// ValidateContent validates file content for indexing.
// It checks for valid UTF-8 and reasonable size.
func ValidateContent(content string, maxSize int) error {
	if len(content) > maxSize {
		return &ContentError{
			Reason: "content exceeds maximum size",
			Size:   len(content),
			Max:    maxSize,
		}
	}

	if !utf8.ValidString(content) {
		return &ContentError{Reason: "content is not valid UTF-8"}
	}

	return nil
}

// ContentError represents a content validation error.
type ContentError struct {
	Reason string
	Size   int
	Max    int
}

func (e *ContentError) Error() string {
	if e.Size > 0 && e.Max > 0 {
		return fmt.Sprintf("%s (size: %s, max: %s)", e.Reason, formatSize(e.Size), formatSize(e.Max))
	}
	return e.Reason
}

// formatSize formats a byte size as human-readable.
func formatSize(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f%s", float64(bytes)/float64(div), units[exp])
}

// IsBinaryContent checks if content appears to be binary (non-text).
func IsBinaryContent(content string) bool {
	if len(content) == 0 {
		return false
	}

	// Check first 8KB for binary indicators
	checkLen := minInt(len(content), 8192)
	sample := content[:checkLen]

	nullCount := 0
	nonPrintable := 0

	for _, b := range []byte(sample) {
		if b == 0 {
			nullCount++
			// More than a few null bytes strongly indicates binary
			if nullCount > 3 {
				return true
			}
		} else if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			nonPrintable++
		}
	}

	// If more than 10% non-printable, likely binary
	return float64(nonPrintable)/float64(checkLen) > 0.1
}

// minInt returns the smaller of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
