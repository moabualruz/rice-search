package models

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryStorage_Models(t *testing.T) {
	storage := NewMemoryStorage()

	// Test save and load
	model := &ModelInfo{
		ID:          "test/model",
		Type:        ModelTypeEmbed,
		DisplayName: "Test Model",
		MaxTokens:   512,
		OutputDim:   768,
	}

	if err := storage.SaveModel(model); err != nil {
		t.Fatalf("SaveModel() error = %v", err)
	}

	loaded, err := storage.LoadModel(model.ID)
	if err != nil {
		t.Fatalf("LoadModel() error = %v", err)
	}

	if loaded.ID != model.ID {
		t.Errorf("Loaded model ID = %v, want %v", loaded.ID, model.ID)
	}

	// Test load all
	models, err := storage.LoadAllModels()
	if err != nil {
		t.Fatalf("LoadAllModels() error = %v", err)
	}

	if len(models) != 1 {
		t.Errorf("LoadAllModels() count = %v, want 1", len(models))
	}

	// Test exists
	if !storage.ModelExists(model.ID) {
		t.Errorf("ModelExists() = false, want true")
	}

	// Test delete
	if err := storage.DeleteModel(model.ID); err != nil {
		t.Fatalf("DeleteModel() error = %v", err)
	}

	if storage.ModelExists(model.ID) {
		t.Errorf("ModelExists() after delete = true, want false")
	}
}

