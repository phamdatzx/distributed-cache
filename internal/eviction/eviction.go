package eviction

// Policy stores key/value pairs and decides which key to evict under capacity pressure.
// Implementations are not thread-safe; callers (e.g. cache.Cache) must serialize access.
type Policy interface {
	Set(key string, value any)
	Get(key string) (any, bool)  // hit: update recency / frequency
	Peek(key string) (any, bool) // read without updating stats (TTL checks)
	Delete(key string) (any, bool)
	Len() int
	Keys() []string // for janitor sweep
}
