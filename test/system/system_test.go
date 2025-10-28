package system_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBasicClusterFormation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	setup := NewConcordSetup()

	// Create 2 nodes
	nodes, err := setup.CreateClusterNodes(t, ctx, 2)
	require.NoError(t, err, "failed to create cluster nodes")
	defer setup.StopNodes(ctx, nodes)

	// Connect them into a ring
	err = setup.ConnectCluster(ctx, nodes)
	require.NoError(t, err, "failed to connect cluster")

	// Allow time for stabilization
	time.Sleep(1 * time.Second)

	key := []byte("test")

	// Assert ring consistency
	AssertConsistentRing(t, nodes)

	// Assert consistent lookups
	AssertConsistentLookupForKey(t, ctx, nodes, key)
}
