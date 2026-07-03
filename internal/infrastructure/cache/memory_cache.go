package cache

import (
	"context"
	"sync"
	"time"
)

// InMemoryResponseCache implementa domain.ResponseCache. Serve bem para
// rodar localmente ou em um único processo. Para produção multi-instância,
// troque por uma implementação Redis/ElastiCache — a interface não muda.
type InMemoryResponseCache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]entry
}

type entry struct {
	value     string
	expiresAt time.Time
}

func NewInMemoryResponseCache(ttl time.Duration) *InMemoryResponseCache {
	return &InMemoryResponseCache{ttl: ttl, m: make(map[string]entry)}
}

func (c *InMemoryResponseCache) Get(_ context.Context, key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.m[key]
	if !ok || time.Now().After(e.expiresAt) {
		return "", false
	}
	return e.value, true
}

func (c *InMemoryResponseCache) Set(_ context.Context, key string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = entry{value: value, expiresAt: time.Now().Add(c.ttl)}
}
