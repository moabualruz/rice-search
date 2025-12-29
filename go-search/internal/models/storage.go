package models

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Storage is the interface for model and mapper persistence.
type Storage interface {
	// Model operations
	SaveModel(model *ModelInfo) error
	LoadModel(id string) (*ModelInfo, error)
	LoadAllModels() ([]*ModelInfo, error)
	DeleteModel(id string) error
	ModelExists(id string) bool

	// Mapper operations
	SaveMapper(mapper *ModelMapper) error
	LoadMapper(id string) (*ModelMapper, error)
	LoadAllMappers() ([]*ModelMapper, error)
	DeleteMapper(id string) error
	MapperExists(id string) bool

	// Type config operations
	SaveTypeConfig(cfg *ModelTypeConfig) error
	LoadTypeConfig(modelType ModelType) (*ModelTypeConfig, error)
	LoadAllTypeConfigs() ([]*ModelTypeConfig, error)
	DeleteTypeConfig(modelType ModelType) error
}

// MemoryStorage stores data in memory (for testing).
type MemoryStorage struct {
	models      map[string]*ModelInfo
	mappers     map[string]*ModelMapper
	typeConfigs map[ModelType]*ModelTypeConfig
	mu          sync.RWMutex
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		models:      make(map[string]*ModelInfo),
		mappers:     make(map[string]*ModelMapper),
		typeConfigs: make(map[ModelType]*ModelTypeConfig),
	}
}

// Model operations

func (m *MemoryStorage) SaveModel(model *ModelInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	modelCopy := *model
	m.models[model.ID] = &modelCopy
	return nil
}

func (m *MemoryStorage) LoadModel(id string) (*ModelInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	model, exists := m.models[id]
	if !exists {
		return nil, fmt.Errorf("model %s not found", id)
	}

	modelCopy := *model
	return &modelCopy, nil
}

func (m *MemoryStorage) LoadAllModels() ([]*ModelInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	models := make([]*ModelInfo, 0, len(m.models))
	for _, model := range m.models {
		modelCopy := *model
		models = append(models, &modelCopy)
	}
	return models, nil
}

func (m *MemoryStorage) DeleteModel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.models, id)
	return nil
}

func (m *MemoryStorage) ModelExists(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.models[id]
	return exists
}

// Mapper operations

func (m *MemoryStorage) SaveMapper(mapper *ModelMapper) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mapperCopy := *mapper
	m.mappers[mapper.ID] = &mapperCopy
	return nil
}

func (m *MemoryStorage) LoadMapper(id string) (*ModelMapper, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mapper, exists := m.mappers[id]
	if !exists {
		return nil, fmt.Errorf("mapper %s not found", id)
	}

	mapperCopy := *mapper
	return &mapperCopy, nil
}

func (m *MemoryStorage) LoadAllMappers() ([]*ModelMapper, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mappers := make([]*ModelMapper, 0, len(m.mappers))
	for _, mapper := range m.mappers {
		mapperCopy := *mapper
		mappers = append(mappers, &mapperCopy)
	}
	return mappers, nil
}

func (m *MemoryStorage) DeleteMapper(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.mappers, id)
	return nil
}

func (m *MemoryStorage) MapperExists(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.mappers[id]
	return exists
}

// Type config operations

func (m *MemoryStorage) SaveTypeConfig(cfg *ModelTypeConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfgCopy := *cfg
	m.typeConfigs[cfg.Type] = &cfgCopy
	return nil
}

func (m *MemoryStorage) LoadTypeConfig(modelType ModelType) (*ModelTypeConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, exists := m.typeConfigs[modelType]
	if !exists {
		return nil, fmt.Errorf("type config for %s not found", modelType)
	}

	cfgCopy := *cfg
	return &cfgCopy, nil
}

func (m *MemoryStorage) LoadAllTypeConfigs() ([]*ModelTypeConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]*ModelTypeConfig, 0, len(m.typeConfigs))
	for _, cfg := range m.typeConfigs {
		cfgCopy := *cfg
		configs = append(configs, &cfgCopy)
	}
	return configs, nil
}

