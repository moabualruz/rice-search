# Kafka Event Bus Implementation

## Overview

The Kafka Event Bus provides distributed event communication for go-search, enabling horizontal scaling and distributed deployments.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Rice Search Nodes                    │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Node 1   │  │ Node 2   │  │ Node 3   │              │
│  │ KafkaBus │  │ KafkaBus │  │ KafkaBus │              │
│  └─────┬────┘  └─────┬────┘  └─────┬────┘              │
│        │             │             │                     │
│        └─────────────┼─────────────┘                     │
│                      │                                   │
└──────────────────────┼───────────────────────────────────┘
                       │
         ┌─────────────▼─────────────┐
         │    Kafka Cluster          │
         │  ┌──────────────────────┐ │
         │  │  Topics:             │ │
         │  │  - ml.embed.request  │ │
         │  │  - ml.embed.response │ │
         │  │  - search.request    │ │
         │  │  - index.request     │ │
         │  │  ...                 │ │
         │  └──────────────────────┘ │
         └───────────────────────────┘
```

## Configuration

### Environment Variables

```bash
# Event Bus Type
RICE_BUS_TYPE=kafka

# Kafka Brokers (comma-separated)
RICE_KAFKA_BROKERS=localhost:9092,broker2:9092,broker3:9092

# Consumer Group ID
RICE_KAFKA_GROUP=rice-search

# Optional: Event Logging (for debugging)
RICE_EVENT_LOG_ENABLED=false
RICE_EVENT_LOG_PATH=./data/events.log
```

### YAML Configuration

```yaml
bus:
  type: kafka
  kafka_brokers: localhost:9092,broker2:9092
  kafka_group: rice-search
  event_log_enabled: false
  event_log_path: ./data/events.log
```

### Programmatic Configuration

```go
import (
    "github.com/ricesearch/rice-search/internal/bus"
    "github.com/ricesearch/rice-search/internal/config"
)

cfg := config.BusConfig{
    Type:         "kafka",
    KafkaBrokers: "localhost:9092,broker2:9092",
    KafkaGroup:   "rice-search",
}

eventBus, err := bus.NewBus(cfg)
if err != nil {
    // Falls back to MemoryBus automatically in server initialization
    log.Fatal(err)
}
defer eventBus.Close()
```

## Features

### 1. Request/Reply Pattern

Uses correlation IDs in Kafka headers for request-response pairing:

```go
// Client side
resp, err := eventBus.Request(ctx, bus.TopicEmbedRequest, bus.Event{
    ID:            "req-123",
    CorrelationID: uuid.New().String(),
    Payload:       map[string]interface{}{"texts": []string{"hello"}},
})

// Server side (handler)
func handleEmbedRequest(ctx context.Context, event bus.Event) error {
    // Process request...
    embeddings := ml.Embed(ctx, texts)
    
    // Respond to same correlation ID
    return bus.Publish(ctx, bus.TopicEmbedResponse, bus.Event{
        ID:            event.ID + "-response",
        CorrelationID: event.CorrelationID, // Match request
        Payload:       map[string]interface{}{"embeddings": embeddings},
    })
}
```

### 2. Publish/Subscribe Pattern

Fan-out events to multiple consumers:

```go
// Publisher
eventBus.Publish(ctx, bus.TopicChunkCreated, bus.Event{
    ID:      "chunk-456",
    Type:    "chunk.created",
    Payload: chunk,
})

// Subscribers (multiple nodes can subscribe)
eventBus.Subscribe(ctx, bus.TopicChunkCreated, func(ctx context.Context, event bus.Event) error {
    // Handle chunk creation
    return processChunk(event.Payload)
})
```

### 3. Graceful Shutdown

Proper consumer group cleanup on shutdown:

```go
eventBus := bus.NewKafkaBus(cfg)
defer eventBus.Close() // Gracefully closes producer + consumer + client

