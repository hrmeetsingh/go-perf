package payload

import "fmt"

// ProviderConfig holds the parameters needed to construct a DynamicProvider.
type ProviderConfig struct {
	Type   string
	Min    int64
	Max    int64
	Length int
	Values []interface{}
}

// NewProviderFromConfig creates a DynamicProvider from a declarative configuration.
func NewProviderFromConfig(cfg ProviderConfig) (DynamicProvider, error) {
	switch cfg.Type {
	case "uuid":
		return NewUUIDProvider(), nil
	case "int_range":
		return NewIntRangeProvider(cfg.Min, cfg.Max), nil
	case "pool":
		if len(cfg.Values) == 0 {
			return nil, fmt.Errorf("pool provider requires at least one value")
		}
		return NewPoolProvider(cfg.Values), nil
	case "string":
		length := cfg.Length
		if length == 0 {
			length = 10
		}
		return NewStringProvider(length), nil
	case "timestamp":
		return NewTimestampProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %q", cfg.Type)
	}
}