func (m *MemoryStorage) DeleteTypeConfig(modelType ModelType) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.typeConfigs, modelType)
	return nil
}

// FileStorage stores data in YAML files.
type FileStorage struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStorage creates a new file-based storage.
func NewFileStorage(basePath string) *FileStorage {
	return &FileStorage{
		basePath: basePath,
	}
}

func (f *FileStorage) modelsFile() string {
	return filepath.Join(f.basePath, "models.yaml")
}

func (f *FileStorage) mappersDir() string {
	return filepath.Join(f.basePath, "mappers")
}

func (f *FileStorage) mapperFile(id string) string {
	return filepath.Join(f.mappersDir(), id+".yaml")
}

func (f *FileStorage) typeConfigsFile() string {
	return filepath.Join(f.basePath, "type_configs.yaml")
}

// Model operations

func (f *FileStorage) SaveModel(model *ModelInfo) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Load all models
	models, err := f.loadAllModelsUnlocked()
	if err != nil {
		models = make([]*ModelInfo, 0)
	}

	// Update or append
	found := false
	for i, m := range models {
		if m.ID == model.ID {
			models[i] = model
			found = true
			break
		}
	}
	if !found {
		models = append(models, model)
	}

	// Save all models
	return f.saveAllModelsUnlocked(models)
}

func (f *FileStorage) LoadModel(id string) (*ModelInfo, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	models, err := f.loadAllModelsUnlocked()
	if err != nil {
		return nil, err
	}

	for _, model := range models {
		if model.ID == id {
			return model, nil
		}
	}

	return nil, fmt.Errorf("model %s not found", id)
}

func (f *FileStorage) LoadAllModels() ([]*ModelInfo, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.loadAllModelsUnlocked()
}

func (f *FileStorage) loadAllModelsUnlocked() ([]*ModelInfo, error) {
	modelsFile := f.modelsFile()

	// Check if file exists
	if _, err := os.Stat(modelsFile); os.IsNotExist(err) {
		return []*ModelInfo{}, nil
	}

	data, err := os.ReadFile(modelsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read models file: %w", err)
	}

	var models []*ModelInfo
	if err := yaml.Unmarshal(data, &models); err != nil {
		return nil, fmt.Errorf("failed to unmarshal models: %w", err)
	}

	return models, nil
}

func (f *FileStorage) saveAllModelsUnlocked(models []*ModelInfo) error {
	// Ensure directory exists
	if err := os.MkdirAll(f.basePath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := yaml.Marshal(models)
	if err != nil {
		return fmt.Errorf("failed to marshal models: %w", err)
	}

	if err := os.WriteFile(f.modelsFile(), data, 0644); err != nil {
		return fmt.Errorf("failed to write models file: %w", err)
	}

	return nil
}

func (f *FileStorage) DeleteModel(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	models, err := f.loadAllModelsUnlocked()
	if err != nil {
		return err
	}

	// Filter out the model
	filtered := make([]*ModelInfo, 0, len(models))
	for _, model := range models {
		if model.ID != id {
			filtered = append(filtered, model)
		}
	}

	return f.saveAllModelsUnlocked(filtered)
}

func (f *FileStorage) ModelExists(id string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	models, err := f.loadAllModelsUnlocked()
	if err != nil {
		return false
	}

	for _, model := range models {
		if model.ID == id {
			return true
		}
	}

	return false
}

// Mapper operations

func (f *FileStorage) SaveMapper(mapper *ModelMapper) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Ensure mappers directory exists
	if err := os.MkdirAll(f.mappersDir(), 0755); err != nil {
		return fmt.Errorf("failed to create mappers directory: %w", err)
	}

	data, err := yaml.Marshal(mapper)
	if err != nil {
		return fmt.Errorf("failed to marshal mapper: %w", err)
	}

	if err := os.WriteFile(f.mapperFile(mapper.ID), data, 0644); err != nil {
		return fmt.Errorf("failed to write mapper file: %w", err)
	}

	return nil
}

