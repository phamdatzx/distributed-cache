# Long-Term Roadmap: Distributed Cache in Go

A phased roadmap to design, build, and productionize a high-performance distributed cache in Go. Each phase is a meaningful milestone in distributed systems, concurrency, networking, and engineering practice — structured so the project reads well on a CV and holds up in system design interviews.

**Related docs:** [PHASE1.md](./PHASE1.md) (Phase 1 checklist) · [README.md](./README.md)

---

## Current Status

| Phase | Status | Notes |
|-------|--------|-------|
| 0 — Foundation | 🟡 Partial | Go module, layout, and docs exist; CI/linting not yet set up |
| 1 — Core Cache | 🟡 In progress | Basic `Get`/`Set` with TTL check on read; no LRU, janitor, tests, or benchmarks yet |
| 2 — Network Layer | ⬜ Not started | `main.go` is still a concurrency demo |
| 3 — Clustering | ⬜ Not started | — |
| 4 — Discovery | ⬜ Not started | — |
| 5 — Replication | ⬜ Not started | — |
| 6 — Durability | ⬜ Not started | — |
| 7 — Production | ⬜ Not started | — |

**Suggested focus now:** Finish Phase 1 (LRU eviction, active TTL cleanup, tests, benchmarks). See [PHASE1.md](./PHASE1.md).

---

## Roadmap Overview

Phases build on each other. Observability and CI are introduced early so every later phase ships with tests and metrics — not bolted on at the end.

```
┌─────────────────────────┐
│  Phase 0: Foundation    │ ◄── Layout, Makefile, CI, linting, ADRs
└────────────┬────────────┘
             ▼
┌─────────────────────────┐
│  Phase 1: Cache Core    │ ◄── Thread-safe store, LRU/LFU, TTL, benchmarks
└────────────┬────────────┘
             ▼
┌─────────────────────────┐
│  Phase 2: Network Layer │ ◄── TCP server, command protocol, concurrency
└────────────┬────────────┘
             ▼
┌─────────────────────────┐
│ Phase 3: Clustering     │ ◄── Consistent hashing, static peers, forwarding
└────────────┬────────────┘
             ▼
┌─────────────────────────┐
│ Phase 4: Discovery      │ ◄── Heartbeats, failure detection, rebalancing
└────────────┬────────────┘
             ▼
┌─────────────────────────┐
│ Phase 5: Replication    │ ◄── RF, quorums, consistency trade-offs
└────────────┬────────────┘
             ▼
┌─────────────────────────┐
│ Phase 6: Durability     │ ◄── WAL, snapshots, crash recovery
└────────────┬────────────┘
             ▼
┌─────────────────────────┐
│ Phase 7: Production     │ ◄── Docker, load tests, Grafana, CLI, docs
└─────────────────────────┘
             │
             ▼ (optional)
┌─────────────────────────┐
│ Phase 8: Stretch Goals  │ ◄── RESP compat, chaos tests, hot-key mitigation
└─────────────────────────┘
```

**Design principle:** Static cluster + routing (Phase 3) before dynamic discovery (Phase 4). Replication (Phase 5) before persistence (Phase 6) — understand in-memory HA before adding disk I/O complexity.

---

## Phase Detail & Definition of Done

Each phase is **done** when: code is merged, tests pass (`go test -race ./...`), benchmarks or integration checks are documented, and README/architecture notes are updated.

---

### Phase 0: Project Foundation

**Goal:** Establish repo hygiene so every future phase is testable and reviewable from day one.

- [ ] Standard layout: `cmd/`, `internal/`, `pkg/` (only if you expose a public client API)
- [ ] `Makefile` with `test`, `bench`, `lint`, `run` targets
- [ ] GitHub Actions: `go test -race`, `golangci-lint`, build on every push
- [ ] Architecture Decision Records (`docs/adr/`) for non-obvious choices (eviction policy, protocol, consistency model)
- [ ] README: Mermaid architecture diagram (even if it evolves each phase)

**Done when:** CI is green on main; a new contributor can clone, `make test`, and understand the project in 5 minutes.

---

### Phase 1: Core Cache Engine (Single Node)

**Goal:** A correct, thread-safe, bounded in-memory cache — the foundation everything else wraps.

Track detailed tasks in [PHASE1.md](./PHASE1.md).

| Area | Tasks |
|------|-------|
| **Eviction** | LRU via doubly-linked list + map (O(1) get/set/evict). Optional LFU behind an `EvictionPolicy` interface (`internal/eviction/`). |
| **Concurrency** | `sync.RWMutex` wrapper first; optional sharded buckets later if benchmarks show lock contention. |
| **TTL** | Passive expiry on `Get`; active cleanup via background janitor goroutine with clean shutdown. |
| **API** | `NewCache`, `Get`, `Set`, `Delete`, `Len`, `Close` (stop janitor). |
| **Quality** | Table-driven unit tests, concurrency test with `-race`, `BenchmarkGet`/`BenchmarkSet` with `-benchmem`. |

