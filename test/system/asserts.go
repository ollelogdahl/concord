package system_test

import (
	"context"
	"sort"

	"github.com/ollelogdahl/concord"
	"github.com/stretchr/testify/assert"
)

// AssertConsistentRing verifies that all nodes form a consistent ring
// where each node's successor has that node as predecessor
func AssertConsistentRing(t assert.TestingT, nodes []*concord.Concord) {
	assert.NotEmpty(t, nodes, "node list must not be empty")

	expectedSize := len(nodes)
	idToNode := make(map[uint64]*concord.Concord)

	for _, node := range nodes {
		idToNode[node.Id()] = node
	}

	visitedIds := make(map[uint64]bool)
	current := nodes[0]

	for i := 0; i <= expectedSize; i++ {
		currentID := current.Id()

		if visitedIds[currentID] {
			if len(visitedIds) == expectedSize {
				return // Ring is consistent
			}
			t.Errorf("Ring closed early after visiting %d nodes (expected %d)", len(visitedIds), expectedSize)
		}
		visitedIds[currentID] = true

		successors := current.Successors()
		assert.NotEmpty(t, successors, "Node %d has no successors", currentID)

		successor := successors[0]
		next, ok := idToNode[successor.Id]
		assert.True(t, ok, "Successor %d is not a known node (from node %d)", successor.Id, currentID)

		// Check bidirectional consistency
		pred := next.Predecessor()
		assert.NotNil(t, pred, "Node %d has null predecessor (successor of node %d)", successor.Id, currentID)
		assert.Equal(t, currentID, pred.Id, "Inconsistent links: Node %d → successor %d, but successor's predecessor is %d",
			currentID, successor.Id, pred.Id)

		current = next
	}

	t.Errorf("Walked %d steps without closing the loop (visited %d unique nodes)", expectedSize+1, len(visitedIds))
}

// AssertConsistentLookupForKey verifies that all nodes return the same owner for a given key
func AssertConsistentLookupForKey(t assert.TestingT, ctx context.Context, nodes []*concord.Concord, key []byte) {
	assert.NotEmpty(t, nodes, "node list must not be empty")

	expectedResult, err := nodes[0].Lookup(key)
	assert.NoError(t, err, "Lookup failed on starting node")
	assert.NotNil(t, expectedResult, "Lookup returned nil on starting node")

	for i := 1; i < len(nodes); i++ {
		node := nodes[i]
		actualResult, err := node.Lookup(key)
		assert.NoError(t, err, "Lookup failed on node %d", i)
		assert.NotNil(t, actualResult, "Node %d returned nil", i)
		assert.Equal(t, expectedResult.Id, actualResult.Id,
			"Node %d returned different server: expected %d, got %d", i, expectedResult.Id, actualResult.Id)
	}
}

// AssertFullRangeCover verifies that all nodes' ranges cover the entire ring without gaps
func AssertFullRangeCover(t assert.TestingT, nodes []*concord.Concord) {

	assert.NotEmpty(t, nodes, "node list must not be empty")

	type rangeData struct {
		start uint64
		end   uint64
		idx   int
	}

	var ranges []rangeData
	for i, node := range nodes {
		r := node.Range()
		ranges = append(ranges, rangeData{start: r.Start, end: r.End, idx: i})
	}

	// Sort ranges by start
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].start < ranges[j].start
	})

	// Check continuity
	for i := 0; i < len(ranges); i++ {
		current := ranges[i]
		next := ranges[(i+1)%len(ranges)]

		assert.Equal(t, current.end, next.start,
			"Range at index %d (%d, %d] must connect to next range's start (%d)",
			current.idx, current.start, current.end, next.start)
	}
}
