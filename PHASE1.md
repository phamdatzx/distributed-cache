# Phase 1: Core Cache Engine (Single Node) Progress Tracker

Use this checklist to track your progress as you design, implement, and verify the core in-memory cache engine. 

---

## 📅 Roadmap & Checklist

### 1. LRU Eviction Engine (`internal/cache/lru.go`)
- [ ] **Define Data Structures:**
  - [ ] Implement the `entry` struct holding both the original string key and an `interface{}` value.
  - [ ] Implement the `LRU` struct with fields for a standard doubly-linked list (`*list.List`), lookup map (`map[string]*list.Element`), the max capacity limit, and an optional eviction callback (`onEvicted`).
- [ ] **Implement Core Methods:**
  - [ ] `NewLRU(maxEntries int, onEvicted func(string, interface{}))`: Constructor function.
  - [ ] `Add(key string, value interface{})`: Insert or update a value, moving it to the front of the list, and evicting the oldest if over capacity.
  - [ ] `Get(key string)`: Retrieve an item and mark it as recently used by moving its list node to the front.
  - [ ] `Remove(key string)`: Manually remove an element from the list and map.
  - [ ] `RemoveOldest()`: Evict the least recently used element (at the back of the list) and invoke the `onEvicted` callback.

---

### 2. Thread-Safe & TTL Wrapper (`internal/cache/cache.go`)
- [ ] **Define Structures:**
  - [ ] Define the `Item` struct wrapping the raw value with an absolute Unix nanoseconds expiration timestamp (`Expiration`).
  - [ ] Define `Expired() bool` helper on `Item` to easily check if the current time has surpassed the expiration threshold.
  - [ ] Define `Cache` struct combining a `sync.RWMutex` lock, the custom `LRU` instance, and a background cleanup janitor.
- [ ] **Implement Thread-Safe Interface:**
  - [ ] `NewCache(maxEntries int, cleanupInterval time.Duration)`: Constructor to initialize both the LRU and the passive/active TTL engine.
  - [ ] `Set(key string, value interface{}, ttl time.Duration)`: Safely acquire a write-lock, wrap the value in an `Item`, calculate expiration, and add it to the LRU engine.
  - [ ] `Get(key string)`: Retrieve an item under a read-lock. If expired, release read-lock, acquire write-lock, delete the key, and return a cache miss (Passive Eviction).
  - [ ] `Delete(key string)`: Safely remove a key under a write-lock.
- [ ] **Implement Active TTL Cleanup (Janitor):**
  - [ ] Implement a `janitor` struct that manages a cleanup loop using `time.Ticker`.
  - [ ] Launch the janitor loop in a separate goroutine (`go c.janitor.Run(c)`) upon cache creation.
  - [ ] Implement clean exit channels for the janitor loop to prevent resource leaks during tests.

---

### 3. Testing & Validation (`internal/cache/cache_test.go`)
- [ ] **Unit Tests:**
  - [ ] `TestCacheGetSet`: Verify basic value insertion and retrieval.
  - [ ] `TestCacheEviction`: Ensure oldest elements are removed when the cache size exceeds maximum capacity.
  - [ ] `TestCacheTTLPassive`: Verify that expired items return a miss on retrieval (passive eviction).
  - [ ] `TestCacheTTLActive`: Verify that a background janitor successfully cleans up expired keys without manual retrieval.
- [ ] **Concurrency Tests:**
  - [ ] `TestCacheConcurrency`: Write a test spawning hundreds of goroutines simultaneously writing, reading, and deleting to check for race conditions. Run using:
    ```bash
    go test -v -race ./internal/cache/...
    ```

---

### 4. Performance & Profiling (CV Highlights)
- [ ] **Benchmarks:**
  - [ ] Write `BenchmarkCacheSet` and `BenchmarkCacheGet` in `cache_test.go`.
  - [ ] Run benchmarks and record baseline results:
    ```bash
    go test -bench=. -benchmem ./internal/cache/...
    ```
- [ ] **Optimization (Optional):**
  - [ ] Compare `sync.RWMutex` lock contention with a map-sharding approach (e.g., splitting keys into $N$ different cache buckets, each with its own mutex) to see how it affects concurrent benchmark scores.

---

## 📝 Notes & Learnings
*Record any performance breakthroughs, tricky bugs (like deadlocks or lock upgrades), or key technical decisions here. These make excellent talking points during job interviews!*