**Avoid:** Chasing 100% coverage — aim for meaningful coverage of eviction, TTL, and concurrent access paths instead.

**Done when:** LRU eviction works under load; expired keys are removed passively and actively; benchmarks are recorded in README or `docs/benchmarks.md`.

---

### Phase 2: Network & Server Layer

**Goal:** Expose the cache over the network with a well-defined command set.

| Area | Tasks |
|------|-------|
| **Protocol** | Pick one primary path (recommended for learning + CV): |
| | **A — Custom text protocol** (Redis-like): `GET key`, `SET key value EX 60`, `DEL key`, `PING`. Easiest to debug. |
| | **B — HTTP/REST**: Fast to ship; good for demos, weaker for raw throughput story. |
| | **C — gRPC + protobuf**: Strong typing; heavier upfront cost. |
| | **Stretch (Phase 8):** Subset of [RESP](https://redis.io/docs/reference/protocol-spec/) for Redis-cli compatibility — high CV impact. |
| **Server** | TCP listener, one goroutine per connection (or connection pool later), graceful shutdown on `SIGINT`/`SIGTERM`. |
| **Wire-up** | Replace `cmd/cache/main.go` demo with real server bootstrapping `internal/cache`. |
| **Observability (early)** | Structured logging (`slog`); basic counters: ops/sec, active connections, errors. |

**Done when:** A client (netcat, curl, or small Go client) can `SET`/`GET`/`DEL` keys against a running server; server shuts down cleanly.

---

### Phase 3: Distributed Routing (Static Cluster)

**Goal:** Multiple nodes form a cluster; keys route to the correct owner via consistent hashing.

| Area | Tasks |
|------|-------|
| **Hash ring** | Consistent hashing with virtual nodes to reduce hotspots. |
| **Static config** | Peer list from config file or env vars (no gossip yet — keep it simple). |
| **Routing** | Client hits any node; if key belongs elsewhere, node forwards internally and returns the result (proxy pattern). |
| **Integration test** | 3-node cluster in-process or via `docker-compose` with static peers. |

**Done when:** Key `foo` always lands on the same owner; adding a node in config changes minimal key assignments; proxy forwarding works end-to-end.

---

### Phase 4: Dynamic Cluster Management & Discovery

**Goal:** Nodes join and leave without manual config edits; the cluster detects failures.

| Area | Tasks |
|------|-------|
| **Failure detection** | Periodic heartbeats / health endpoints; mark peers dead after timeout. |
| **Discovery** | Start with static + health checks; advance to Gossip ([hashicorp/memberlist](https://github.com/hashicorp/memberlist)) or service registry (Consul/etcd) if you want deeper learning. |
| **Rebalancing** | On membership change, update the ring and migrate keys in the background (with rate limiting to avoid storms). |
| **Edge cases** | Document behavior during network partitions (even if v1 is "best effort"). |

**Done when:** A node can be stopped and the cluster continues serving keys owned by survivors; new nodes receive migrated data over time.

---

### Phase 5: High Availability & Replication

**Goal:** Survive node crashes without losing recently written data.

| Area | Tasks |
|------|-------|
| **Replication factor** | Replicate each key to N successors on the hash ring. |
| **Write path** | Primary-coordinated writes; sync vs async replication (implement both, default async). |
| **Read path** | Quorum reads (`R + W > N`) for stronger consistency; single-replica reads for speed (document trade-off). |
| **Conflict handling** | Last-Write-Wins with hybrid logical clocks or HLC timestamps — sufficient for a cache; vector clocks are optional deep-dive. |
| **Hinted handoff** (optional) | Temporary write buffer when a replica is down. |

**Done when:** Killing a replica does not lose acknowledged writes (under your chosen W/R policy); split-brain behavior is documented in an ADR.

---

### Phase 6: Durability & Persistence

**Goal:** Survive full process restarts without cold-cache penalty.

| Area | Tasks |
|------|-------|
| **WAL / AOF** | Append `SET`/`DEL` to disk before ACK (configurable `fsync` policy: always / every sec / never). |
| **Snapshots** | Periodic RDB-style point-in-time dump to compact the log. |
| **Recovery** | Load latest snapshot + replay WAL on startup. |
| **Trade-offs** | Document latency impact of sync writes vs durability guarantees. |

**Done when:** Restart a node; previously written keys are restored; WAL does not grow unbounded.

---

### Phase 7: Production Readiness & Observability

**Goal:** Package and present the project at professional standards.

| Area | Tasks |
|------|-------|
| **Metrics** | Prometheus endpoint: hit/miss ratio, evictions, expirations, latency histograms (p50/p99), cluster health. |
| **Dashboards** | Grafana dashboard + screenshot in README. |
| **CLI** | `cache-cli` for interactive GET/SET and cluster status. |
| **Containers** | `Dockerfile` + `docker-compose.yml` for multi-node local cluster. |
| **Load testing** | [vegeta](https://github.com/tsenart/vegeta) or k6 scripts; publish results in `docs/benchmarks.md`. |
| **Docs** | Architecture diagram, runbook (start/stop/scale), comparison table vs Redis/Memcached (what you implemented vs deferred). |

**Done when:** Someone can `docker compose up`, run load tests, view Grafana, and read docs without asking you questions.

---

### Phase 8: Stretch Goals (Optional, High CV Impact)

Pick 1–2 after Phase 7 — depth beats breadth on a resume.

- **RESP compatibility** — use `redis-cli` against your server
- **Cache stampede protection** — singleflight on hot keys
- **TLS + token auth** on client connections
- **Chaos testing** — inject latency/partitions (Toxiproxy), verify quorum behavior
- **Hot-key detection** — sample access patterns, optional local read replicas
- **Multi-tenancy** — key namespaces with per-tenant limits

---

## Suggested Timeline (Learning Pace)

Rough guide — adjust to your schedule. Depth in one phase beats rushing to Phase 7.

| Phase | Estimated effort |
|-------|------------------|
| 0 — Foundation | 1–3 days |
| 1 — Core Cache | 2–4 weeks |
| 2 — Network | 2–3 weeks |
| 3 — Static cluster | 3–4 weeks |
| 4 — Discovery | 3–5 weeks |
| 5 — Replication | 4–6 weeks |
| 6 — Durability | 3–4 weeks |
| 7 — Production | 2–3 weeks |
| 8 — Stretch | Ongoing |

---

## Learning Resources

| Topic | Resource |
|-------|----------|
| LRU cache design | [Design a LRU Cache (LeetCode 146)](https://leetcode.com/problems/lru-cache/) — then implement without `container/list` for deeper understanding |
| Consistent hashing | Dynamo paper (2007); "Introduction to Consistent Hashing" (Medium / system design primers) |
| Gossip / SWIM | hashicorp/memberlist docs; "SWIM: Scalable Weakly-consistent Infection-style Process Group Membership Protocol" |
| Redis internals | [Redis architecture docs](https://redis.io/docs/reference/); antirez blog posts on persistence |
| Go concurrency | [Go Memory Model](https://go.dev/ref/mem); "Concurrency is not Parallelism" (Rob Pike) |
| Quorum consistency | Dynamo / Cassandra replication sections; Jepsen talks for intuition on what *not* to claim |

---

## CV & Portfolio Strategy

A distributed cache is a strong portfolio piece **if you can explain trade-offs**, not just list features.

### README must-haves (by Phase 7)

1. **Architecture diagram** (Mermaid) showing client → proxy node → hash ring → replicas
2. **Benchmark table** — single-node ops/sec, clustered ops/sec, p99 latency, hardware spec
3. **Feature matrix** — your cache vs Redis/Memcached (honest about gaps)
4. **Demo** — GIF or asciinema of `cache-cli` + 3-node cluster
5. **Design decisions** — link to ADRs (eviction, consistency, protocol)

### Blog post ideas (1 post per major phase)

1. "Building an O(1) LRU cache in Go: concurrency and the `-race` detector"
2. "From single node to cluster: consistent hashing and request forwarding"
3. "Replication and quorum reads in a distributed cache — what I got wrong first"
4. "WAL vs snapshot recovery: latency measurements on real hardware"

### CV bullet templates (fill in after benchmarks)

- Designed and implemented a distributed in-memory cache in Go (~**X**K ops/sec single-node, ~**Y**K ops/sec 3-node cluster) with consistent hashing, configurable replication, and quorum reads.
- Built thread-safe LRU eviction with active/passive TTL expiration; documented **X**% memory reduction vs unbounded map in load tests.
- Implemented peer failure detection and key rebalancing; validated behavior with integration tests and partition injection.
- Shipped production tooling: Prometheus metrics, Grafana dashboards, Docker Compose cluster, and CI pipeline with race-detector tests.

### Interview prep

For each phase, prepare answers to:

- **Why this design?** (alternatives considered)
- **What breaks?** (partition, clock skew, concurrent eviction + TTL)
- **How did you test it?** (unit, integration, `-race`, load test)
- **What would you do differently at 10× scale?**

---

## What Changed in This Roadmap (Review Notes)

| Change | Reason |
|--------|--------|
| Added **Phase 0** (foundation, CI, ADRs) | Professional repos need CI early; avoids rework in Phase 7 |
| Added **status table** | Makes progress visible for you and recruiters |
| Moved **observability** into Phase 2 | Metrics/logging should grow with the system, not appear last |
| Split **static cluster (Phase 3)** from **discovery (Phase 4)** | Smaller, testable milestones; less overwhelming |
| Softened **100% coverage** | Misleading goal; race-safe meaningful tests matter more |
| Added **Phase 8 stretch goals** | RESP/chaos/TLS are CV differentiators without blocking core delivery |
| Added **timeline + learning resources** | Long-term learning project needs pacing guidance |
| Linked **PHASE1.md** | Detailed checklist stays in phase doc; roadmap stays high-level |
| Added **interview prep** section | CV value comes from being able to defend design choices |
