package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("RICE_PORT", "9090")
	os.Setenv("RICE_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("RICE_PORT")
		os.Unsetenv("RICE_LOG_LEVEL")
	}()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}

	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %s, want debug", cfg.Log.Level)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
host: "127.0.0.1"
port: 8888
log:
  level: warn
  format: json
qdrant:
  url: "http://custom:6333"
ml:
  device: cuda
  embed_dim: 768
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %s, want 127.0.0.1", cfg.Host)
	}

	if cfg.Port != 8888 {
		t.Errorf("Port = %d, want 8888", cfg.Port)
	}

	if cfg.Log.Level != "warn" {
		t.Errorf("Log.Level = %s, want warn", cfg.Log.Level)
	}

	if cfg.Qdrant.URL != "http://custom:6333" {
		t.Errorf("Qdrant.URL = %s, want http://custom:6333", cfg.Qdrant.URL)
	}

	if cfg.ML.Device != "cuda" {
		t.Errorf("ML.Device = %s, want cuda", cfg.ML.Device)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid defaults",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "invalid port",
			modify: func(c *Config) {
				c.Port = 0
			},
			wantErr: true,
		},
		{
			name: "invalid ML device",
			modify: func(c *Config) {
				c.ML.Device = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			modify: func(c *Config) {
				c.Log.Level = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid cache type",
			modify: func(c *Config) {
				c.Cache.Type = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid bus type",
			modify: func(c *Config) {
				c.Bus.Type = "invalid"
			},
			wantErr: true,
		},
		{
			name: "chunk overlap >= chunk size",
			modify: func(c *Config) {
				c.Index.ChunkSize = 100
				c.Index.ChunkOverlap = 100
			},
			wantErr: true,
		},
		{
			name: "sparse weight out of range",
			modify: func(c *Config) {
				c.Search.DefaultSparseWeight = 1.5
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			setDefaults(cfg)
			tt.modify(cfg)

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddress(t *testing.T) {
	cfg := &Config{
		Host: "localhost",
		Port: 8080,
	}

	if addr := cfg.Address(); addr != "localhost:8080" {
		t.Errorf("Address() = %s, want localhost:8080", addr)
	}
}

func TestIsDevelopment(t *testing.T) {
	cfg := &Config{}

	cfg.Log.Level = "debug"
	if !cfg.IsDevelopment() {
		t.Error("IsDevelopment() = false, want true for debug level")
	}

	cfg.Log.Level = "info"
	if cfg.IsDevelopment() {
		t.Error("IsDevelopment() = true, want false for info level")
	}
}
