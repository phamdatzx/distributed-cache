# Distributed Cache in Go 🚀

An in-development, thread-safe, in-memory distributed cache written in Go. This repository is built as a learning journey into distributed systems, concurrent programming, and robust engineering.

## 📁 Directory Structure

```
├── .gitignore                  # standard Go gitignore rules
├── go.mod                      # Go module definition
├── ROADMAP.md                  # Multi-phase long-term development roadmap
├── PHASE1.md                   # Phase 1 checklist (core cache engine)
├── PHASE2.md                   # Phase 2 checklist (network/server layer)
├── README.md                   # This document
├── cmd/
│   └── cache/
│       └── main.go             # server entrypoint (flags, env, signals)
└── internal/
    ├── cache/                  # thread-safe cache + TTL wrapper
    │   └── cache.go
    ├── eviction/               # storage layer (Policy, LRU, LFU stub)
    │   ├── eviction.go
    │   ├── lru.go
    │   └── lfu.go
    └── server/                 # TCP server + RESP-flavored protocol
        ├── protocol.go
        ├── commands.go
        └── server.go
```

## 🛠️ Getting Started

### Prerequisites
- [Go](https://go.dev/doc/install) (version 1.26 or newer)

### Run the Server
```bash
go run ./cmd/cache
```

This starts the cache server on `:6380`. Config flags, each with an environment-variable fallback:

| Flag | Env var | Default | Meaning |
|------|---------|---------|---------|
| `-addr` | `CACHE_ADDR` | `:6380` | TCP listen address |
| `-max-entries` | `CACHE_MAX_ENTRIES` | `1024` | LRU capacity |
| `-cleanup-interval` | `CACHE_CLEANUP_INTERVAL` | `1m` | TTL janitor interval (`0` disables) |

### Try it with netcat

```text
$ nc localhost 6380
PING
+PONG
SET greeting hello
+OK
GET greeting
$5
hello
GET missing
$-1
DEL greeting
:1
STATS
$39
commands:6
active_connections:1
errors:0
QUIT
+OK
```

`Ctrl-C` (SIGINT/SIGTERM) drains in-flight connections and exits cleanly.

---

## 🗺️ Roadmap & Progression

To see the detailed 8-phase execution plan for building this project from a single-node memory store to a production-grade, highly available clustered system, check out **[ROADMAP.md](./ROADMAP.md)**.
