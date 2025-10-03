package store

type Store[V any] interface {
	Put(key string, value V) error
	Get(key string) (V, error)
	List() ([]V, error)
	Count() (int, error)
}

func NewOfType[V any](storeType string) Store[V] {
	switch storeType {
	case "inmemory":
		return NewInMemoryTaskStore[V]()

	default:
		return NewInMemoryTaskStore[V]()
	}
}