func TestMemoryStorage_Mappers(t *testing.T) {
	storage := NewMemoryStorage()

	mapper := &ModelMapper{
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
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := storage.SaveMapper(mapper); err != nil {
		t.Fatalf("SaveMapper() error = %v", err)
	}

	loaded, err := storage.LoadMapper(mapper.ID)
	if err != nil {
		t.Fatalf("LoadMapper() error = %v", err)
	}

	if loaded.ID != mapper.ID {
		t.Errorf("Loaded mapper ID = %v, want %v", loaded.ID, mapper.ID)
	}

	// Test load all
	mappers, err := storage.LoadAllMappers()
	if err != nil {
		t.Fatalf("LoadAllMappers() error = %v", err)
	}

	if len(mappers) != 1 {
		t.Errorf("LoadAllMappers() count = %v, want 1", len(mappers))
	}

	// Test delete
	if err := storage.DeleteMapper(mapper.ID); err != nil {
		t.Fatalf("DeleteMapper() error = %v", err)
	}

	if storage.MapperExists(mapper.ID) {
		t.Errorf("MapperExists() after delete = true, want false")
	}
}

func TestMemoryStorage_TypeConfigs(t *testing.T) {
	storage := NewMemoryStorage()

	cfg := &ModelTypeConfig{
		Type:         ModelTypeEmbed,
		DefaultModel: "test/model",
		GPUEnabled:   true,
	}

	if err := storage.SaveTypeConfig(cfg); err != nil {
		t.Fatalf("SaveTypeConfig() error = %v", err)
	}

	loaded, err := storage.LoadTypeConfig(cfg.Type)
	if err != nil {
		t.Fatalf("LoadTypeConfig() error = %v", err)
	}

	if loaded.Type != cfg.Type {
		t.Errorf("Loaded config type = %v, want %v", loaded.Type, cfg.Type)
	}

	// Test load all
	configs, err := storage.LoadAllTypeConfigs()
	if err != nil {
		t.Fatalf("LoadAllTypeConfigs() error = %v", err)
	}

	if len(configs) != 1 {
		t.Errorf("LoadAllTypeConfigs() count = %v, want 1", len(configs))
	}

	// Test delete
	if err := storage.DeleteTypeConfig(cfg.Type); err != nil {
		t.Fatalf("DeleteTypeConfig() error = %v", err)
	}
}

func TestFileStorage_Models(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	model := &ModelInfo{
		ID:          "test/model",
		Type:        ModelTypeEmbed,
		DisplayName: "Test Model",
		MaxTokens:   512,
		OutputDim:   768,
	}

	// Test save
	if err := storage.SaveModel(model); err != nil {
		t.Fatalf("SaveModel() error = %v", err)
	}

	// Verify file was created
	modelsFile := filepath.Join(tmpDir, "models.yaml")
	if _, err := os.Stat(modelsFile); os.IsNotExist(err) {
		t.Errorf("models.yaml not created")
	}

	// Test load
	loaded, err := storage.LoadModel(model.ID)
	if err != nil {
		t.Fatalf("LoadModel() error = %v", err)
	}

	if loaded.ID != model.ID {
		t.Errorf("Loaded model ID = %v, want %v", loaded.ID, model.ID)
	}

	// Test update
	model.DisplayName = "Updated Model"
	if err := storage.SaveModel(model); err != nil {
		t.Fatalf("SaveModel() update error = %v", err)
	}

	loaded, err = storage.LoadModel(model.ID)
	if err != nil {
		t.Fatalf("LoadModel() after update error = %v", err)
	}

	if loaded.DisplayName != "Updated Model" {
		t.Errorf("Updated model name = %v, want Updated Model", loaded.DisplayName)
	}

	// Test delete
	if err := storage.DeleteModel(model.ID); err != nil {
		t.Fatalf("DeleteModel() error = %v", err)
	}

	if storage.ModelExists(model.ID) {
		t.Errorf("ModelExists() after delete = true, want false")
	}
}

func TestFileStorage_Mappers(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	mapper := &ModelMapper{
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
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test save
	if err := storage.SaveMapper(mapper); err != nil {
		t.Fatalf("SaveMapper() error = %v", err)
	}

	// Verify file was created
	mapperFile := filepath.Join(tmpDir, "mappers", mapper.ID+".yaml")
	if _, err := os.Stat(mapperFile); os.IsNotExist(err) {
		t.Errorf("mapper file not created")
	}

	// Test load
	loaded, err := storage.LoadMapper(mapper.ID)
	if err != nil {
		t.Fatalf("LoadMapper() error = %v", err)
	}

	if loaded.ID != mapper.ID {
		t.Errorf("Loaded mapper ID = %v, want %v", loaded.ID, mapper.ID)
	}

	// Test load all
	mappers, err := storage.LoadAllMappers()
	if err != nil {
		t.Fatalf("LoadAllMappers() error = %v", err)
	}

	if len(mappers) != 1 {
		t.Errorf("LoadAllMappers() count = %v, want 1", len(mappers))
	}

	// Test delete
	if err := storage.DeleteMapper(mapper.ID); err != nil {
		t.Fatalf("DeleteMapper() error = %v", err)
	}

	if storage.MapperExists(mapper.ID) {
		t.Errorf("MapperExists() after delete = true, want false")
	}
}

func TestFileStorage_TypeConfigs(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	cfg := &ModelTypeConfig{
		Type:         ModelTypeEmbed,
		DefaultModel: "test/model",
		GPUEnabled:   true,
	}

	// Test save
	if err := storage.SaveTypeConfig(cfg); err != nil {
		t.Fatalf("SaveTypeConfig() error = %v", err)
	}

	// Verify file was created
	configFile := filepath.Join(tmpDir, "type_configs.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Errorf("type_configs.yaml not created")
	}

	// Test load
	loaded, err := storage.LoadTypeConfig(cfg.Type)
	if err != nil {
		t.Fatalf("LoadTypeConfig() error = %v", err)
	}

	if loaded.Type != cfg.Type {
		t.Errorf("Loaded config type = %v, want %v", loaded.Type, cfg.Type)
	}

	// Test update
	cfg.GPUEnabled = false
	if err := storage.SaveTypeConfig(cfg); err != nil {
		t.Fatalf("SaveTypeConfig() update error = %v", err)
	}

	loaded, err = storage.LoadTypeConfig(cfg.Type)
	if err != nil {
		t.Fatalf("LoadTypeConfig() after update error = %v", err)
	}

	if loaded.GPUEnabled {
		t.Errorf("Updated GPU enabled = true, want false")
	}

	// Test delete
	if err := storage.DeleteTypeConfig(cfg.Type); err != nil {
		t.Fatalf("DeleteTypeConfig() error = %v", err)
	}
}
