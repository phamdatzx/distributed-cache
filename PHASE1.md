# Phase 1: Core Cache Engine (Single Node) Progress Tracker

Use this checklist to track your progress as you design, implement, and verify the core in-memory cache engine.

---

## Roadmap & Checklist

### 1. Eviction Policies (`internal/eviction/`)

Pluggable store: values live inside the policy. LRU and future LFU both implement `Policy`.

#### Policy interface (`internal/eviction/eviction.go`)
- [x] `Set`, `Get`, `Peek`, `Delete`, `Len`, `Keys`

#### LRU (`internal/eviction/lru.go`)
- [x] **Data structures:** `entry{key, value}`, list + map index, capacity, `onEvicted`
- [x] `NewLRU(cap int, onEvicted func(string, any))`
- [x] `Set` / `Get` / `Peek` / `Delete` / `Len` / `Keys`
- [x] Capacity eviction via `removeOldest` (LRU at list back)
- [x] Unit tests in `lru_test.go`

#### LFU (`internal/eviction/lfu.go`)
- [ ] Implement `Policy` (TODO stub)

---

### 2. Thread-Safe & TTL Wrapper (`internal/cache/cache.go`)
- [x] `Item` with `ExpiresAt` + `Expired()`
- [x] `Cache` with `sync.Mutex` + `eviction.Policy` (no duplicate data map)
- [x] `NewCache` / `NewCacheWithOptions` (default LRU; injectable policy for LFU later)
- [x] `Set` / `Get` / `Delete` / `Len`
- [x] Passive TTL: expired keys deleted on `Get`
- [ ] Active TTL janitor (`time.Ticker` + clean shutdown / `Close`)

---

### 3. Testing & Validation
- [x] `TestCacheGetSet`
- [x] `TestCacheEviction`
- [x] `TestCacheTTLPassive`
- [ ] `TestCacheTTLActive` (needs janitor)
- [ ] `TestCacheConcurrency` with `go test -race ./internal/cache/...`

---

### 4. Performance & Profiling (CV Highlights)
- [ ] `BenchmarkCacheSet` / `BenchmarkCacheGet`
- [ ] Optional: compare mutex vs map sharding under load

---

## Notes & Learnings

* Policy owns storage; Cache owns locking + TTL. Do not keep a second `map` in Cache.
* `Get` on the policy updates LRU/LFU stats; use `Peek` for TTL/janitor so expired keys are not promoted.
* `RWMutex` does not help while every live hit mutates eviction state — use `Mutex` for Phase 1.
