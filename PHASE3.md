# Phase 3: Distributed Routing (Static Cluster) Progress Tracker

Turn the single-node cache + TCP server into a **static cluster**: multiple nodes, each
running the same `internal/server`, agree on who owns every key via **consistent hashing**,
and forward requests to the right owner. Depends on Phase 1 (`cache.Cache`) and Phase 2
(`internal/server`, `cmd/cache/main.go`).

This phase is about **routing**, not availability. Nodes know the full peer list up front
(no discovery), and no data is replicated or migrated yet — if a node dies, its keys are
simply unreachable until it returns (that is Phase 4/5).

**Decisions:**
- **Consistent hashing with virtual nodes** for key → owner mapping (static ring).
- **FNV-1a (64-bit)** as the hash function — fast, deterministic, non-cryptographic.
- **150 virtual nodes** per physical node (tunable) to smooth distribution / avoid hotspots.
- **Static peer list** via `-peers`/`CACHE_PEERS` (`id=addr,...`) + `-node-id`/`CACHE_NODE_ID`.
- **Server-side proxy forwarding**: any node accepts any key; non-owner keys are forwarded
  to the owner and the reply relayed. The client stays dumb (client-side routing is a later
  optimization).
- **`pkg/client`**: a minimal RESP client used internally for forwarding (and later by
  `cache-cli` in Phase 7). This is where a production-grade RESP *reply reader* finally lands
  (it was test-only until now).
- **`internal/ring`** keeps the ring pure (keys → node ID, no addresses); a peer registry
  (node ID → address) lives in `internal/config`. This lets integration tests bind listeners
  on `:0` and inject real addresses afterwards.

---

## Roadmap & Checklist

### 1. Consistent Hash Ring (`internal/ring/`)

Pure, dependency-free, unit-testable. The ring maps a key to an **owner node ID**; it does
not know about addresses.

- [ ] Hash function: `hash(s string) uint64` using FNV-1a (`hash/fnv`, `fnv.New64a`).
- [ ] Virtual-node placement: physical node `id` occupies points `hash(id + "#" + i)` for
      `i` in `[0, vnodes)`.
- [ ] `type point struct { hash uint64; node string }`; the ring holds a `[]point` kept
      **sorted by hash**.
- [ ] `New(vnodes int) *Ring`
- [ ] `Add(nodeID string)` / `Remove(nodeID string)` (removes every virtual node of that ID)
- [ ] `Get(key string) (string, bool)` — hash the key, `sort.Search` for the first point
      `>= keyHash`, wrap to `points[0]` if none; return that point's node ID.
- [ ] `Nodes() []string` — unique node IDs currently on the ring.
- [ ] Unit tests (`ring_test.go`):
  - [ ] Determinism: same key → same owner across repeated `Get` calls and fresh rings.
  - [ ] Wrap-around: a key that hashes past the largest point maps to the smallest.
  - [ ] `Remove` fully drops a node (no residual virtual nodes).
  - [ ] Distribution: many random keys land roughly evenly across nodes (loose bound).
  - [ ] **Minimal remapping**: after adding one node to an N-node ring, only ~`1/(N+1)` of
        the existing keys change owner (assert a generous upper bound).

### 2. Static Cluster Config (`internal/config/`)

Extracted from `main.go` now that config grows beyond three flags.

- [ ] `type Node struct { ID, Addr string }`
- [ ] `type Config struct { NodeID string; Peers map[string]string }` (ID → addr).
- [ ] Parse flags/env: `-node-id`/`CACHE_NODE_ID`, `-peers`/`CACHE_PEERS`
      (comma-separated `id=addr`).
- [ ] Validation: peer list non-empty, self present in peers, no duplicate IDs, addresses
      parseable (`net.SplitHostPort` or `net.ResolveTCPAddr`).
- [ ] `Self()` helper returning this node's `Node`.
- [ ] Unit tests: valid parse, missing self, duplicate ID, malformed `id=addr`, empty list.

### 3. Forwarding Client (`pkg/client/`)

A tiny, reusable RESP client. For Phase 3 it only needs to forward and return the **raw**
reply bytes (the server relays them unchanged), but a normalized reader is useful later.

- [ ] `type Client struct { addr string; mu sync.Mutex; conn net.Conn }`
- [ ] `New(addr string) *Client`
- [ ] `Do(cmd string) ([]byte, error)` — dial lazily (or on first use), write `cmd + "\r\n"`,
      read **exactly one** RESP reply, return its raw bytes.
- [ ] RESP reply reader (handles `+`, `-`, `:`, `$` bulk and `$-1` null; reads the exact
      byte count for bulks).
- [ ] Connection reuse under `mu`; reconnect on a dead connection; `Close()`.
- [ ] Unit tests: round-trip each reply type against an in-process `server` or a stub conn;
      reconnect after the peer closes.

### 4. Routing & Forwarding (`internal/server/`)

- [ ] `Server` gains `selfID string`, `ring *ring.Ring`, `peers map[string]string`
      (or `*config.Config`), and a forwarding client (or a tiny per-peer client cache).
