<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/concord-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="assets/concord-light.svg">
    <img alt="Concord Logo" width="300">
  </picture>
</p>
<p align="center">
  <strong>A resilient Chord implementation in Go</strong>
</p>
<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#usage">Usage</a> •
  <a href="#development">Development</a> •
  <a href="#license">License</a>
</p>
<p align="center">
  <a href="https://pkg.go.dev/github.com/ollelogdahl/concord"><img src="https://pkg.go.dev/badge/github.com/ollelogdahl/concord.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/ollelogdahl/concord"><img src="https://goreportcard.com/badge/github.com/ollelogdahl/concord" alt="Go Report Card"></a>
  <a href="https://github.com/ollelogdahl/concord/actions"><img src="https://github.com/ollelogdahl/concord/workflows/CI/badge.svg" alt="CI Status"></a>
  <a href="https://codecov.io/gh/ollelogdahl/concord"><img alt="Codecov" src="https://img.shields.io/codecov/c/gh/ollelogdahl/concord"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
</p>

# Overview

Concord is a resilient implementation of the core [Chord protocol](https://en.wikipedia.org/wiki/Chord_(peer-to-peer)) in Go.
The protocol enables distributed key lookup in a peer-to-peer network using consistent hashing, a technique
for evenly distributing keys across multiple nodes while minimizing reassignments when nodes join and leave.
Chord allows nodes in this dynamic network to efficiently determine which node is responsible for a given key.
While Chord is often conflated with its common use case, Distributed Hash Tables, this library
implements the more general lookup protocol, allowing you to build DHTs or other distributed
applications on top of it.

This implementation closely follows Pamela Zave's formally verified Chord specification, thereby
achieving resilience to node failures and maintaining consistency during crash faults.

# Features

* **Failure Resilient:** Built-in configurable resilience to node failures ($N$), ensuring the ring remains operational and consistent.
* **Consistent Hashing Core:** Provides the basic primitives for consistent hashing in a distributed
  environment.
* **Range Change Callbacks:** Includes a callback function (`OnRangeChange`) that notifies the
  application when a node becomes responsible for a new range of keys, essential for
  building a DHT.
* **Customizable Hashing:** Supports custom hash functions and configurable hash bit-widths (up to
  64-bit keys).
* **Structured Logging:** Uses Go's built-in `log/slog` for structured and customizable logging.
* **gRPC Based:** Uses **gRPC** for internal node-to-node communication.

# What is Chord?

Chord is a peer-to-peer protocol for distributed key lookup using consistent hashing. Consistent
hashing is a technique for evenly distributing keys across multiple nodes, minimizing the number
of keys that need to be moved when nodes join and leave the network. Chord allows nodes in this
dynamic network to efficiently determine which node is responsible for a given key.
While Chord is often conflated with its common use case, Distributed Hash Tables, this library
implements the more general lookup protocol, allowing you to build DHTs or other distributed
applications on top of it.

# Installation

```sh
go get github.com/ollelogdahl/concord
```

# Quick Start

## Creating a New Cluster

```go
package main

import (
    "log"
    "github.com/ollelogdahl/concord"
)

func main() {
    // Configure the node
    config := concord.Config{
        Name:     "node1",
        BindAddr: "0.0.0.0:7946",
        AdvAddr:  "node1.example.com:7946",
        OnRangeChange: func(r concord.Range) {
            log.Printf("Range changed: %d-%d", r.Start, r.End)
        },
    }

    // Create and start the node
    node := concord.New(config)
    if err := node.Start(); err != nil {
        log.Fatal(err)
    }
    defer node.Stop()

    // Create a new cluster
    if err := node.Create(); err != nil {
        log.Fatal(err)
    }

    log.Printf("Node %s (ID: %d) created cluster", node.Name(), node.Id())
}
```

## Joining an Existing Cluster

```go
package main

import (
    "context"
    "log"
    "github.com/ollelogdahl/concord"
)

func main() {
    config := concord.Config{
        Name:     "node2",
        BindAddr: "0.0.0.0:7947",
        AdvAddr:  "node2.example.com:7947",
    }

    node := concord.New(config)
    if err := node.Start(); err != nil {
        log.Fatal(err)
    }
    defer node.Stop()

    // Join an existing cluster
    ctx := context.Background()
    if err := node.Join(ctx, "node1.example.com:7946"); err != nil {
        log.Fatal(err)
    }

    log.Printf("Node %s joined cluster", node.Name())
}
```

# Usage

## Looking Up Keys

```go
// Lookup which node is responsible for a key
key := []byte("my-key")
server, err := node.Lookup(key)
if err != nil {
    log.Fatal(err)
}

log.Printf("Key is managed by node %s (ID: %d) at %s",
    server.Name, server.Id, server.Address)
```

## Monitoring range changes

```go
config := concord.Config{
    Name:     "node1",
    BindAddr: "0.0.0.0:7946",
    AdvAddr:  "node1.example.com:7946",
    OnRangeChange: func(r concord.Range) {
        log.Printf("Now responsible for range (%d, %d]", r.Start, r.End)

        // Migrate data, update local state, etc.
        migrateData(r)
    },
}
```

## Hash Function

By default, sha256 truncated to 64-bits is used as the hash function. Currently, the system
only allows for max 64-bit keys.

Custom hash functions can be used instead:

```go
import "hash/fnv"

func customHash(data []byte) uint64 {
    h := fnv.New32a()
    h.Write(data)
    return uint64(h.Sum32())
}

config := concord.Config{
    Name:     "node1",
    BindAddr: "0.0.0.0:7946",
    AdvAddr:  "node1.example.com:7946",
    HashFunc: customHash,
    HashBits:  32,
}
```

## Structured Logging

```go
import (
    "log/slog"
    "os"
)

config := concord.Config{
    Name:       "node1",
    BindAddr:   "0.0.0.0:7946",
    AdvAddr:    "node1.example.com:7946",
    LogHandler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }),
}
```

## Resilience

By default, the system supports up to 2 simultaneous failures of nodes. This is configurable
by setting `SuccessorCount` to $N+1$, where $N$ is the amount of simultaneous fails.

```go
config := concord.Config{
    Name:       "node1",
    BindAddr:   "0.0.0.0:7946",
    AdvAddr:    "node1.example.com:7946",
    SuccessorCount: 5,
    LogHandler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }),
}
```

# Development

## Prerequisites

- Go 1.24.0 or higher
- Protocol Buffers compiler and go generators (grpc, protobuf)

## Building

```sh
git clone https://github.com/ollelogdahl/concord.git
cd concord
go generate ./...
go build
```

## Running tests

```sh
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...
```

## Fuzzing

Concord includes fuzz tests for checking eventual consistent invariants.

```
go run -race ./test/fuzz/fuzz.go
```

## Test Nodes

Example nodes are provided in the `examples/` directory.

```sh
go run examples/node/node.go -name node1 -addr :7946
go run examples/node/node.go -name node2 -addr :7947 -join localhost:7946
go run examples/node/node.go -name node3 -addr :7948 -join localhost:7946
```

# Licence

This project is licenced under the [MIT License](github.com/ollelogdahl/concord/blob/master/LICENSE).

# Acknowledgements

- Original chord paper: [Chord: A Scalable Peer-to-Peer Lookup Service for Internet](https://pdos.csail.mit.edu/papers/chord:sigcomm01/chord_sigcomm.pdf)
- Formally proven Chord: [Reasoning about Identifier Spaces: How to Make Chord Correct](https://www.pamelazave.com/TSE_Chord_final.pdf)

<p align="center">Made with ☕ by Olle</p>