func (f *FileStorage) LoadMapper(id string) (*ModelMapper, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	data, err := os.ReadFile(f.mapperFile(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("mapper %s not found", id)
		}
		return nil, fmt.Errorf("failed to read mapper file: %w", err)
	}

	var mapper ModelMapper
	if err := yaml.Unmarshal(data, &mapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal mapper: %w", err)
	}

	return &mapper, nil
}

func (f *FileStorage) LoadAllMappers() ([]*ModelMapper, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	mappersDir := f.mappersDir()

	// Check if directory exists
	if _, err := os.Stat(mappersDir); os.IsNotExist(err) {
		return []*ModelMapper{}, nil
	}

	entries, err := os.ReadDir(mappersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read mappers directory: %w", err)
	}

	var mappers []*ModelMapper
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		path := filepath.Join(mappersDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip files we can't read
		}

		var mapper ModelMapper
		if err := yaml.Unmarshal(data, &mapper); err != nil {
			continue // Skip invalid files
		}

		mappers = append(mappers, &mapper)
	}

	return mappers, nil
}

func (f *FileStorage) DeleteMapper(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := f.mapperFile(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete mapper file: %w", err)
	}

	return nil
}

func (f *FileStorage) MapperExists(id string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	_, err := os.Stat(f.mapperFile(id))
	return err == nil
}

// Type config operations

func (f *FileStorage) SaveTypeConfig(cfg *ModelTypeConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Load all configs
	configs, err := f.loadAllTypeConfigsUnlocked()
	if err != nil {
		configs = make([]*ModelTypeConfig, 0)
	}

	// Update or append
	found := false
	for i, c := range configs {
		if c.Type == cfg.Type {
			configs[i] = cfg
			found = true
			break
		}
	}
	if !found {
		configs = append(configs, cfg)
	}

	// Save all configs
	return f.saveAllTypeConfigsUnlocked(configs)
}

func (f *FileStorage) LoadTypeConfig(modelType ModelType) (*ModelTypeConfig, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	configs, err := f.loadAllTypeConfigsUnlocked()
	if err != nil {
		return nil, err
	}

	for _, cfg := range configs {
		if cfg.Type == modelType {
			return cfg, nil
		}
	}

	return nil, fmt.Errorf("type config for %s not found", modelType)
}

func (f *FileStorage) LoadAllTypeConfigs() ([]*ModelTypeConfig, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.loadAllTypeConfigsUnlocked()
}

func (f *FileStorage) loadAllTypeConfigsUnlocked() ([]*ModelTypeConfig, error) {
	configsFile := f.typeConfigsFile()

	// Check if file exists
	if _, err := os.Stat(configsFile); os.IsNotExist(err) {
		return []*ModelTypeConfig{}, nil
	}

	data, err := os.ReadFile(configsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read type configs file: %w", err)
	}

	var configs []*ModelTypeConfig
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal type configs: %w", err)
	}

	return configs, nil
}

func (f *FileStorage) saveAllTypeConfigsUnlocked(configs []*ModelTypeConfig) error {
	// Ensure directory exists
	if err := os.MkdirAll(f.basePath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := yaml.Marshal(configs)
	if err != nil {
		return fmt.Errorf("failed to marshal type configs: %w", err)
	}

	if err := os.WriteFile(f.typeConfigsFile(), data, 0644); err != nil {
		return fmt.Errorf("failed to write type configs file: %w", err)
	}

	return nil
}

func (f *FileStorage) DeleteTypeConfig(modelType ModelType) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	configs, err := f.loadAllTypeConfigsUnlocked()
	if err != nil {
		return err
	}

	// Filter out the config
	filtered := make([]*ModelTypeConfig, 0, len(configs))
	for _, cfg := range configs {
		if cfg.Type != modelType {
			filtered = append(filtered, cfg)
		}
	}

	return f.saveAllTypeConfigsUnlocked(filtered)
}
