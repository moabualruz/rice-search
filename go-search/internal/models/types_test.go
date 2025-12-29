package models

import (
	"testing"
	"time"
)

func TestModelType_Valid(t *testing.T) {
	tests := []struct {
		name  string
		mt    ModelType
		valid bool
	}{
		{"embed valid", ModelTypeEmbed, true},
		{"rerank valid", ModelTypeRerank, true},
		{"query_understand valid", ModelTypeQueryUnderstand, true},
		{"invalid", ModelType("invalid"), false},
		{"empty", ModelType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mt.Valid(); got != tt.valid {
				t.Errorf("ModelType.Valid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestModelInfo_Validate(t *testing.T) {
	tests := []struct {
		name    string
		model   ModelInfo
		wantErr bool
	}{
		{
			name: "valid embed model",
			model: ModelInfo{
				ID:          "test/embed",
				Type:        ModelTypeEmbed,
				DisplayName: "Test Embed",
				MaxTokens:   512,
				OutputDim:   768,
			},
			wantErr: false,
		},
		{
			name: "valid rerank model",
			model: ModelInfo{
				ID:          "test/rerank",
				Type:        ModelTypeRerank,
				DisplayName: "Test Rerank",
				MaxTokens:   512,
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			model: ModelInfo{
				Type:        ModelTypeEmbed,
				DisplayName: "Test",
				MaxTokens:   512,
				OutputDim:   768,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			model: ModelInfo{
				ID:          "test/model",
				Type:        ModelType("invalid"),
				DisplayName: "Test",
				MaxTokens:   512,
			},
			wantErr: true,
		},
		{
			name: "embed without output_dim",
			model: ModelInfo{
				ID:          "test/embed",
				Type:        ModelTypeEmbed,
				DisplayName: "Test",
				MaxTokens:   512,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.model.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ModelInfo.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestModelMapper_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mapper  ModelMapper
		wantErr bool
	}{
		{
			name: "valid mapper",
			mapper: ModelMapper{
				ID:      "test-mapper",
				Name:    "Test Mapper",
				ModelID: "test/model",
				Type:    ModelTypeEmbed,
				InputMapping: map[string]string{
					"text": "text",
				},
				OutputMapping: map[string]string{
					"embedding": "embedding",
				},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			mapper: ModelMapper{
				Name:    "Test",
				ModelID: "test/model",
				Type:    ModelTypeEmbed,
				InputMapping: map[string]string{
					"text": "text",
				},
				OutputMapping: map[string]string{
					"embedding": "embedding",
				},
			},
			wantErr: true,
		},
		{
			name: "nil input mapping",
			mapper: ModelMapper{
				ID:      "test",
				Name:    "Test",
				ModelID: "test/model",
				Type:    ModelTypeEmbed,
				OutputMapping: map[string]string{
					"embedding": "embedding",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mapper.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ModelMapper.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestModelMapper_Touch(t *testing.T) {
	mapper := ModelMapper{
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}

	before := mapper.UpdatedAt
	time.Sleep(10 * time.Millisecond)
	mapper.Touch()

	if !mapper.UpdatedAt.After(before) {
		t.Errorf("Touch() did not update timestamp")
	}
}

func TestModelTypeConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ModelTypeConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ModelTypeConfig{
				Type:         ModelTypeEmbed,
				DefaultModel: "test/model",
				GPUEnabled:   true,
			},
			wantErr: false,
		},
		{
			name: "invalid type",
			config: ModelTypeConfig{
				Type:         ModelType("invalid"),
				DefaultModel: "test/model",
			},
			wantErr: true,
		},
		{
			name: "missing default model",
			config: ModelTypeConfig{
				Type: ModelTypeEmbed,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ModelTypeConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
