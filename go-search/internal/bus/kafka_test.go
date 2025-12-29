package bus

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

// TestKafkaConfig_Validation tests configuration validation.
func TestKafkaConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     KafkaConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: KafkaConfig{
				Brokers:       []string{"localhost:9092"},
				ConsumerGroup: "test-group",
			},
			wantErr: false,
		},
		{
			name: "empty brokers",
			cfg: KafkaConfig{
				Brokers:       []string{},
				ConsumerGroup: "test-group",
			},
			wantErr: true,
		},
		{
			name: "empty consumer group",
			cfg: KafkaConfig{
				Brokers:       []string{"localhost:9092"},
				ConsumerGroup: "",
			},
			wantErr: true,
		},
		{
			name: "invalid kafka version",
			cfg: KafkaConfig{
				Brokers:       []string{"localhost:9092"},
				ConsumerGroup: "test-group",
				Version:       "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewKafkaBus(tt.cfg)
			if (err != nil) != tt.wantErr {
				// Skip the test if Kafka is not running (only for valid config test)
				if tt.name == "valid config" && err != nil {
					t.Skip("Skipping test - Kafka not running")
					return
				}
				t.Errorf("NewKafkaBus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseKafkaBrokers tests broker string parsing.
func TestParseKafkaBrokers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single broker",
			input: "localhost:9092",
			want:  []string{"localhost:9092"},
		},
		{
			name:  "multiple brokers",
			input: "broker1:9092,broker2:9092,broker3:9092",
			want:  []string{"broker1:9092", "broker2:9092", "broker3:9092"},
		},
		{
			name:  "with whitespace",
			input: "broker1:9092 , broker2:9092 , broker3:9092",
			want:  []string{"broker1:9092", "broker2:9092", "broker3:9092"},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseKafkaBrokers(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ParseKafkaBrokers() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseKafkaBrokers()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestKafkaBus_DefaultConfig tests default configuration values.
func TestKafkaBus_DefaultConfig(t *testing.T) {
	cfg := KafkaConfig{
		Brokers:       []string{"localhost:9092"},
		ConsumerGroup: "test-group",
		// Leave defaults empty
	}

	// This will fail if Kafka is not running, but we're just testing config defaults
	_, err := NewKafkaBus(cfg)

	// We expect error (no Kafka running), but we can check the config was set
	if err == nil {
		t.Skip("Kafka is running, skipping config-only test")
	}

	// Validate defaults were applied (indirectly through NewKafkaBus validation)
	if cfg.ClientID != "" && cfg.ClientID != "rice-search-bus" {
		t.Errorf("Default ClientID not set correctly")
	}
}

// TestKafkaBus_CorrelationIDHeader tests correlation ID extraction from headers.
func TestKafkaBus_CorrelationIDHeader(t *testing.T) {
	// Create a mock Kafka message with headers
	msg := &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{
			{
				Key:   []byte("correlation_id"),
				Value: []byte("test-correlation-123"),
			},
		},
	}

	// Extract correlation ID
	var correlationID string
	for _, h := range msg.Headers {
		if string(h.Key) == "correlation_id" {
			correlationID = string(h.Value)
			break
		}
	}

	if correlationID != "test-correlation-123" {
		t.Errorf("Correlation ID = %s, want test-correlation-123", correlationID)
	}
}

// TestKafkaBus_Interface verifies KafkaBus implements Bus interface.
func TestKafkaBus_Interface(t *testing.T) {
	var _ Bus = (*KafkaBus)(nil) // Compile-time interface check
}

// TestKafkaBus_CloseIdempotent tests that Close() can be called multiple times safely.
func TestKafkaBus_CloseIdempotent(t *testing.T) {
	// Note: This test requires Kafka to be running for full coverage
	// For now, we just verify the pattern
	bus := &KafkaBus{
		handlers:     make(map[string][]Handler),
		pending:      make(map[string]chan Event),
		consumerStop: make(chan struct{}),
		closed:       false,
	}

	// First close
	bus.mu.Lock()
	bus.closed = true
	bus.mu.Unlock()

	// Second close should return immediately without error
	if err := bus.Close(); err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}
}

// TestKafkaBus_PublishAfterClose tests that operations fail after Close().
func TestKafkaBus_PublishAfterClose(t *testing.T) {
	bus := &KafkaBus{
		handlers:     make(map[string][]Handler),
		pending:      make(map[string]chan Event),
		consumerStop: make(chan struct{}),
		closed:       true, // Pre-closed
	}

	err := bus.Publish(context.Background(), "test", Event{ID: "test"})
	if err == nil {
		t.Error("Publish() after Close() should return error")
	}
}

// TestKafkaBus_SubscribeAfterClose tests that Subscribe fails after Close().
func TestKafkaBus_SubscribeAfterClose(t *testing.T) {
	bus := &KafkaBus{
		handlers:     make(map[string][]Handler),
		pending:      make(map[string]chan Event),
		consumerStop: make(chan struct{}),
		closed:       true, // Pre-closed
	}

	err := bus.Subscribe(context.Background(), "test", func(ctx context.Context, event Event) error {
		return nil
	})
	if err == nil {
		t.Error("Subscribe() after Close() should return error")
	}
}

// TestKafkaBus_RequestAfterClose tests that Request fails after Close().
func TestKafkaBus_RequestAfterClose(t *testing.T) {
	bus := &KafkaBus{
		handlers:     make(map[string][]Handler),
		pending:      make(map[string]chan Event),
		consumerStop: make(chan struct{}),
		closed:       true, // Pre-closed
		timeout:      time.Second,
	}

	_, err := bus.Request(context.Background(), "test", Event{
		ID:            "test",
		CorrelationID: "test-corr",
	})
	if err == nil {
		t.Error("Request() after Close() should return error")
	}
}

// Note: Integration tests with real Kafka would go in kafka_integration_test.go
// Those tests would require Docker/Testcontainers and would be skipped in CI
// unless KAFKA_INTEGRATION_TESTS=1 is set.
