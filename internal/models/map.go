package models

import (
	"fmt"
	"iter"
	"maps"

	"gopkg.in/yaml.v3"
)

// Read-only way to store ordered maps from a YAML structure
type OrderedYAMLMap[K comparable, V any] struct {
	keys []K
	data map[K]V
}

func NewOrderedYAMLMap[K comparable, V any](keys []K, values []V) (*OrderedYAMLMap[K, V], error) {
	if len(keys) != len(values) {
		return nil, fmt.Errorf("keys and values must have the same length")
	}

	om := &OrderedYAMLMap[K, V]{
		keys: make([]K, len(keys)),
		data: make(map[K]V, len(keys)),
	}

	copy(om.keys, keys)

	for i := range keys {
		om.data[keys[i]] = values[i]
	}

	return om, nil
}

func (om *OrderedYAMLMap[K, V]) Items() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, key := range om.keys {
			value, ok := om.data[key]
			if !ok {
				continue
			}
			if !yield(key, value) {
				return
			}
		}
	}
}

func (om *OrderedYAMLMap[K, V]) Get(key K) (V, bool) {
	value, ok := om.data[key]
	return value, ok
}

func (self *OrderedYAMLMap[K, V]) Merge(other *OrderedYAMLMap[K, V]) *OrderedYAMLMap[K, V] {
	merged := &OrderedYAMLMap[K, V]{
		keys: make([]K, 0, len(self.keys)+len(other.keys)),
		data: make(map[K]V, len(self.data)+len(other.data)),
	}

	merged.keys = append(merged.keys, self.keys...)
	maps.Copy(merged.data, self.data)

	for _, key := range other.keys {
		if _, exists := self.data[key]; !exists {
			merged.keys = append(merged.keys, key)
		}
	}
	maps.Copy(merged.data, other.data)

	return merged
}

func (om *OrderedYAMLMap[K, V]) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("orderedMap: expected mapping node, got %d", node.Kind)
	}

	if len(node.Content)%2 != 0 {
		return fmt.Errorf("orderedMap: expected even number of content items, got %d", len(node.Content))
	}

	om.keys = make([]K, len(node.Content)/2)
	om.data = make(map[K]V, len(node.Content)/2)

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		var key K
		if err := keyNode.Decode(&key); err != nil {
			return fmt.Errorf("orderedMap: decoding key: %v", err)
		}

		if _, ok := om.data[key]; ok {
			return fmt.Errorf("orderedMap: duplicate key %v", key)
		}

		var value V
		if err := valueNode.Decode(&value); err != nil {
			return fmt.Errorf("orderedMap: decoding value: %v", err)
		}

		(*om).keys[i/2] = key
		(*om).data[key] = value
	}

	return nil
}
