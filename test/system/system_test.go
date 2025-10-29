package system_test

import (
	"context"
	"testing"
	"time"

	"github.com/ollelogdahl/concord"
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
		AssertFullRangeCover(ct, nodes)
	}, 10*time.Second, 100*time.Millisecond)
}

func TestNodeShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	setup := NewConcordSetup()

	// Create 2 nodes
	nodes, err := setup.CreateClusterNodes(t, ctx, 3)
	require.NoError(t, err, "failed to create cluster nodes")
	defer setup.StopNodes(ctx, nodes)

	// Connect them into a ring
	err = setup.ConnectCluster(ctx, nodes)
	require.NoError(t, err, "failed to connect cluster")

	err = nodes[1].Stop()
	require.NoError(t, err, "failed to stope node 2")

	ns := []*concord.Concord{nodes[0], nodes[2]}

	// Assert ring consistency
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		AssertConsistentRing(ct, ns)
		AssertFullRangeCover(ct, ns)
	}, 10*time.Second, 100*time.Millisecond)
}

func Test2NodeShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	setup := NewConcordSetup()

	// Create 2 nodes
	nodes, err := setup.CreateClusterNodes(t, ctx, 3)
	require.NoError(t, err, "failed to create cluster nodes")
	defer setup.StopNodes(ctx, nodes)

	// Connect them into a ring
	err = setup.ConnectCluster(ctx, nodes)
	require.NoError(t, err, "failed to connect cluster")

	err = nodes[1].Stop()
	require.NoError(t, err, "failed to stope node 2")

	err = nodes[2].Stop()
	require.NoError(t, err, "failed to stope node 2")

	ns := []*concord.Concord{nodes[0]};

	// Assert ring consistency
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		AssertConsistentRing(ct, ns)
		AssertFullRangeCover(ct, ns)
	}, 10*time.Second, 100*time.Millisecond)
}
