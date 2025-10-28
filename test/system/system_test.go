package system_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		AssertConsistentRing(ct, nodes)
		AssertConsistentLookupForKey(ct, ctx, nodes, key)
	}, 10 * time.Second, 100 * time.Millisecond)
}
