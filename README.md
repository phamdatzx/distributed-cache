# Distributed Cache in Go 🚀

An in-development high-performance, thread-safe, in-memory distributed cache written in Go. This repository is built as a learning journey into distributed systems, concurrent programming, and robust engineering.

## 📁 Directory Structure

```
├── .gitignore          # standard Go gitignore rules
├── go.mod              # Go module definition
├── ROADMAP.md          # Multi-phase long-term development roadmap
├── README.md           # This document
├── cmd/
│   └── cache/          # Application entry point
│       └── main.go     # main function
└── internal/
    └── cache/          # Private core cache logic (LRU/LFU, thread-safe maps, TTL)
        └── cache.go
```

## 🛠️ Getting Started

### Prerequisites
- [Go](https://go.dev/doc/install) (version 1.26 or newer)

### Run the Skeleton Server
To run the server's entrypoint, use:
```bash
go run cmd/cache/main.go
```

---

## 🗺️ Roadmap & Progression

To see the detailed 7-phase execution plan for building this project from a single-node memory store to a production-grade, highly available clustered system, check out **[ROADMAP.md](./ROADMAP.md)**.
# distributed-cache
