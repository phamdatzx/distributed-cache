package cache

import (
	"sync"
	"time"

	"distributed-cache/internal/eviction"
)

// Item wraps a cached value with an optional absolute expiration time.
// Zero ExpiresAt means the entry does not expire.
type Item struct {
	Value     any
	ExpiresAt time.Time
}

func (i Item) Expired() bool {
	return !i.ExpiresAt.IsZero() && time.Now().After(i.ExpiresAt)
}

// Options configures a new Cache.
type Options struct {
	MaxEntries int
	Policy     eviction.Policy // if nil, defaults to LRU with MaxEntries
}

type Cache struct {
	mu     sync.Mutex
	policy eviction.Policy
}

// NewCache creates a cache with an LRU policy of the given capacity.
func NewCache(maxEntries int) (*Cache, error) {
	return NewCacheWithOptions(Options{MaxEntries: maxEntries})
}

// NewCacheWithOptions creates a cache from Options.
// When Policy is nil, an LRU with MaxEntries is used.
func NewCacheWithOptions(opts Options) (*Cache, error) {
	policy := opts.Policy
	if policy == nil {
		lru, err := eviction.NewLRU(opts.MaxEntries, nil)
		if err != nil {
			return nil, err
		}
		policy = lru
	}
	return &Cache{policy: policy}, nil
}

func (c *Cache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	c.policy.Set(key, Item{
		Value:     value,
		ExpiresAt: expiresAt,
	})
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	raw, ok := c.policy.Peek(key)
	if !ok {
		return nil, false
	}
	item, ok := raw.(Item)
	if !ok {
		return nil, false
	}
	if item.Expired() {
		c.policy.Delete(key)
		return nil, false
	}
	// Promote only live hits so LRU/LFU stats stay accurate.
	c.policy.Get(key)
	return item.Value, true
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.policy.Delete(key)
}

func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.policy.Len()
}
