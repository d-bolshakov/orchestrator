package store

import "fmt"

type Store[V any] interface {
	Put(key string, value V) error
	Get(key string) (V, error)
	List() ([]V, error)
	Count() (int, error)
}

func NewOfType[V any](storeType string, name string) Store[V] {
	switch storeType {
	case "inmemory":
		return NewInMemoryTaskStore[V]()

	case "persistent":
		s, _ := NewPersistentStore[V](generateStoreFileName(name), 0600, name)
		return s

	default:
		return NewInMemoryTaskStore[V]()
	}
}

func generateStoreFileName(storeName string) string {
	return fmt.Sprintf("%s.db", storeName)
}
