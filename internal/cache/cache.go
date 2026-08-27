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
	// CleanupInterval, when > 0, starts a background janitor that periodically
	// sweeps expired entries. Call Close to stop it.
	CleanupInterval time.Duration
}

type Cache struct {
	mu     sync.Mutex
	policy eviction.Policy

	stop      chan struct{} // closed by Close to signal the janitor; nil if no janitor
	done      chan struct{} // closed by the janitor on exit, so Close can wait for it
	closeOnce sync.Once
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
	c := &Cache{policy: policy}
	if opts.CleanupInterval > 0 {
		c.stop = make(chan struct{})
		c.done = make(chan struct{})
		go c.runJanitor(opts.CleanupInterval)
	}
	return c, nil
}

// runJanitor sweeps expired entries every interval until Close is called.
func (c *Cache) runJanitor(interval time.Duration) {
	defer close(c.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-c.stop:
			return
		}
	}
}

// deleteExpired removes every entry whose TTL has elapsed. It uses Peek so the
// eviction policy's recency/frequency stats are left untouched.
func (c *Cache) deleteExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Keys returns a snapshot slice, so deleting mid-iteration is safe.
	for _, k := range c.policy.Keys() {
		raw, ok := c.policy.Peek(k)
		if !ok {
			continue
		}
		if item, ok := raw.(Item); ok && item.Expired() {
			c.policy.Delete(k)
		}
	}
}

// Close stops the background janitor, if one is running. It is idempotent and
// safe to call on a cache created without CleanupInterval. The cache remains
// readable after Close; entries are simply no longer swept actively.
func (c *Cache) Close() {
	c.closeOnce.Do(func() {
		if c.stop == nil {
			return
		}
		close(c.stop)
		<-c.done
	})
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
