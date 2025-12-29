package models

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// MapperService manages model mappers.
type MapperService struct {
	mappers  map[string]*ModelMapper
	storage  Storage
	registry *Registry
	log      *logger.Logger
	mu       sync.RWMutex
}

// NewMapperService creates a new mapper service.
func NewMapperService(storage Storage, registry *Registry, log *logger.Logger) (*MapperService, error) {
	s := &MapperService{
		mappers:  make(map[string]*ModelMapper),
		storage:  storage,
		registry: registry,
		log:      log,
	}

	// Load existing mappers from storage
	if err := s.loadMappers(); err != nil {
		return nil, fmt.Errorf("failed to load mappers: %w", err)
	}

	// Load default mappers if none exist
	if len(s.mappers) == 0 {
		if err := s.loadDefaults(); err != nil {
			return nil, fmt.Errorf("failed to load default mappers: %w", err)
		}
	}

	return s, nil
}

// loadMappers loads all mappers from storage.
func (s *MapperService) loadMappers() error {
	mappers, err := s.storage.LoadAllMappers()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, mapper := range mappers {
		s.mappers[mapper.ID] = mapper
	}

	return nil
}

// loadDefaults loads default mappers into the service.
func (s *MapperService) loadDefaults() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for i := range DefaultMappers {
		mapper := DefaultMappers[i]
		mapper.CreatedAt = now
		mapper.UpdatedAt = now

		s.mappers[mapper.ID] = &mapper

		// Save to storage
		if err := s.storage.SaveMapper(&mapper); err != nil {
			s.log.Warn("Failed to save default mapper", "mapper", mapper.ID, "error", err)
		}
	}

	s.log.Info("Loaded default mappers", "count", len(DefaultMappers))
	return nil
}

// ListMappers returns all mappers.
func (s *MapperService) ListMappers(ctx context.Context) ([]ModelMapper, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ModelMapper, 0, len(s.mappers))
	for _, mapper := range s.mappers {
		result = append(result, *mapper)
	}

	return result, nil
}

// GetMapper retrieves a mapper by ID.
func (s *MapperService) GetMapper(ctx context.Context, id string) (*ModelMapper, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mapper, exists := s.mappers[id]
	if !exists {
		return nil, errors.NotFoundError(fmt.Sprintf("mapper: %s", id))
	}

	// Return copy
	mapperCopy := *mapper
	return &mapperCopy, nil
}

// GetMapperForModel retrieves the mapper for a specific model.
func (s *MapperService) GetMapperForModel(ctx context.Context, modelID string) (*ModelMapper, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, mapper := range s.mappers {
		if mapper.ModelID == modelID {
			mapperCopy := *mapper
			return &mapperCopy, nil
		}
	}

	return nil, errors.NotFoundError(fmt.Sprintf("mapper for model: %s", modelID))
}

// CreateMapper creates a new mapper.
func (s *MapperService) CreateMapper(ctx context.Context, mapper *ModelMapper) error {
	if err := mapper.Validate(); err != nil {
		return errors.ValidationError(err.Error())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	if _, exists := s.mappers[mapper.ID]; exists {
		return errors.AlreadyExistsError(fmt.Sprintf("mapper: %s", mapper.ID))
	}

	// Verify model exists
	if s.registry != nil {
		if _, err := s.registry.GetModel(ctx, mapper.ModelID); err != nil {
			return errors.ValidationError(fmt.Sprintf("model %s not found", mapper.ModelID))
		}
	}

	// Set timestamps
	now := time.Now()
	mapper.CreatedAt = now
	mapper.UpdatedAt = now

	// Save to storage
	if err := s.storage.SaveMapper(mapper); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to save mapper", err)
	}

	s.mappers[mapper.ID] = mapper
	s.log.Info("Created mapper", "mapper", mapper.ID, "model", mapper.ModelID)
	return nil
}

// UpdateMapper updates an existing mapper.
func (s *MapperService) UpdateMapper(ctx context.Context, mapper *ModelMapper) error {
	if err := mapper.Validate(); err != nil {
		return errors.ValidationError(err.Error())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.mappers[mapper.ID]
	if !exists {
		return errors.NotFoundError(fmt.Sprintf("mapper: %s", mapper.ID))
	}

	// Preserve creation time
	mapper.CreatedAt = existing.CreatedAt
	mapper.Touch()

	// Save to storage
	if err := s.storage.SaveMapper(mapper); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to save mapper", err)
	}

	s.mappers[mapper.ID] = mapper
	s.log.Info("Updated mapper", "mapper", mapper.ID)
	return nil
}

// DeleteMapper deletes a mapper.
func (s *MapperService) DeleteMapper(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.mappers[id]; !exists {
		return errors.NotFoundError(fmt.Sprintf("mapper: %s", id))
	}

	// Delete from storage
	if err := s.storage.DeleteMapper(id); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to delete mapper", err)
	}

	delete(s.mappers, id)
	s.log.Info("Deleted mapper", "mapper", id)
	return nil
}

// GenerateMapper auto-generates a mapper for a model based on its type.
func (s *MapperService) GenerateMapper(ctx context.Context, modelID string) (*ModelMapper, error) {
	// Get model info
	var model *ModelInfo
	var err error

	if s.registry != nil {
		model, err = s.registry.GetModel(ctx, modelID)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New(errors.CodeInternal, "registry not available")
	}

	// Generate mapper based on type
	mapper := &ModelMapper{
		ID:      fmt.Sprintf("%s-mapper", modelID),
		Name:    fmt.Sprintf("%s Mapper", model.DisplayName),
		ModelID: modelID,
		Type:    model.Type,
	}

	// Set default mappings based on model type
	switch model.Type {
	case ModelTypeEmbed:
		mapper.InputMapping = map[string]string{
			"text": "text",
		}
		mapper.OutputMapping = map[string]string{
			"embedding": "embedding",
		}

	case ModelTypeRerank:
		mapper.InputMapping = map[string]string{
			"query":    "query",
			"document": "document",
		}
		mapper.OutputMapping = map[string]string{
			"score": "score",
		}

	case ModelTypeQueryUnderstand:
		mapper.InputMapping = map[string]string{
			"query": "text",
		}
		mapper.OutputMapping = map[string]string{
			"intent":     "intent",
			"difficulty": "difficulty",
			"confidence": "confidence",
		}

	default:
		return nil, errors.ValidationError(fmt.Sprintf("unsupported model type: %s", model.Type))
	}

	// Create the mapper
	if err := s.CreateMapper(ctx, mapper); err != nil {
		return nil, err
	}

	return mapper, nil
}
