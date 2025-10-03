package store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/boltdb/bolt"
)

type PersistentStore[V any] struct {
	Db       *bolt.DB
	DbFile   string
	FileMode os.FileMode
	Bucket   string
}

func (s *PersistentStore[V]) Put(key string, value V) error {
	return s.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(s.Bucket))

		buf, err := json.Marshal(value)
		if err != nil {
			return err
		}

		err = b.Put([]byte(key), buf)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *PersistentStore[V]) Get(key string) (V, error) {
	var value V

	err := s.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(s.Bucket))
		buf := b.Get([]byte(key))
		if buf == nil {
			return fmt.Errorf("value for the key %s not found", key)
		}

		err := json.Unmarshal(buf, &value)
		if err != nil {
			return err
		}
		return nil
	})
	return value, err
}

func (s *PersistentStore[V]) List() ([]V, error) {
	values := []V{}

	err := s.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(s.Bucket))
		b.ForEach(func(k, v []byte) error {
			var value V
			err := json.Unmarshal(v, &value)
			if err != nil {
				return err
			}
			values = append(values, value)
			return nil
		})
		return nil
	})

	return values, err
}

func (s *PersistentStore[V]) Count() (int, error) {
	taskCount := 0

	err := s.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(s.Bucket))
		b.ForEach(func(k, v []byte) error {
			taskCount++
			return nil
		})
		return nil
	})

	if err != nil {
		return -1, err
	}

	return taskCount, nil
}

func (s *PersistentStore[V]) CreateBucket() error {
	return s.Db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(s.Bucket))
		if err != nil {
			return fmt.Errorf("create bucket %s: %v", s.Bucket, err)
		}
		return nil
	})
}

func (s *PersistentStore[V]) Close() {
	s.Db.Close()
}

func NewPersistentStore[V any](file string, mode os.FileMode, bucket string) (*PersistentStore[V], error) {
	db, err := bolt.Open(file, mode, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to open %v", file)
	}

	s := PersistentStore[V]{
		Db:       db,
		DbFile:   file,
		FileMode: mode,
		Bucket:   bucket,
	}

	err = s.CreateBucket()
	if err != nil {
		log.Printf("bucket already exists, will use ut instead of creating new one")
	}

	return &s, nil
}
