package cache

import (
	"sync"
	"time"
)
type Item struct {
    Value     any
    ExpiresAt time.Time
}

type Cache struct {
    mu   sync.RWMutex
    data map[string]Item
}

func (c *Cache) Get(key string) (any, bool) {
    c.mu.RLock()
    item, ok := c.data[key]
    c.mu.RUnlock()

    if !ok {
        return nil, false
    }

    if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
        return nil, false
    }

    return item.Value, true
}

func (c *Cache) Set(key string, value any, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()

    var expiresAt time.Time

    if ttl > 0 {
        expiresAt = time.Now().Add(ttl)
    }

    c.data[key] = Item{
        Value:     value,
        ExpiresAt: expiresAt,
    }
}