package payload

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type FieldType string

const (
	FieldTypeString  FieldType = "string"
	FieldTypeInt32   FieldType = "int32"
	FieldTypeInt64   FieldType = "int64"
	FieldTypeBool    FieldType = "bool"
	FieldTypeDouble  FieldType = "double"
	FieldTypeFloat   FieldType = "float"
	FieldTypeMessage FieldType = "message"
)

type FieldDescriptor struct {
	Name     string
	Type     FieldType
	Path     string
	Children []FieldDescriptor
}

type DynamicProvider interface {
	Generate() interface{}
}

func GenerateSamplePayload(fields []FieldDescriptor) map[string]interface{} {
	result := make(map[string]interface{})
	for _, f := range fields {
		result[f.Name] = defaultValueForType(f)
	}
	return result
}

func defaultValueForType(f FieldDescriptor) interface{} {
	switch f.Type {
	case FieldTypeString:
		return "sample_" + f.Name
	case FieldTypeInt32, FieldTypeInt64:
		return int64(0)
	case FieldTypeBool:
		return false
	case FieldTypeDouble, FieldTypeFloat:
		return float64(0.0)
	case FieldTypeMessage:
		nested := make(map[string]interface{})
		for _, child := range f.Children {
			nested[child.Name] = defaultValueForType(child)
		}
		return nested
	default:
		return nil
	}
}

func HashPayload(payload map[string]interface{}) (string, error) {
	data, err := json.Marshal(canonicalize(payload))
	if err != nil {
		return "", fmt.Errorf("marshaling payload for hash: %w", err)
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8]), nil
}

func canonicalize(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case map[string]interface{}:
			result[k] = canonicalize(val)
		default:
			result[k] = val
		}
	}
	return result
}

func ApplyDynamicFields(base map[string]interface{}, providers map[string]DynamicProvider) map[string]interface{} {
	result := copyMap(base)
	for path, provider := range providers {
		setNestedField(result, path, provider.Generate())
	}
	return result
}

func setNestedField(m map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[part] = next
		}
		current = next
	}
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		if nested, ok := v.(map[string]interface{}); ok {
			result[k] = copyMap(nested)
		} else {
			result[k] = v
		}
	}
	return result
}

// --- Dynamic Providers ---

type uuidProvider struct{}

func NewUUIDProvider() DynamicProvider {
	return &uuidProvider{}
}

func (p *uuidProvider) Generate() interface{} {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type intRangeProvider struct {
	min, max int64
}

func NewIntRangeProvider(min, max int64) DynamicProvider {
	return &intRangeProvider{min: min, max: max}
}

func (p *intRangeProvider) Generate() interface{} {
	if p.min == p.max {
		return p.min
	}
	return p.min + rand.Int63n(p.max-p.min+1)
}

type poolProvider struct {
	values []interface{}
}

func NewPoolProvider(values []interface{}) DynamicProvider {
	return &poolProvider{values: values}
}

func (p *poolProvider) Generate() interface{} {
	return p.values[rand.Intn(len(p.values))]
}

type stringProvider struct {
	length int
}

func NewStringProvider(length int) DynamicProvider {
	return &stringProvider{length: length}
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func (p *stringProvider) Generate() interface{} {
	b := make([]byte, p.length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

type timestampProvider struct{}

func NewTimestampProvider() DynamicProvider {
	return &timestampProvider{}
}

func (p *timestampProvider) Generate() interface{} {
	return time.Now().UnixNano()
}
