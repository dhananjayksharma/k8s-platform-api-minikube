package metadata

import "sync"

type Store struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewStore() *Store {
	return &Store{
		data: map[string]string{
			"cluster":     "minikube",
			"environment": "local",
		},
	}
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *Store) Put(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}
