# Long-Term Roadmap: Distributed Cache in Go

This document outlines a phased, step-by-step roadmap to design, build, and productionize a high-performance distributed cache in Go. Each phase represents a significant milestone in distributed systems design, concurrency, network programming, and engineering practices—making this a standout addition to your professional CV/resume.

---

## 🗺️ Roadmap Overview

```
┌────────────────────────┐
│  Phase 1: Cache Core   │ ◄── Single-node, thread-safe, LRU/LFU eviction, TTL
└───────────┬────────────┘
            ▼
┌────────────────────────┐
│ Phase 2: Network Layer │ ◄── Custom TCP protocol, HTTP/REST, or gRPC
└───────────┬────────────┘
            ▼
┌────────────────────────┐
│  Phase 3: Clustering   │ ◄── Consistent hashing, peer-to-peer routing
└───────────┬────────────┘
            ▼
┌────────────────────────┐
│ Phase 4: Peer Discovery│ ◄── Heartbeats, auto-rebalancing, Gossip protocol
└───────────┬────────────┘
            ▼
┌────────────────────────┐
│ Phase 5: Replication   │ ◄── Primary/Replica, consistency models, Quorums
└───────────┬────────────┘
            ▼
┌────────────────────────┐
│  Phase 6: Durability   │ ◄── Write-Ahead Log (WAL), State Snapshots
└───────────┬────────────┘
            ▼
┌────────────────────────┐
│ Phase 7: Production    │ ◄── Docker, CI/CD, Prometheus metrics, Grafana
└────────────────────────┘
```

---

## 🎯 Phase Detail & Implementation Steps

### Phase 1: Core Cache Engine (Single Node)
**Goal:** Build a thread-safe, high-performance, in-memory cache engine.
*   **Thread-Safety:** Implement a custom concurrency wrapper using Go's `sync.RWMutex` (or explore sharded maps like `concurrent-map` patterns to reduce lock contention).
*   **Eviction Policies:**
    *   **LRU (Least Recently Used):** Implement using a doubly linked list and a hash map (O(1) lookups and updates).
    *   **LFU (Least Frequently Used):** For advanced practice, add an optional LFU policy.
*   **Time-to-Live (TTL):** Implement automatic key expiration using background cleanup goroutines (active/passive expiration).
*   **Benchmarks & Testing:**
    *   Write 100% test coverage for edge cases (cache eviction, concurrent writes, TTL expiration).
    *   Write Go `testing.B` benchmarks (`BenchmarkGet`, `BenchmarkSet`) to measure throughput and memory allocations.

### Phase 2: Network & Server Layer
**Goal:** Expose the cache over a network so external clients can read and write data.
*   **Protocol Choice:** Select a protocol.
    *   *Option A (Easy/Interoperable):* HTTP/REST or JSON-RPC.
    *   *Option B (High Performance):* Custom binary TCP protocol or gRPC with Protocol Buffers.
*   **Command Parser:** Write a robust lexer/parser for cache commands (e.g., `GET`, `SET`, `DELETE`, `EXPIRE`).
*   **High Concurrency:** Configure the server to handle thousands of concurrent TCP connections efficiently using goroutines.

### Phase 3: Distributed Basics & Routing (The Ring)
**Goal:** Turn your single node into a collaborative cluster of cache nodes.
*   **Consistent Hashing:** Implement a consistent hashing ring. This ensures keys are distributed evenly across peer nodes and minimizes key remapping when nodes join or leave the cluster.
*   **Virtual Nodes:** Add virtual node support to consistent hashing to prevent hotspots (uneven distribution).
*   **Peer Routing (P2P):** If a client requests a key from Node A, but Node B owns that key, Node A should:
    *   Look up the owner of the key using the consistent hashing ring.
    *   Fetch the key from Node B (via the internal peer communication protocol) and return it to the client.

### Phase 4: Dynamic Cluster Management & Discovery
**Goal:** Make the cluster dynamic and resilient to nodes joining and leaving.
*   **Node Discovery:**
    *   *Basic:* Static list of peer addresses in a configuration file.
    *   *Advanced:* Implement dynamic peer discovery using a Gossip Protocol (e.g., integrating `hashicorp/memberlist`) or dynamic registration (using Consul or etcd).
*   **Failure Detection:** Use periodic heartbeats or health checks to detect dead nodes.
*   **Rebalancing:** When a new node joins or an existing node dies, automatically adjust the hashing ring and trigger background migration of keys to ensure data is where it belongs.

### Phase 5: High Availability & Replication
**Goal:** Protect against data loss if one or more cache nodes crash.
*   **Replication Factor:** Replicate each key-value pair across $N$ successive nodes in the consistent hashing ring.
*   **Replication Strategy:**
    *   *Active-Passive:* Writes go to the primary node, which asynchronously or synchronously replicates them to replica nodes.
    *   *Quorum Writes/Reads:* Use a basic quorum system ($W + R > N$) to guarantee strong consistency across cluster partitions.
*   **Conflict Resolution:** Handle split-brain or outdated write scenarios using vector clocks or a "Last-Write-Wins" policy.

### Phase 6: Durability & Persistence (Advanced)
**Goal:** Enable your cache to survive complete system restarts without losing hot data.
*   **Write-Ahead Log (WAL) / Append-Only File (AOF):** Persist every write command (`SET`, `DEL`) to an append-only disk log before acknowledging the write to the client.
*   **Snapshots (RDB):** Create background snapshots of the in-memory data to truncate/compact the WAL file, preventing it from growing indefinitely.
*   **Recovery Engine:** On startup, read the snapshot file and replay any transactions recorded in the WAL after the snapshot was taken.

### Phase 7: Production Readiness, CI/CD & Observability
**Goal:** Package, instrument, and deliver the project to professional standards.
*   **CLI Client:** Build an interactive CLI tool (`cache-cli`) to run queries against the cluster.
*   **Prometheus Metrics:** Expose key system metrics via an HTTP endpoint:
    *   Cache hit/miss ratios.
    *   Active client connections.
    *   Keys evicted, expired, and stored.
    *   Operation latencies (P50, P99).
*   **Dockerization:** Standardize node deployments using a `Dockerfile` and setup multi-node integration environments using `docker-compose.yml`.
*   **CI/CD Pipeline:** Set up GitHub Actions to run linters (`golangci-lint`), execute tests, and automatically build/tag docker images.

---

## 📈 CV & Portfolio Strategy

Having this project on your CV is great, but presenting it correctly makes it standout:

1.  **System Design Diagram:** Place a clear, professional architecture diagram at the top of your `README.md` (use Mermaid.js or Excalidraw).
2.  **Benchmark Section:** Include a dedicated `BENCHMARK.md` or README section showing how many requests per second your single-node cache can handle, detailing CPU/RAM usage.
3.  **Blog Posts:** Write a series of technical blog posts describing your challenges and solutions.
    *   *Example 1:* "Designing an LRU Cache from scratch in Go: A Concurrency Deep-Dive"
    *   *Example 2:* "Distributed Routing: How I implemented Consistent Hashing"
4.  **Bullet Points for your CV:**
    *   *Designed and implemented a high-throughput, distributed, in-memory cache in Go handling X,XXX ops/sec with consistent hashing and dynamic peer-to-peer request routing.*
    *   *Engineered a thread-safe LRU eviction system with active TTL cleanup, reducing peak memory usage by XX%.*
    *   *Implemented replication and a Gossip-based node discovery mechanism to handle node churn and network partitioning.*
