package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"
	"flag"

	"github.com/ollelogdahl/concord"
	"github.com/ollelogdahl/concord/test/fuzz/fz"
	"github.com/stretchr/testify/assert"
)

type State struct {
	Nodes map[string]*concord.Concord
	Addrs map[string]string

	portIncrementor uint
}

type spawnP struct {
	Name string
	ToJoin string
}

type killP struct {
	Names []string
}

func generateCombinations(n, k int) [][]int {
	var result [][]int
	var current []int

	var backtrack func(start int)
	backtrack = func(start int) {
		if len(current) == k {
			combo := make([]int, k)
			copy(combo, current)
			result = append(result, combo)
			return
		}

		for i := start; i < n; i++ {
			current = append(current, i)
			backtrack(i + 1)
			current = current[:len(current)-1]
		}
	}

	backtrack(0)
	return result
}

func genSpawn(s *State) []spawnP {
	var tasks []spawnP

	if len(s.Nodes) == MAX_SIMULATED_NODES {
		return tasks
	}

	for name, _ := range s.Nodes {
		tasks = append(tasks, spawnP{
			Name: fmt.Sprintf("cord%d", s.portIncrementor),
			ToJoin: name,
		})
	}

	return tasks
}

func genKill(s *State) []killP {
	var tasks []killP

	// must always be at least 1 node left
	if len(s.Nodes) == 1 {
		return tasks
	}

	nodeNames := make([]string, 0, len(s.Nodes))
	for name := range s.Nodes {
		nodeNames = append(nodeNames, name)
	}

	maxToKill := min(MAX_SIMULTANEOUS_KILLS, len(nodeNames)-1)
	for size := 1; size <= maxToKill; size++ {
		combinations := generateCombinations(len(nodeNames), size)
		for _, combo := range combinations {
			names := make([]string, len(combo))
			for i, idx := range combo {
				names[i] = nodeNames[idx]
			}
			tasks = append(tasks, killP{Names: names})
		}
	}
	return tasks
}

func (s *State) nextAddrs() (string, string) {
	baddr := fmt.Sprintf(":%d", 10000 + s.portIncrementor)
	addr := fmt.Sprintf("localhost%s", baddr)

	return baddr, addr
}

func (s *State) add(name string, addr string, instance *concord.Concord) {
	s.Nodes[name] = instance
	s.Addrs[name] = addr
	s.portIncrementor++
}

func doSpawn(s *State, p spawnP) {
	name := p.Name
	baddr, addr := s.nextAddrs()

	cfg := concord.Config{
		Name: name,
		BindAddr: baddr,
		AdvAddr: addr,
	}

	instance := concord.New(cfg)
	_ = instance.Start()

	_ = instance.Join(context.Background(), s.Addrs[p.ToJoin])

	s.add(name, addr, instance)
}

func doKill(s *State, p killP) {
	for _, name := range p.Names {
		s.Nodes[name].Stop()

		delete(s.Nodes, name)
		delete(s.Addrs, name)
	}
}

func invEvConsistentRingAndCoverage(t assert.TestingT, s *State) {
	assert.EventuallyWithT(t, func(ct *assert.CollectT){
		// ensure that:
		// - \forall n \in s.Nodes n.Successor \in s.Nodes
		// - following successors forms a correct ring
		// - \forall n \in s.Nodes nodes[n.Successor].Predecessor = n
		if len(s.Nodes) == 0 {
			assert.Fail(ct, "No nodes in state")
			return
		}

		for name, node := range s.Nodes {
			// Check 1: Successor exists in cluster
			successors := node.Successors()
			if len(successors) == 0 {
				assert.Fail(ct, "Node %s has no successors", name)
				return
			}

			if _, exists := s.Nodes[successors[0].Name]; !exists {
				assert.Fail(ct, "Node %s has successor %s not in cluster", name, successors[0].Name)
				return
			}

			// Check 2: Predecessor exists and is symmetric
			pred := node.Predecessor()
			if pred == nil {
				assert.Fail(ct, "Node %s has no predecessor", name)
				return
			}

			predNode, exists := s.Nodes[pred.Name]
			if !exists {
				assert.Fail(ct, "Node %s has predecessor %s not in cluster", name, pred.Name)
				return
			}

			predSuccessors := predNode.Successors()
			if len(predSuccessors) == 0 || predSuccessors[0].Name != name {
				assert.Fail(ct, "Predecessor link broken: %s -> %s -/-> %s", name, pred.Name, name)
				return
			}
		}

		// Check 3: Ring traversal visits all nodes
		var startNode *concord.Concord
		for _, node := range s.Nodes {
			startNode = node
			break
		}

		visited := make(map[string]bool)
		current := startNode

		for len(visited) < len(s.Nodes) {
			name := current.Name()
			if visited[name] {
				assert.Fail(ct, "inconsistent", "Ring has cycle before visiting all nodes. expected %+v, visited %+v", s.Nodes, visited)
				return
			}
			visited[name] = true

			nextName := current.Successors()[0].Name
			current = s.Nodes[nextName]
		}

	}, 60 * time.Second, 100 * time.Millisecond)
}

const MAX_SIMULATED_NODES = 10
const MAX_SIMULTANEOUS_KILLS = 4

func main() {

	maxTime := flag.Duration("fuzztime", 1<<63 - 1, "duration to run the fuzzer")

	flag.Parse()

	rng := rand.New(rand.NewPCG(0, 1))
	fuzz := fz.NewFuzzer[State](rng, *maxTime)

	fz.AddAction(fuzz, "spawn", genSpawn, doSpawn)
	fz.AddAction(fuzz, "kill", genKill, doKill)

	// checks that eventually, the ring view is consistent and the entire
	// hash range is exclusively owned by one node.
	fuzz.AddInvariant("eventual-consistent-ring-and-coverage", invEvConsistentRingAndCoverage)

	// initial state
	initialState := State{
		Nodes: make(map[string]*concord.Concord),
		Addrs: make(map[string]string),
		portIncrementor: 0,
	}

	// add the initial node
	{
		name := "cord0"
		baddr, addr := initialState.nextAddrs()

		cfg := concord.Config{
			Name: name,
			BindAddr: baddr,
			AdvAddr: addr,
		}

		instance := concord.New(cfg)
		_ = instance.Start()
		_ = instance.Create()

		initialState.add(name, addr, instance)
	}

	fuzz.Run(&initialState)
}
