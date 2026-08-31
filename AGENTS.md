# AGENTS.md

Init / onboarding file for AI coding agents. Read this before touching code. It is the
authoritative project map; the phase checklists are the source of truth for *what* to do,
this file is the source of truth for *how* the code is shaped and why.

## What this project is

An in-development, **single-node in-memory cache in Go**, built deliberately as a learning
journey toward a **distributed cache**. Not production software — the point is to learn
distributed systems, concurrency, networking, and Go engineering, and to produce something
that holds up on a CV / in system-design interviews.

- Module: `distributed-cache` (see `go.mod`)
- Requires **Go 1.26+** (module declares `go 1.26.5`)
- No third-party dependencies; stdlib only.

## Phase plan (high level)

`ROADMAP.md` lays out 8 phases: `0 Foundation → 1 Core Cache → 2 Network → 3 Clustering →
4 Discovery → 5 Replication → 6 Durability → 7 Production`, plus optional `8 Stretch Goals`
(RESP compat, chaos testing, etc.). Each phase has a Definition of Done.

### Current state (verified at last init)

| Phase | Status | Reality on disk |
|-------|--------|-----------------|
| 0 Foundation | Partial | Module + layout + docs exist; no Makefile/CI/lint yet |
| 1 Core Cache | In progress | LRU + cache wrapper + TTL (passive + janitor) + tests done; LFU, concurrency test, benchmarks pending |
| 2 Network | **Done** | TCP server + RESP-flavored protocol (`PING/GET/SET/DEL/STATS/QUIT`), graceful shutdown, tests — green under `-race` |
| 3–8 | Not started | Roadmap only |

The live checklists (`PHASE1.md`, `PHASE2.md`) are the source of truth for per-phase
done-vs-pending; `ROADMAP.md`'s status table mirrors them at a glance.

## Directory layout

```
cmd/cache/main.go          # real server entrypoint (flags/env, slog, signal handling)
internal/eviction/         # storage layer: Policy interface + LRU (done) + LFU (stub)
  eviction.go              #   Policy interface
  lru.go                   #   LRU implementation
  lfu.go                   #   LFU — TODO stub, does NOT compile into a Policy yet
  lru_test.go              #   LRU unit tests
internal/cache/            # concurrency + TTL wrapper over a Policy
  cache.go                 #   Cache, Item, Options
  cache_test.go            #   Cache unit tests
internal/server/           # TCP server + RESP-flavored inline protocol
  protocol.go              #   request parser + reply writers
  commands.go              #   command dispatch (PING/GET/SET/DEL/STATS/QUIT)
  server.go                #   Server struct, accept loop, graceful shutdown
  protocol_test.go         #   parser + writer tests
  server_test.go           #   end-to-end server tests over real TCP
ROADMAP.md                 # 8-phase long-term plan
PHASE1.md                  # live checklist: core cache engine
PHASE2.md                  # live checklist: network/server layer (done)
CLAUDE.md                  # Claude Code specific guidance (keep in sync)
README.md                  # getting started + nc transcript
go.mod
.gitignore
.claude/settings.local.json
```

## Architecture (the part that matters)

Three layers, **deliberately separated**:

### `internal/eviction` — the storage layer
- `Policy` is the interface (`Set`, `Get`, `Peek`, `Delete`, `Len`, `Keys`).
- **Values live inside the policy** — there is no separate data map anywhere.
- `LRU` implements it with `container/list` + `map[string]*list.Element` for O(1) ops,
  a capacity bound, and an optional `onEvicted` callback. `NewLRU(cap, onEvicted)` returns
  `(*LRU, error)` and rejects `cap <= 0`.
- `LFU` is a TODO stub.
- Policy implementations are **not thread-safe by contract** — callers must serialize.

### `internal/cache` — the concurrency + TTL wrapper
- `Cache` holds a `sync.Mutex` + a `Policy`, serializing all access.
- `Item` wraps a caller value with an optional `ExpiresAt`; `Item.Expired()` is the TTL check.
- `NewCache(maxEntries)` defaults to LRU; `NewCacheWithOptions` injects a custom policy and
  optionally starts a background janitor.
- `Cache.Set(key, value, ttl)`; `ttl <= 0` means no expiration.

### `internal/server` — the network layer
- `Server` exposes a `*cache.Cache` over TCP; it adds **no locking of its own** — the cache
  is already concurrency-safe, so the server is just one goroutine per connection.