// The Close() method:
// 1. Stops all consumer goroutines
// 2. Waits for in-flight messages to complete
// 3. Commits offsets
// 4. Closes Kafka connections
```

### 4. At-Least-Once Delivery

Kafka configuration ensures reliability:

- **Producer**: `RequiredAcks = WaitForAll` (all replicas acknowledge)
- **Producer**: `Retry.Max = 3` (automatic retries)
- **Consumer**: Manual offset commits after processing
- **Consumer**: `OffsetNewest` (start from latest on first run)

### 5. Consumer Groups

Horizontal scaling with automatic partition assignment:

```
┌──────────────────────────────────────────┐
│          Consumer Group: rice-search      │
├──────────────────────────────────────────┤
│  Node 1 → Partition 0, 1                │
│  Node 2 → Partition 2, 3                │
│  Node 3 → Partition 4, 5                │
└──────────────────────────────────────────┘
```

Each message is processed by only ONE consumer in the group.

## Implementation Details

### KafkaBus Structure

```go
type KafkaBus struct {
    config   KafkaConfig
    producer sarama.SyncProducer    // Sends messages
    consumer sarama.ConsumerGroup   // Receives messages
    client   sarama.Client          // Kafka connection

    mu       sync.RWMutex
    handlers map[string][]Handler   // Topic → handlers
    pending  map[string]chan Event  // CorrelationID → response channel
    closed   bool

    consumerWg   sync.WaitGroup     // Wait for consumer goroutines
    consumerStop chan struct{}      // Signal to stop consumers
    timeout      time.Duration      // Request timeout
}
```

### Message Flow

#### Publish Flow

```
Application
    │
    ▼
KafkaBus.Publish()
    │
    ├─ Serialize Event to JSON
    │
    ├─ Add correlation ID to headers (if present)
    │
    ▼
Kafka Producer (sarama.SyncProducer)
    │
    ▼
Kafka Brokers
```

#### Subscribe Flow

```
Application
    │
    ▼
KafkaBus.Subscribe()
    │
    ├─ Register handler for topic
    │
    ├─ Start consumer goroutine (if first handler)
    │
    ▼
Consumer Group Handler
    │
    ├─ Consume messages from partition(s)
    │
    ├─ Deserialize JSON → Event
    │
    ├─ Execute all registered handlers
    │
    ├─ Mark message as processed (commit offset)
    │
    └─ Loop
```

#### Request/Reply Flow

```
Requester
    │
    ├─ Generate correlation ID
    │
    ├─ Create response channel
    │
    ├─ Store in pending map: correlationID → chan
    │
    ├─ Publish request to topic
    │
    └─ Wait on response channel (with timeout)
        │
        ▼
    Responder
        │
        ├─ Subscribe to request topic
        │
        ├─ Process request
        │
        ├─ Extract correlation ID from request
        │
        ├─ Publish response to response topic
        │
        └─ Include correlation ID in headers
            │
            ▼
        Response Handler (KafkaBus.handleResponse)
            │
            ├─ Extract correlation ID
            │
            ├─ Look up response channel in pending map
            │
            ├─ Send event to channel
            │
            └─ Requester receives response
```

### Consumer Group Handler

Implements `sarama.ConsumerGroupHandler` interface:

```go
type consumerGroupHandler struct {
    bus   *KafkaBus
    topic string
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
    // Called when consumer joins group
    return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
    // Called when consumer leaves group
    return nil
}

