package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/ollelogdahl/concord"
)

type Sample struct {
	DurationMs float64 `json:"duration_ms"`
}

type BenchmarkRun struct {
	Name      string         `json:"name"`
	Timestamp time.Time      `json:"timestamp"`
	Params    map[string]int `json:"params"`
	Samples   []Sample       `json:"samples"`
}

var (
	results     []BenchmarkRun
	mu          sync.Mutex
	iterations  = 10
	outputFile  = "bench_results.json"
)

func recordSample(name string, params map[string]int, durationMs float64) {
	mu.Lock()
	defer mu.Unlock()
	var run *BenchmarkRun
	for i := range results {
		if results[i].Name == name {
			run = &results[i]
			break
		}
	}
	if run == nil {
		results = append(results, BenchmarkRun{
			Name:      name,
			Timestamp: time.Now(),
			Params:    params,
			Samples:   []Sample{},
		})
		run = &results[len(results)-1]
	}
	run.Samples = append(run.Samples, Sample{DurationMs: durationMs})
}

func saveResults() {
	mu.Lock()
	defer mu.Unlock()
	data, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(outputFile, data, 0644)
}

func createTestConfig(name, bindAddr, advAddr string) concord.Config {
	return concord.Config{
		Name:     name,
		BindAddr: bindAddr,
		AdvAddr:  advAddr,
		LogHandler: slog.NewTextHandler(io.Discard, nil),
	}
}

func setupCluster(n int) []*concord.Concord {
	nodes := make([]*concord.Concord, n)
	basePort := 50000
	for i := 0; i < n; i++ {
		addr := fmt.Sprintf("127.0.0.1:%d", basePort+i)
		cfg := createTestConfig(fmt.Sprintf("node-%d", i), addr, addr)
		nodes[i] = concord.New(cfg)
		if err := nodes[i].Start(); err != nil {
			log.Fatalf("start node %d: %v", i, err)
		}
	}
	if err := nodes[0].Create(); err != nil {
		log.Fatalf("create cluster: %v", err)
	}
	ctx := context.Background()
	for i := 1; i < n; i++ {
		if err := nodes[i].Join(ctx, nodes[0].Address()); err != nil {
			log.Fatalf("join node %d: %v", i, err)
		}
	}
	for {
		if isRingStable(nodes) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nodes
}

func isRingStable(nodes []*concord.Concord) bool {
	if len(nodes) == 0 {
		return true
	}
	for _, node := range nodes {
		_, ok := node.Predecessor()
		if len(node.Successors()) == 0 || !ok {
			return false
		}
	}
	visited := make(map[uint64]bool)
	current := nodes[0].Id()
	for i := 0; i < len(nodes)*2; i++ {
		if visited[current] {
			break
		}
		visited[current] = true
		var found bool
		for _, node := range nodes {
			if node.Id() == current {
				succ := node.Successors()
				if len(succ) > 0 {
					current = succ[0].Id
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return len(visited) == len(nodes)
}

func cleanup(nodes []*concord.Concord) {
	for _, node := range nodes {
		node.Stop()
	}
	time.Sleep(300 * time.Millisecond)
}

func reportLookup() {
	tests := []struct {
		nodes, keys int
	}{
		{1, 1}, {1, 100}, {1, 1000},
		{5, 1}, {5, 100}, {5, 1000},
		{10, 1}, {10, 100}, {10, 1000},
	}
	for _, t := range tests {
		size := t.nodes
		keyCount := t.keys

		nodes := setupCluster(size)
		name := fmt.Sprintf("Lookup_N=%d_K=%d", size, keyCount)

		params := map[string]int{"nodes": size, "keys": keyCount}

		keyList := make([][]byte, keyCount)
		for i := range keyList {
			keyList[i] = []byte(fmt.Sprintf("key-%d", i))
		}

		for i := 0; i < iterations; i++ {
			key := keyList[i%len(keyList)]
			start := time.Now()
			_, err := nodes[0].Lookup(key)
			if err != nil {
				continue
			}
			d := time.Since(start).Seconds() * 1000
			recordSample(name, params, d)
		}
		saveResults()
		cleanup(nodes)
	}
}

func reportJoin() {
	sizes := []int{5, 10}
	for _, size := range sizes {
		name := fmt.Sprintf("Join_N=%d", size)
		params := map[string]int{"nodes": size}
		for i := 0; i < iterations; i++ {
			nodes := setupCluster(size)
			baseAddr := nodes[0].Address()
			port := 60000 + (i * 100)
			addr := fmt.Sprintf("127.0.0.1:%d", port)
			cfg := createTestConfig(fmt.Sprintf("joining-%d-%d", size, i), addr, addr)
			joining := concord.New(cfg)
			if err := joining.Start(); err != nil {
				log.Fatalf("join start: %v", err)
			}
			start := time.Now()
			ctx := context.Background()
			if err := joining.Join(ctx, baseAddr); err != nil {
				log.Fatalf("join err: %v", err)
			}
			d := float64(time.Since(start).Milliseconds())
			recordSample(name, params, d)
			joining.Stop()
			cleanup(nodes)
		}
		saveResults()
	}
}

func reportConcurrentLookups() {
	clusterSize := 10
	concurrencyLevels := []int{5, 20}
	nodes := setupCluster(clusterSize)
	for _, c := range concurrencyLevels {
		name := fmt.Sprintf("ConcurrentLookup_C=%d", c)
		params := map[string]int{"concurrency": c, "nodes": clusterSize}
		var wg sync.WaitGroup
		opsPerRoutine := iterations
		for j := 0; j < c; j++ {
			wg.Add(1)
			go func(rid int) {
				defer wg.Done()
				node := nodes[rid%len(nodes)]
				for k := 0; k < opsPerRoutine; k++ {
					key := []byte(fmt.Sprintf("key-%d-%d", rid, k))
					start := time.Now()
					_, err := node.Lookup(key)
					if err != nil {
						continue
					}
					d := time.Since(start).Seconds() * 1000
					recordSample(name, params, d)
				}
			}(j)
		}
		wg.Wait()
		saveResults()
	}
	cleanup(nodes)
}

func main() {
	log.SetFlags(0)
	reportLookup()
	reportJoin()
	reportConcurrentLookups()
	fmt.Printf("Results saved to %s\n", outputFile)
}