- `protocol.go`: `parseCommand` (inline `\r\n`-terminated, space-split) + RESP-subset reply
  writers (`+OK`, `$n\r\n`, `$-1`, `:n`, `-ERR msg`).
- `commands.go`: `dispatch` switch — `PING/GET/SET/DEL/STATS/QUIT`, case-insensitive, with
  arity + `EX` validation → `-ERR`.
- `server.go`: `ListenAndServe` (accept loop), `handleConn` (bufio read/dispatch/flush loop),
  `Shutdown(ctx)` (close listener → drain `WaitGroup` bounded by ctx → idempotent via
  `sync.Once`). Atomic counters: `commands`, `activeConns`, `errors`.

## Key invariants (do NOT violate these)

1. **`Get` mutates, `Peek` does not.** `Policy.Get` updates recency/frequency state;
   `Policy.Peek` reads without side effects. `Cache.Get` `Peek`s first for the TTL check,
   then calls `Get` **only on a live hit** — so expired entries are never promoted before
   deletion.
2. **Use `sync.Mutex`, not `RWMutex`.** Every live read mutates eviction state, so a read
   lock buys nothing. Do not "upgrade" to RWMutex.
3. **Never reintroduce a second map in `Cache`.** The policy owns storage. `Len` and the
   janitor go through `Policy.Keys()` / `Peek`, not a shadow index.
4. **TTL has two paths:**
   - *Passive*: expired keys removed inside `Cache.Get`.
   - *Active*: opt-in janitor (enabled by `Options.CleanupInterval > 0`) that sweeps via
     `Policy.Keys()` + `Peek` (Peek, so stats are untouched).
   - `Cache.Close()` stops the janitor; idempotent (`sync.Once`) and a no-op when none was
     started. The cache stays readable after `Close`.
5. **`NewLRU` returns an error for `cap <= 0`** — constructors validate, don't panic.
6. **Values are strings on the wire.** `Cache` stores `any`; the server's `GET` type-asserts
   to `string` and errors if not. `SET` always stores a `string`.
7. **Inline parsing blocks spaces/binary in values.** The RESP array parser is a Phase 8
   stretch — don't "fix" this by changing the wire format mid-phase.

## Commands

```bash
go test ./...                        # all tests (currently green)
go test -race ./...                  # race detector — REQUIRED for any concurrency work
go test ./internal/server/ -run TestServerPing
go vet ./...
go run ./cmd/cache                   # runs the TCP server (default :6380)
```

Config flags (each with an env-var fallback): `-addr`/`CACHE_ADDR` (`:6380`),
`-max-entries`/`CACHE_MAX_ENTRIES` (`1024`), `-cleanup-interval`/`CACHE_CLEANUP_INTERVAL`
(`1m`, `0` disables the janitor).

## Conventions & style

- Public docs (`ROADMAP.md`, `PHASE1.md`, `PHASE2.md`, `README.md`) use emoji headers and
  checkboxes; keep that style when editing them.
- Phase checklists are the source of truth for done-vs-pending. **Update the relevant
  `PHASE*.md` checkboxes as work lands**, and keep `CLAUDE.md` / `AGENTS.md` / `README.md`
  in sync when the architecture changes.
- Tests are plain stdlib `testing` (table-driven where sensible), no assertion library.
- API returns `(value, ok)` style; `Cache.Get` returns `(any, bool)`.
- The server test helper (`internal/server/server_test.go`) dials `127.0.0.1:0` and reads
  replies with a small RESP reader — reuse it, don't reinvent it.

## What to do next (if unasked)

1. Finish Phase 1 leftovers: implement `LFU`, add `TestCacheConcurrency` under `-race`, and
   write `BenchmarkCacheSet` / `BenchmarkCacheGet` (PHASE1.md §3–4).
2. Then start Phase 3 (distributed routing / consistent hashing) per `ROADMAP.md`.

## Gotchas

- `Cache.Get` type-asserts the stored value to `Item`; a failed assertion is treated as a
  miss (defensive, keeps the policy's value type opaque).
- The janitor's `Keys()` returns a snapshot slice, so deleting mid-iteration is safe.
- The server's `DEL` uses `Cache.Get` to detect presence (also folds in TTL). It promotes
  recency, but the key is deleted immediately after, so the effect is moot.
- `STATS` reports `commands` including the `STATS` command itself (the counter increments
  before dispatch).
- `Shutdown`'s listener access is mutex-guarded (no `-race`), but it assumes
  `ListenAndServe` has already bound; in the real `main` flow this always holds because the
  signal handler only fires after startup.
