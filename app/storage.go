package main

import (
	"sync"
	"time"
)

// Value interface that reports its data type (StringEntry,Stream)
type Value interface {
	Type() string
}

type StringEntry struct {
	value  string
	expiry time.Time
}

func (e StringEntry) Type() string { return "string" }

// in-memory key-value store with mutex for concurrency
type Storage struct {
	data map[string]Value
	mu   sync.Mutex
}

func (s *Storage) Get(key string) (Value, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *Storage) Set(key string, val Value) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
}
