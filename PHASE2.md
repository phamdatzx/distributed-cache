# Phase 2: Network & Server Layer Progress Tracker

Expose `internal/cache` over TCP with a small Redis-like text protocol. Replaces the
`cmd/cache/main.go` demo with a real server. Depends on Phase 1 (`Cache`, `Close`).

**Decisions:** custom text protocol (inline commands, RESP-flavored replies); config via
flags with env-var fallback; a `pkg/client` is deferred to Phase 3.

---

## Roadmap & Checklist

### 1. Wire Protocol (`internal/server/protocol.go`)

Line-based inline commands, `\r\n`-terminated. A RESP-subset parser is a Phase 8 stretch.

- [ ] Request parse: read a line, split on spaces into `[]string` (verb + args).
      Documented limitation: values cannot contain spaces yet.
- [ ] Response writers, RESP-flavored subset:
  - [ ] Simple string `+OK\r\n`, `+PONG\r\n`
  - [ ] Bulk string `$<n>\r\n<bytes>\r\n` (GET hit)
  - [ ] Null bulk `$-1\r\n` (GET miss)
  - [ ] Integer `:<n>\r\n` (DEL count)
  - [ ] Error `-ERR <message>\r\n` (unknown verb, arity, bad EX)
- [ ] Unit tests: round-trip each type; malformed input → error.

### 2. Command Set (`internal/server/commands.go`)

| Command | Args | Reply | Cache call |
|---------|------|-------|------------|
| `PING`  | — | `+PONG` | — |
| `GET`   | key | bulk / null | `Cache.Get` |
| `SET`   | key value [`EX` seconds] | `+OK` | `Cache.Set` (ttl from EX, else 0) |
| `DEL`   | key | `:0` / `:1` | `Cache.Delete` (+ `Get` to detect presence) |
| `STATS` | — | bulk (counters) | server counters |
| `QUIT`  | — | `+OK` then close | — |

- [ ] Arity + `EX` validation → `-ERR`; verb match case-insensitive.

### 3. TCP Server (`internal/server/server.go`)

- [ ] `Server` struct: `*cache.Cache`, `*slog.Logger`, `net.Listener`, `sync.WaitGroup`
      (in-flight conns), `atomic` counters (commands, active conns, errors), `done` chan.
- [ ] `New(c *cache.Cache, logger *slog.Logger) *Server`
- [ ] `ListenAndServe(addr string) error` — bind, accept loop, one goroutine per conn.
- [ ] `handleConn` — `bufio.Reader`/`Writer` loop: parse → dispatch → write; break on
      `QUIT`, EOF, or write error; always close + `wg.Done` + decrement active count.
- [ ] `Shutdown(ctx context.Context) error` — close listener (unblocks `Accept`), then
      `wg.Wait` bounded by `ctx`; existing connections drain, new ones rejected.
- [ ] Accept loop ignores `net.ErrClosed` after shutdown; logs other accept errors.

### 4. Wire-up (`cmd/cache/main.go`)

- [ ] Delete the `SafeCounter` demo.
- [ ] Config: `flag` vars with defaults from `envOr(key, fallback)`:
      `-addr`/`CACHE_ADDR` (`:6380`), `-max-entries`/`CACHE_MAX_ENTRIES` (`1024`),
      `-cleanup-interval`/`CACHE_CLEANUP_INTERVAL` (`1m`; `0` disables janitor).
- [ ] Build `cache.NewCacheWithOptions`, `slog.NewTextHandler(os.Stderr, ...)`.
- [ ] `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`; on cancel call
      `srv.Shutdown(timeoutCtx)` then `c.Close()`.
- [ ] Non-zero exit on fatal listen error.

### 5. Observability (`log/slog`)

- [ ] Structured logs: server start (addr, max-entries), shutdown (drained conn count),
      per-command errors. Connection open/close at `Debug`.
- [ ] Counters surfaced via `STATS`: total commands, active connections, error count.

### 6. Testing & Validation (`internal/server/*_test.go`)

- [ ] `protocol_test.go`: parse + writer round-trips, malformed lines.
- [ ] `server_test.go` with a raw `net.Dial` helper on `127.0.0.1:0`:
  - [ ] `TestServerPing`
  - [ ] `TestServerSetGet`
  - [ ] `TestServerGetMiss` (null bulk)
  - [ ] `TestServerDel` (`:1` then `:0`)
  - [ ] `TestServerSetEX` — set with `EX`, wait, `GET` → null (passive expiry path)
  - [ ] `TestServerUnknownCommand` / arity errors → `-ERR`
  - [ ] `TestServerConcurrentClients` under `go test -race`
  - [ ] `TestServerGracefulShutdown` — in-flight request completes, `Accept` stops
- [ ] `go test -race ./...`, `go vet ./...`

### 7. Docs

- [ ] `README.md`: replace the false "Run the Skeleton Server" section with real
      `go run ./cmd/cache` usage + a `nc localhost 6380` transcript.
- [ ] `ROADMAP.md`: Phase 2 status → In progress / Done as work lands.
- [ ] `CLAUDE.md`: new `internal/server` layer in Architecture; `go run` no longer a
      demo; add `nc` / protocol notes.

---

## Definition of Done (from ROADMAP)

`nc localhost 6380` can `SET` / `GET` / `DEL` / `PING`; `Ctrl-C` drains connections and
exits cleanly; `go test -race ./...` green.

## Notes & Learnings

* `cache.Cache` is already concurrency-safe — the server adds no locking, just one
  goroutine per connection.
* Values are strings on the wire; `Cache` stores `any`, so `GET` type-asserts to `string`.
* Inline parsing (space-split) blocks spaces/binary in values — the RESP array parser in
  Phase 8 removes that limit.
* Keep config in `main.go` for now; extract `internal/config` when Phase 3 adds peer lists.
