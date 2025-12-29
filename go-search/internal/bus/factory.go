package bus

import (
	"fmt"
	"strings"

	"github.com/ricesearch/rice-search/internal/config"
	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// NewBus creates a new Bus instance based on the configuration.
func NewBus(cfg config.BusConfig) (Bus, error) {
	switch strings.ToLower(cfg.Type) {
	case "memory", "":
		return NewMemoryBus(), nil

	case "kafka":
		brokers := ParseKafkaBrokers(cfg.KafkaBrokers)
		if len(brokers) == 0 {
			return nil, errors.New(errors.CodeValidation, "kafka brokers not configured")
		}

		consumerGroup := cfg.KafkaGroup
		if consumerGroup == "" {
			consumerGroup = "rice-search"
		}

		return NewKafkaBus(KafkaConfig{
			Brokers:       brokers,
			ConsumerGroup: consumerGroup,
			ClientID:      "rice-search-bus",
		})

	case "nats":
		return nil, errors.New(errors.CodeInternal, "NATS bus not implemented yet")

	case "redis":
		return nil, errors.New(errors.CodeInternal, "Redis Streams bus not implemented yet")

	default:
		return nil, errors.New(errors.CodeValidation, fmt.Sprintf("unknown bus type: %s", cfg.Type))
	}
}
