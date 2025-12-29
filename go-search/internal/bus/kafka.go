package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// KafkaBus is a Kafka-based event bus implementation.
type KafkaBus struct {
	config   KafkaConfig
	producer sarama.SyncProducer
	consumer sarama.ConsumerGroup
	client   sarama.Client

	mu       sync.RWMutex
	handlers map[string][]Handler
	pending  map[string]chan Event
	closed   bool

	// Consumer coordination
	consumerWg   sync.WaitGroup
	consumerStop chan struct{}
	timeout      time.Duration
}

// KafkaConfig holds Kafka connection settings.
type KafkaConfig struct {
	Brokers       []string      // Kafka broker addresses
	ConsumerGroup string        // Consumer group ID
	ClientID      string        // Client identifier
	Version       string        // Kafka version (e.g., "2.8.0")
	Timeout       time.Duration // Request timeout (default: 30s)
}

// NewKafkaBus creates a new Kafka-based event bus.
func NewKafkaBus(cfg KafkaConfig) (*KafkaBus, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New(errors.CodeValidation, "kafka brokers cannot be empty")
	}
	if cfg.ConsumerGroup == "" {
		return nil, errors.New(errors.CodeValidation, "kafka consumer group cannot be empty")
	}

	// Set defaults
	if cfg.ClientID == "" {
		cfg.ClientID = "rice-search-bus"
	}
	if cfg.Version == "" {
		cfg.Version = "2.8.0"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Parse Kafka version
	version, err := sarama.ParseKafkaVersion(cfg.Version)
	if err != nil {
		return nil, errors.Wrap(errors.CodeValidation, "invalid kafka version", err)
	}

	// Create Kafka client config
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = version
	kafkaConfig.ClientID = cfg.ClientID
	kafkaConfig.Producer.Return.Successes = true
	kafkaConfig.Producer.Return.Errors = true
	kafkaConfig.Producer.Retry.Max = 3
	kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll
	kafkaConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	kafkaConfig.Consumer.Return.Errors = true
	kafkaConfig.Net.DialTimeout = 10 * time.Second
	kafkaConfig.Net.ReadTimeout = 10 * time.Second
	kafkaConfig.Net.WriteTimeout = 10 * time.Second

	// Create Kafka client
	client, err := sarama.NewClient(cfg.Brokers, kafkaConfig)
	if err != nil {
		return nil, errors.Wrap(errors.CodeUnavailable, "failed to create kafka client", err)
	}

	// Create producer
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		client.Close()
		return nil, errors.Wrap(errors.CodeUnavailable, "failed to create kafka producer", err)
	}

	// Create consumer group
	consumer, err := sarama.NewConsumerGroupFromClient(cfg.ConsumerGroup, client)
	if err != nil {
		producer.Close()
		client.Close()
		return nil, errors.Wrap(errors.CodeUnavailable, "failed to create kafka consumer group", err)
	}

	bus := &KafkaBus{
		config:       cfg,
		producer:     producer,
		consumer:     consumer,
		client:       client,
		handlers:     make(map[string][]Handler),
		pending:      make(map[string]chan Event),
		consumerStop: make(chan struct{}),
		timeout:      cfg.Timeout,
	}

	return bus, nil
}

// Publish publishes an event to a Kafka topic.
func (b *KafkaBus) Publish(ctx context.Context, topic string, event Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return errors.New(errors.CodeUnavailable, "bus is closed")
	}

	// Serialize event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to marshal event", err)
	}

	// Create Kafka message
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
		Key:   sarama.StringEncoder(event.ID), // Use event ID as partition key
	}

	// Add correlation ID as header for request/reply
	if event.CorrelationID != "" {
		msg.Headers = []sarama.RecordHeader{
			{
				Key:   []byte("correlation_id"),
				Value: []byte(event.CorrelationID),
			},
		}
	}

	// Publish to Kafka
	_, _, err = b.producer.SendMessage(msg)
	if err != nil {
		return errors.Wrap(errors.CodeUnavailable, "failed to publish to kafka", err)
	}

	return nil
}

// Subscribe registers a handler for events on a Kafka topic.
func (b *KafkaBus) Subscribe(ctx context.Context, topic string, handler Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return errors.New(errors.CodeUnavailable, "bus is closed")
	}

	// Add handler to topic
	isNewTopic := len(b.handlers[topic]) == 0
	b.handlers[topic] = append(b.handlers[topic], handler)

	// Start consumer for this topic if it's the first handler
	if isNewTopic {
		b.consumerWg.Add(1)
		go b.consumeTopic(topic)
	}

	return nil
}

