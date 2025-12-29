package qdrant

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

const (
	// CollectionPrefix is prepended to all collection names.
	CollectionPrefix = "rice_"

	// DefaultHost is the default Qdrant host.
	DefaultHost = "localhost"

	// DefaultPort is the default Qdrant gRPC port.
	DefaultPort = 6334

	// DefaultTimeout is the default operation timeout.
	DefaultTimeout = 30 * time.Second
)

// ClientConfig holds configuration for the Qdrant client.
type ClientConfig struct {
	// Host is the Qdrant server host.
	Host string

	// Port is the Qdrant gRPC port.
	Port int

	// APIKey for authentication (optional).
	APIKey string

	// UseTLS enables TLS connection.
	UseTLS bool

	// Timeout for operations.
	Timeout time.Duration
}

// DefaultClientConfig returns sensible defaults for local development.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Host:    DefaultHost,
		Port:    DefaultPort,
		Timeout: DefaultTimeout,
	}
}

// Client wraps the Qdrant Go client with Rice Search specific operations.
type Client struct {
	client *qdrant.Client
	config ClientConfig
	mu     sync.RWMutex
	closed bool
}

// NewClient creates a new Qdrant client wrapper.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.Host == "" {
		cfg.Host = DefaultHost
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   cfg.Host,
		Port:   cfg.Port,
		APIKey: cfg.APIKey,
		UseTLS: cfg.UseTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create qdrant client: %w", err)
	}

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.client.Close()
}

// HealthCheck verifies the Qdrant server is reachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	reply, err := c.client.HealthCheck(ctx)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if reply.GetTitle() == "" {
		return fmt.Errorf("unexpected health check response")
	}

	return nil
}

// GetVersion returns the Qdrant server version.
func (c *Client) GetVersion(ctx context.Context) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return "", fmt.Errorf("client is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	reply, err := c.client.HealthCheck(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	return reply.GetVersion(), nil
}

// collectionName returns the full collection name with prefix.
func collectionName(name string) string {
	return CollectionPrefix + name
}

// rawClient returns the underlying Qdrant client for advanced operations.
// This should only be used when the wrapper doesn't provide the needed functionality.
func (c *Client) rawClient() *qdrant.Client {
	return c.client
}