func (h *consumerGroupHandler) ConsumeClaim(
    session sarama.ConsumerGroupSession,
    claim sarama.ConsumerGroupClaim,
) error {
    for {
        select {
        case <-session.Context().Done():
            return nil // Graceful shutdown
        case msg := <-claim.Messages():
            // Deserialize, execute handlers, mark message
            session.MarkMessage(msg, "")
        }
    }
}
```

## Comparison with MemoryBus

| Feature | MemoryBus | KafkaBus |
|---------|-----------|----------|
| **Deployment** | Single process | Distributed |
| **Persistence** | No | Yes (Kafka retention) |
| **Scalability** | Vertical only | Horizontal scaling |
| **Fault Tolerance** | Process failure = data loss | Kafka replication |
| **Latency** | < 1ms | 5-10ms (network) |
| **Throughput** | High | Very high (Kafka) |
| **Replay** | No | Yes (from offset) |
| **Dependencies** | None | Kafka cluster |

## Deployment Scenarios

### Single Node (Development)

```bash
# Use MemoryBus (default)
RICE_BUS_TYPE=memory
./rice-search-server
```

### Multi-Node (Production)

```yaml
# docker-compose.yml
services:
  kafka:
    image: confluentinc/cp-kafka:latest
    ports:
      - "9092:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181

  rice-search-1:
    image: rice-search
    environment:
      RICE_BUS_TYPE: kafka
      RICE_KAFKA_BROKERS: kafka:9092
      RICE_KAFKA_GROUP: rice-search

  rice-search-2:
    image: rice-search
    environment:
      RICE_BUS_TYPE: kafka
      RICE_KAFKA_BROKERS: kafka:9092
      RICE_KAFKA_GROUP: rice-search
```

## Troubleshooting

### Connection Errors

**Error**: `SERVICE_UNAVAILABLE: failed to create kafka client`

**Solutions**:
1. Verify Kafka brokers are running: `docker ps | grep kafka`
2. Check network connectivity: `telnet localhost 9092`
3. Verify broker addresses in config
4. Check firewall rules

### Consumer Lag

**Error**: Messages processing slowly

**Solutions**:
1. Increase partition count for parallel processing
2. Scale out (add more nodes to consumer group)
3. Optimize handler performance
4. Check network latency

### Correlation ID Mismatch

**Error**: Request timeout even though Kafka is working

**Solutions**:
1. Verify responder is subscribing to correct response topic
2. Check correlation ID is being preserved in response
3. Verify response topic naming convention: `{request-topic}.response`

## Testing

```bash
# Unit tests (no Kafka required - mocked)
go test ./internal/bus/... -v

# Integration tests (requires Kafka)
docker-compose -f deployments/docker-compose.dev.yml up -d kafka
KAFKA_INTEGRATION_TESTS=1 go test ./internal/bus/... -v -tags=integration

# Manual testing
RICE_BUS_TYPE=kafka \
RICE_KAFKA_BROKERS=localhost:9092 \
RICE_KAFKA_GROUP=test-group \
./rice-search-server
```

## Best Practices

1. **Topic Naming**: Use hierarchical names: `{service}.{entity}.{action}`
   - Example: `ml.embed.request`, `search.request`, `index.chunk.created`

2. **Correlation IDs**: Use UUIDs for uniqueness
   ```go
   correlationID := uuid.New().String()
   ```

3. **Timeouts**: Set reasonable timeouts for requests
   ```go
   ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
   defer cancel()
   ```

4. **Error Handling**: Always handle errors in handlers
   ```go
   eventBus.Subscribe(ctx, topic, func(ctx context.Context, event bus.Event) error {
       if err := processEvent(event); err != nil {
           log.Error("Failed to process event", "error", err)
           return err // Kafka will retry based on config
       }
       return nil
   })
   ```

5. **Graceful Shutdown**: Always close the bus
   ```go
   defer eventBus.Close()
   ```

## Future Enhancements

- **Dead Letter Queue**: Failed messages go to `{topic}.dlq`
- **Schema Registry**: Avro/Protobuf message serialization
- **Exactly-Once Semantics**: Kafka transactions
- **Metrics**: Kafka consumer lag, throughput metrics
- **Admin Operations**: Topic creation, partition rebalancing

## References

- [IBM Sarama Documentation](https://github.com/IBM/sarama)
- [Kafka Consumer Groups](https://kafka.apache.org/documentation/#consumerconfigs)
- [Request-Reply Pattern](https://developer.confluent.io/patterns/event/correlation-identifier/)
