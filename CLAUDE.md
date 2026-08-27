# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

An in-development, single-node in-memory cache written in Go, built as a learning project toward a distributed cache. `ROADMAP.md` lays out 8 phases (core → network → clustering → discovery → replication → durability → production); `PHASE1.md` is the live checklist for the current phase. Only Phase 1 (core cache engine) is partially implemented — everything past `internal/cache` and `internal/eviction` is not started yet.

## Commands

```bash
go test ./...                          # all tests
go test -race ./...                    # race detector (expected before any concurrency work lands)
go test ./internal/cache/ -run TestCacheEviction   # single test
go test -bench . -benchmem ./...       # benchmarks (none written yet; see PHASE1.md §4)
go vet ./...
go run cmd/cache/main.go               # runs the placeholder demo, NOT a server
```

Requires Go 1.26+.

## Architecture

Two layers, deliberately separated (see `PHASE1.md` "Notes & Learnings"):

- **`internal/eviction`** — the storage layer. `Policy` is the interface; **values live inside the policy**, there is no separate data map. `LRU` (`lru.go`) implements it with `container/list` + `map[string]*list.Element` for O(1) ops, a capacity bound, and an optional `onEvicted` callback. `LFU` (`lfu.go`) is a TODO stub. Policy implementations are **not thread-safe** by contract.
- **`internal/cache`** — the concurrency + TTL wrapper. `Cache` holds a `sync.Mutex` and a `Policy`, and serializes all access to it. `Item` wraps the caller's value with an optional `ExpiresAt`. `NewCacheWithOptions` injects a custom policy; `NewCache` defaults to LRU.

Key invariants when touching this code:

- `Policy.Get` mutates recency/frequency state; `Policy.Peek` does not. `Cache.Get` uses `Peek` first for the TTL check, then calls `Get` **only on a live hit** so expired entries are never promoted before deletion.
- Use `sync.Mutex`, not `RWMutex`: every live read mutates eviction state, so a read lock buys nothing in Phase 1.
- Do not reintroduce a second `map` in `Cache` — the policy owns storage.
- TTL has two paths: passive (expired keys removed on `Get`) and active (an opt-in background janitor, enabled by `Options.CleanupInterval > 0`, that sweeps via `Policy.Keys()` + `Peek`). `Cache.Close()` stops the janitor; it is idempotent and a no-op when none was started.

## Notes

- `cmd/cache/main.go` is still a `sync.Mutex` counter demo, not a real entrypoint. Phase 2 replaces it.
- `PHASE1.md` checkboxes are the source of truth for what's done vs. pending in the current phase; update them as work lands.