// Request sends a request and waits for a response.
func (b *KafkaBus) Request(ctx context.Context, topic string, req Event) (Event, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return Event{}, errors.New(errors.CodeUnavailable, "bus is closed")
	}

	// Create response channel for this correlation ID
	responseChan := make(chan Event, 1)
	b.pending[req.CorrelationID] = responseChan

	// Subscribe to response topic if not already subscribed
	responseTopic := topic + ".response"
	if len(b.handlers[responseTopic]) == 0 {
		b.handlers[responseTopic] = []Handler{b.handleResponse}
		b.consumerWg.Add(1)
		go b.consumeTopic(responseTopic)
	}
	b.mu.Unlock()

	// Clean up when done
	defer func() {
		b.mu.Lock()
		delete(b.pending, req.CorrelationID)
		close(responseChan)
		b.mu.Unlock()
	}()

	// Publish the request
	if err := b.Publish(ctx, topic, req); err != nil {
		return Event{}, err
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return Event{}, errors.Wrap(errors.CodeTimeout, "request timeout", ctx.Err())
	case <-time.After(b.timeout):
		return Event{}, errors.New(errors.CodeTimeout, "request timeout")
	case resp := <-responseChan:
		return resp, nil
	}
}

// handleResponse is the internal handler for response events.
func (b *KafkaBus) handleResponse(ctx context.Context, event Event) error {
	b.mu.RLock()
	ch, ok := b.pending[event.CorrelationID]
	b.mu.RUnlock()

	if !ok {
		// No pending request for this correlation ID (may have timed out)
		return nil
	}

	select {
	case ch <- event:
		return nil
	default:
		return errors.New(errors.CodeInternal, "response channel full")
	}
}

// consumeTopic starts a Kafka consumer for a specific topic.
func (b *KafkaBus) consumeTopic(topic string) {
	defer b.consumerWg.Done()

	handler := &consumerGroupHandler{
		bus:   b,
		topic: topic,
	}

	for {
		select {
		case <-b.consumerStop:
			return
		default:
		}

		// This is a blocking call that will run until the consumer is closed
		err := b.consumer.Consume(context.Background(), []string{topic}, handler)
		if err != nil {
			// Log error but continue (Kafka consumer will retry)
			fmt.Printf("kafka consumer error for topic %s: %v\n", topic, err)
		}

		// Check if we should stop
		select {
		case <-b.consumerStop:
			return
		default:
			// Small backoff before retrying
			time.Sleep(time.Second)
		}
	}
}

// Close closes the Kafka bus and releases resources.
func (b *KafkaBus) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()

	// Stop all consumers
	close(b.consumerStop)
	b.consumerWg.Wait()

	// Close Kafka resources
	var errs []error

	if err := b.consumer.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close consumer: %w", err))
	}

	if err := b.producer.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close producer: %w", err))
	}

	if err := b.client.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close client: %w", err))
	}

	b.mu.Lock()
	b.handlers = nil
	b.pending = nil
	b.mu.Unlock()

	if len(errs) > 0 {
		return errors.New(errors.CodeInternal, fmt.Sprintf("errors during close: %v", errs))
	}

	return nil
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler.
type consumerGroupHandler struct {
	bus   *KafkaBus
	topic string
}

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is run at the end of a session, after all ConsumeClaim goroutines have exited.
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim processes messages from a Kafka partition.
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-session.Context().Done():
			return nil
		case msg := <-claim.Messages():
			if msg == nil {
				return nil
			}

			// Deserialize event
			var event Event
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				fmt.Printf("failed to unmarshal event from kafka: %v\n", err)
				session.MarkMessage(msg, "")
				continue
			}

			// Get handlers for this topic
			h.bus.mu.RLock()
			handlers := h.bus.handlers[h.topic]
			h.bus.mu.RUnlock()

			// Execute all handlers
			for _, handler := range handlers {
				if err := handler(session.Context(), event); err != nil {
					fmt.Printf("handler error for topic %s: %v\n", h.topic, err)
					// Continue processing even if handler fails
				}
			}

			// Mark message as processed
			session.MarkMessage(msg, "")
		}
	}
}

// ParseKafkaBrokers parses a comma-separated string of Kafka brokers.
func ParseKafkaBrokers(brokersStr string) []string {
	if brokersStr == "" {
		return nil
	}
	brokers := strings.Split(brokersStr, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}
	return brokers
}
