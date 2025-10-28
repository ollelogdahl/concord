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
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
</p>

# Overview

Concord is a resilient implementation of the core Chord protocol in Go. It provides the core
foundation for consistent hashing that's fully resilient to node failures, making it ideal
for building scalable distributed systems.

The design of this library is based upon the formally proven work by Pamela Zave.

# What is Chord?

# Features

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
        OnRangeChange: func(r concord.Range) error {
            log.Printf("Range changed: %d-%d", r.Start, r.End)
            return nil
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
    OnRangeChange: func(r concord.Range) error {
        log.Printf("Now responsible for range (%d, %d]", r.Start, r.End)

        // Migrate data, update local state, etc.
        return migrateData(r)
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

# Development

## Prerequisites

- Go 1.25.1 or higher
- Protocol Buffers compiler and go generators (grpc, protobuf)

## Building

```sh
git clone https://github.com/yourusername/concord.git
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
go run ./test/fuzz/fuzz.go
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