- [ ] Command classifier — keyed vs local:
      | Command | Key | Behavior |
      |---------|-----|----------|
      | `GET` / `SET` / `DEL` | `args[1]` | route by owner |
      | `PING` / `QUIT` | — | local |
      | `STATS` | — | local (this node only; aggregate is a later phase) |
- [ ] Route: if keyed and `ring.Get(key) != selfID`, forward the **raw command line** to
      `peers[owner]` via `pkg/client` and write the relayed reply back; otherwise `dispatch`
      locally.
- [ ] Log routing decisions at `Debug` (forwarded key → owner, local hit).
- [ ] Unit tests: routing decision (local vs forward) via table tests; forwarded reply is
      relayed byte-for-byte.

### 5. Wire-up (`cmd/cache/main.go`)

- [ ] Parse `-node-id` + `-peers` (via `internal/config`) alongside the existing flags.
- [ ] Build the `ring.Ring`, `Add` every peer ID.
- [ ] Construct `server.New` with self ID + ring + peer map.
- [ ] Log cluster membership (peer count, self ID/addr) at startup.

### 6. Integration Tests (3-node in-process)

- [ ] Small `server` refactor to **separate bind from serve** (e.g. `Serve(ln net.Listener)`
      or expose the bound listener) so the test can bind three `127.0.0.1:0` listeners,
      collect real addrs, build the shared peer map + ring, then start the accept loops.
- [ ] Helper: build N caches + N servers sharing one ring + peer map.
- [ ] `TestClusterSetGet` — `SET` via node A, `GET` via node B returns the value (forwarding
      from a non-owner).
- [ ] `TestClusterOwnership` — the same key resolves to the same owner regardless of which
      node receives the request.
- [ ] `TestClusterDel` — `DEL` via a non-owner removes the key cluster-wide.
- [ ] `TestClusterStats` — `STATS` reports the *local* node's counters (not forwarded).
- [ ] `go test -race ./...` across the whole repo.

### 7. Docs

- [ ] `README.md`: "Run a cluster" section — start 3 nodes, `nc` demo showing a key being
      routed to its owner and read back through a different node.
- [ ] `ROADMAP.md`: Phase 3 status → In progress / Done; link `PHASE3.md`.
- [ ] `CLAUDE.md` + `AGENTS.md`: add `internal/ring`, `internal/config`, `pkg/client`;
      document the "route or forward" invariant.

---

## Definition of Done (from ROADMAP)

Key `foo` always lands on the same owner; adding a node changes minimal key assignments;
proxy forwarding works end-to-end (`SET`/`GET`/`DEL` through any node).

---

## Notes & Learnings

* **Why consistent hashing (vs `hash(key) % N`).** Modulo hashing is simplest, but changing
  N (adding/removing a node) remaps almost *every* key — for a cache that means a full cold
  start on every membership change. Consistent hashing maps both nodes and keys onto the same
  circular number space; a key is owned by the *first node clockwise*. Adding/removing a node
  only reassigns the keys between that node and its predecessor — roughly `1/N` of them.

* **Virtual nodes fix the distribution problem.** With one point per node, the ring segments
  are uneven and a few nodes can own a disproportionate share (hotspots), especially with few
  nodes. Giving each node many points (`id + "#" + i`) evens out the arc lengths, so keys
  spread uniformly and adding a node takes load from *several* neighbours rather than one.

* **Hash choice.** FNV-1a 64-bit is fast and plenty uniform for this; you don't need a
  cryptographic hash (SHA-256) here — you're not defending against adversarial keys, just
  spreading them. The only hard requirement: use the **same** hash for nodes and keys.

* **Ring over node IDs, not addresses.** Keeping the ring pure (`key → node ID`) means it has
  no opinion about networking, and tests can build it without ports. Resolve `node ID → addr`
  only at forward time. This is also what lets the 3-node test bind `:0` and inject real
  addresses afterwards.

* **Proxy forwarding vs client-side routing.** Server-side forwarding means any node is a
  valid entry point and the client stays trivial — at the cost of one extra network hop for
  non-local keys. The alternative (teach the client the ring so it dials the owner directly)
  is faster but moves cluster logic into every client; keep it as a future option, not now.

* **Routing ≠ availability.** Consistent hashing only decides *ownership*. With no
  replication, a dead node's keys are simply gone until it returns — that's expected here and
  is exactly what Phase 5 (replication) addresses. Don't be tempted to add replication now.

* **Static membership ≠ dynamic membership.** Every node loads the same peer list from config;
  there is no gossip, heartbeat, or rebalance. Adding a node means editing config and
  restarting, and data is *not* migrated (keys that remap to the new node are cold). Phase 4
  makes this dynamic.

* **`DEL` detection caveat.** To return `:1` vs `:0`, `DEL` checks presence via `Cache.Get`
  (which also folds in TTL). When `DEL` is forwarded, the *owner* does this check, so the
  reply count is still correct cluster-wide.

* **RESP reader becomes production code.** Up to now, reading replies existed only in the
  server's *test* helper. `pkg/client` needs it for real, so implement a clean, reusable
  reader there and (optionally) refactor the test helper to share it.
