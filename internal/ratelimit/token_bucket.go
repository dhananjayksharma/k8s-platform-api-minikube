package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

type Manager struct {
	mu       sync.Mutex
	capacity float64
	refill   float64
	clients  map[string]*bucket
}

func NewManager(capacity, refillPerSecond float64) *Manager {
	return &Manager{
		capacity: capacity,
		refill:   refillPerSecond,
		clients:  make(map[string]*bucket),
	}
}

func (m *Manager) Allow(clientID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	b, ok := m.clients[clientID]
	if !ok {
		b = &bucket{tokens: m.capacity, lastRefill: now}
		m.clients[clientID] = b
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * m.refill
	if b.tokens > m.capacity {
		b.tokens = m.capacity
	}
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
