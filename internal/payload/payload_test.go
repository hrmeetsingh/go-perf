package payload

import (
	"encoding/json"
	"testing"
)

func TestGenerateSamplePayload(t *testing.T) {
	fields := []FieldDescriptor{
		{Name: "name", Type: FieldTypeString, Path: "name"},
		{Name: "age", Type: FieldTypeInt32, Path: "age"},
		{Name: "active", Type: FieldTypeBool, Path: "active"},
		{Name: "score", Type: FieldTypeDouble, Path: "score"},
	}

	payload := GenerateSamplePayload(fields)
	if payload == nil {
		t.Fatal("expected non-nil payload")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := m["name"]; !ok {
		t.Error("missing 'name' field")
	}
	if _, ok := m["age"]; !ok {
		t.Error("missing 'age' field")
	}
	if _, ok := m["active"]; !ok {
		t.Error("missing 'active' field")
	}
	if _, ok := m["score"]; !ok {
		t.Error("missing 'score' field")
	}
}

func TestGenerateSamplePayload_NestedMessage(t *testing.T) {
	fields := []FieldDescriptor{
		{Name: "name", Type: FieldTypeString, Path: "name"},
		{Name: "address", Type: FieldTypeMessage, Path: "address", Children: []FieldDescriptor{
			{Name: "street", Type: FieldTypeString, Path: "address.street"},
			{Name: "city", Type: FieldTypeString, Path: "address.city"},
			{Name: "zip", Type: FieldTypeInt32, Path: "address.zip"},
		}},
	}

	payload := GenerateSamplePayload(fields)
	data, _ := json.Marshal(payload)
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	addr, ok := m["address"]
	if !ok {
		t.Fatal("missing 'address' field")
	}
	addrMap, ok := addr.(map[string]interface{})
	if !ok {
		t.Fatalf("address is not a map, got %T", addr)
	}
	if _, ok := addrMap["street"]; !ok {
		t.Error("missing 'address.street' field")
	}
}

func TestHashPayload(t *testing.T) {
	p1 := map[string]interface{}{"name": "alice", "age": 30}
	p2 := map[string]interface{}{"name": "alice", "age": 30}
	p3 := map[string]interface{}{"name": "bob", "age": 25}

	h1, err := HashPayload(p1)
	if err != nil {
		t.Fatalf("HashPayload(p1) error = %v", err)
	}
	h2, err := HashPayload(p2)
	if err != nil {
		t.Fatalf("HashPayload(p2) error = %v", err)
	}
	h3, err := HashPayload(p3)
	if err != nil {
		t.Fatalf("HashPayload(p3) error = %v", err)
	}

	if h1 != h2 {
		t.Errorf("identical payloads should have same hash: %q != %q", h1, h2)
	}
	if h1 == h3 {
		t.Error("different payloads should have different hashes")
	}
}

func TestHashPayload_EmptyPayload(t *testing.T) {
	h, err := HashPayload(map[string]interface{}{})
	if err != nil {
		t.Fatalf("HashPayload error = %v", err)
	}
	if h == "" {
		t.Error("hash should not be empty for empty payload")
	}
}

func TestUUIDProvider(t *testing.T) {
	p := NewUUIDProvider()
	v1 := p.Generate()
	v2 := p.Generate()

	s1, ok := v1.(string)
	if !ok {
		t.Fatalf("UUID provider should return string, got %T", v1)
	}
	s2, ok := v2.(string)
	if !ok {
		t.Fatalf("UUID provider should return string, got %T", v2)
	}
	if s1 == s2 {
		t.Error("UUID provider should generate unique values")
	}
	if len(s1) != 36 {
		t.Errorf("UUID length = %d, want 36", len(s1))
	}
}

func TestIntRangeProvider(t *testing.T) {
	p := NewIntRangeProvider(1, 100)
	for i := 0; i < 50; i++ {
		v := p.Generate()
		n, ok := v.(int64)
		if !ok {
			t.Fatalf("IntRange should return int64, got %T", v)
		}
		if n < 1 || n > 100 {
			t.Errorf("value %d out of range [1, 100]", n)
		}
	}
}

func TestIntRangeProvider_SingleValue(t *testing.T) {
	p := NewIntRangeProvider(42, 42)
	v := p.Generate()
	n, ok := v.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", v)
	}
	if n != 42 {
		t.Errorf("single-value range should return 42, got %d", n)
	}
}

func TestPoolProvider(t *testing.T) {
	values := []interface{}{"active", "inactive", "pending"}
	p := NewPoolProvider(values)

	seen := map[interface{}]bool{}
	for i := 0; i < 100; i++ {
		v := p.Generate()
		seen[v] = true
	}

	for _, expected := range values {
		if !seen[expected] {
			t.Errorf("pool provider never generated %v after 100 iterations", expected)
		}
	}
}

func TestPoolProvider_SingleElement(t *testing.T) {
	p := NewPoolProvider([]interface{}{"only"})
	v := p.Generate()
	if v != "only" {
		t.Errorf("single-element pool should return %q, got %v", "only", v)
	}
}

func TestStringProvider(t *testing.T) {
	p := NewStringProvider(10)
	v := p.Generate()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("StringProvider should return string, got %T", v)
	}
	if len(s) != 10 {
		t.Errorf("string length = %d, want 10", len(s))
	}
}

func TestTimestampProvider(t *testing.T) {
	p := NewTimestampProvider()
	v := p.Generate()
	n, ok := v.(int64)
	if !ok {
		t.Fatalf("TimestampProvider should return int64, got %T", v)
	}
	if n <= 0 {
		t.Error("timestamp should be positive")
	}
}

func TestApplyDynamicFields(t *testing.T) {
	base := map[string]interface{}{
		"name": "placeholder",
		"age":  0,
	}

	providers := map[string]DynamicProvider{
		"name": NewPoolProvider([]interface{}{"alice", "bob"}),
		"age":  NewIntRangeProvider(20, 30),
	}

	result := ApplyDynamicFields(base, providers)

	name, ok := result["name"].(string)
	if !ok {
		t.Fatalf("name should be string, got %T", result["name"])
	}
	if name != "alice" && name != "bob" {
		t.Errorf("name = %q, want alice or bob", name)
	}

	age, ok := result["age"].(int64)
	if !ok {
		t.Fatalf("age should be int64, got %T", result["age"])
	}
	if age < 20 || age > 30 {
		t.Errorf("age = %d, want [20, 30]", age)
	}
}

func TestApplyDynamicFields_NestedPath(t *testing.T) {
	base := map[string]interface{}{
		"address": map[string]interface{}{
			"city": "placeholder",
		},
	}

	providers := map[string]DynamicProvider{
		"address.city": NewPoolProvider([]interface{}{"NYC", "LA"}),
	}

	result := ApplyDynamicFields(base, providers)
	addr := result["address"].(map[string]interface{})
	city := addr["city"].(string)
	if city != "NYC" && city != "LA" {
		t.Errorf("city = %q, want NYC or LA", city)
	}
}

func TestApplyDynamicFields_PreservesNonDynamic(t *testing.T) {
	base := map[string]interface{}{
		"name":   "static_value",
		"amount": 42,
	}

	providers := map[string]DynamicProvider{
		"amount": NewIntRangeProvider(1, 10),
	}

	result := ApplyDynamicFields(base, providers)
	if result["name"] != "static_value" {
		t.Errorf("non-dynamic field changed: name = %v", result["name"])
	}
}
