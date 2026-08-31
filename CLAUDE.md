# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

An in-development, in-memory cache written in Go, built as a learning project toward a distributed cache. `ROADMAP.md` lays out 8 phases (core → network → clustering → discovery → replication → durability → production); `PHASE1.md` and `PHASE2.md` are the live checklists. Phase 1 (core cache engine) is mostly done (LRU, TTL, janitor, tests); Phase 2 (TCP server) is done; Phases 3–8 are not started.

## Commands

```bash
go test ./...                          # all tests
go test -race ./...                    # race detector
go test ./internal/server/ -run TestServerPing   # single test
go vet ./...
go run ./cmd/cache                     # runs the TCP server (default :6380)
```

Requires Go 1.26+.

## Architecture

Three layers, deliberately separated (see the `PHASE*.md` "Notes & Learnings"):

- **`internal/eviction`** — the storage layer. `Policy` is the interface; **values live inside the policy**, there is no separate data map. `LRU` (`lru.go`) implements it with `container/list` + `map[string]*list.Element` for O(1) ops, a capacity bound, and an optional `onEvicted` callback. `LFU` (`lfu.go`) is a TODO stub. Policy implementations are **not thread-safe** by contract.
- **`internal/cache`** — the concurrency + TTL wrapper. `Cache` holds a `sync.Mutex` and a `Policy`, and serializes all access to it. `Item` wraps the caller's value with an optional `ExpiresAt`. `NewCacheWithOptions` injects a custom policy; `NewCache` defaults to LRU.
- **`internal/server`** — the network layer. `Server` exposes a `*cache.Cache` over TCP with a small RESP-flavored inline protocol (`PING`/`GET`/`SET`/`DEL`/`STATS`/`QUIT`). `ListenAndServe` runs the accept loop + one goroutine per connection; `Shutdown(ctx)` closes the listener and drains in-flight connections. `protocol.go` holds the parser + reply writers, `commands.go` the dispatch, `server.go` the accept/drain lifecycle + atomic counters.

Key invariants when touching this code:

- `Policy.Get` mutates recency/frequency state; `Policy.Peek` does not. `Cache.Get` uses `Peek` first for the TTL check, then calls `Get` **only on a live hit** so expired entries are never promoted before deletion.
- Use `sync.Mutex`, not `RWMutex`: every live read mutates eviction state, so a read lock buys nothing.
- Do not reintroduce a second `map` in `Cache` — the policy owns storage.
- TTL has two paths: passive (expired keys removed on `Get`) and active (an opt-in background janitor, enabled by `Options.CleanupInterval > 0`, that sweeps via `Policy.Keys()` + `Peek`). `Cache.Close()` stops the janitor; it is idempotent and a no-op when none was started.
- The server adds no locking of its own — `cache.Cache` is already concurrency-safe. Values are strings on the wire; `GET` type-asserts the stored value to `string`.
- Wire format: inline commands, `\r\n`-terminated; replies are a RESP subset (`+OK`, `$n\r\n`, `$-1`, `:n`, `-ERR msg`). Values cannot contain spaces yet (inline space-split; a RESP array parser is a Phase 8 stretch).

## Notes

- `go run ./cmd/cache` boots the real server now (config via `-addr`/`-max-entries`/`-cleanup-interval`, each with an env-var fallback). `nc localhost 6380` can `PING`/`SET`/`GET`/`DEL`/`STATS`/`QUIT`.
- `PHASE*.md` checkboxes are the source of truth for what's done vs. pending in the current phase; update them as work lands.
