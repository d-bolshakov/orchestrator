package store

import (
	"fmt"
)

type InMemoryStore[V any] struct {
	db map[string]V
}

func (s *InMemoryStore[V]) Put(key string, value V) error {
	s.db[key] = value
	return nil
}

func (s *InMemoryStore[V]) Get(key string) (V, error) {
	var v V
	v, ok := s.db[key]
	if !ok {
		return v, fmt.Errorf("Value not found for key %s\n", key)
	}
	return v, nil
}

func (s *InMemoryStore[V]) List() ([]V, error) {
	values := []V{}

	for _, v := range s.db {
		values = append(values, v)
	}
	return values, nil
}
func (s *InMemoryStore[V]) Count() (int, error) {
	return len(s.db), nil
}

func NewInMemoryTaskStore[V any]() *InMemoryStore[V] {
	return &InMemoryStore[V]{
		db: make(map[string]V),
	}
}
