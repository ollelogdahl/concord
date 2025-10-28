package system_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/ollelogdahl/concord"
)

func hash(data []byte) uint64 {
    h := sha256.Sum256(data)
    return binary.BigEndian.Uint64(h[:8])
}

type ConcordSetup struct {
	startPort     atomic.Int32
	nodes         []*concord.Concord
}

func NewConcordSetup() *ConcordSetup {
	return &ConcordSetup{
		startPort:   atomic.Int32{},
		nodes:       make([]*concord.Concord, 0),
	}
}

// CreateClusterNodes creates n Concord nodes in a cluster
func (cs *ConcordSetup) CreateClusterNodes(t *testing.T, ctx context.Context, n int) ([]*concord.Concord, error) {
	nodes := make([]*concord.Concord, 0, n)
	for i := 0; i < n; i++ {
		node, err := cs.CreateNode(t, ctx)
		if err != nil {
			// Clean up on failure
			cs.StopNodes(ctx, nodes)
			return nil, err
		}
		err = node.Start()
		if err != nil {
			cs.StopNodes(ctx, nodes)
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

type testLogWriter struct {
	t *testing.T
}

func (w *testLogWriter) Write(p []byte) (n int, err error) {
	w.t.Logf("%s", p)
	return len(p), nil
}

// CreateNode creates a single Concord node
func (cs *ConcordSetup) CreateNode(t *testing.T, ctx context.Context) (*concord.Concord, error) {
	port := cs.startPort.Add(1)
	addr := fmt.Sprintf("localhost:%d", 15000+port)

	config := concord.Config{
		Name:    fmt.Sprintf("node-%d", port),
		BindAddr: addr,
		AdvAddr: addr,
		LogHandler: slog.NewTextHandler(&testLogWriter{t}, nil),
	}

	concord := concord.New(config)
	cs.nodes = append(cs.nodes, concord)

	return concord, nil
}

// StopNodes stops all nodes and closes their gRPC servers
func (cs *ConcordSetup) StopNodes(ctx context.Context, nodes []*concord.Concord) error {
	for _, node := range nodes {
		if err := node.Stop(); err != nil {
			fmt.Printf("error stopping node %s: %v\n", node.Name(), err)
		}
	}

	return nil
}

// GenerateRandomKeys generates count random keys of keySize bytes each
func (cs *ConcordSetup) GenerateRandomKeys(count int, keySize int) ([][]byte, error) {
	keys := make([][]byte, count)
	for i := 0; i < count; i++ {
		key := make([]byte, keySize)
		_, err := rand.Read(key)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random key: %w", err)
		}
		keys[i] = key
	}
	return keys, nil
}

// ConnectCluster connects all nodes into a cluster
func (cs *ConcordSetup) ConnectCluster(ctx context.Context, nodes []*concord.Concord) error {
	if len(nodes) == 0 {
		return fmt.Errorf("cannot create cluster with 0 nodes")
	}

	// Create the ring with the first node
	if err := nodes[0].Create(); err != nil {
		return fmt.Errorf("failed to create initial node: %w", err)
	}

	// Join all other nodes to the cluster
	for i := 1; i < len(nodes); i++ {
		if err := nodes[i].Join(ctx, nodes[0].Address()); err != nil {
			return fmt.Errorf("failed to join node %d: %w", i, err)
		}
	}

	for _, node := range nodes {
		node.Start()
	}

	return nil
}
