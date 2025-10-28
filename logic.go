package concord

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"time"

	"google.golang.org/grpc"
)

const (
	StabilizeInterval = 3 * time.Second
)

func newConcord(config Config) *Concord {
	if config.HashFunc == nil {
		config.HashFunc = func(data []byte) uint64 {
			h := sha256.Sum256(data)
			return binary.BigEndian.Uint64(h[:8])
		}
		config.HashBits = 64
	}

	id := config.HashFunc([]byte(config.Name))

	cc := &Concord{}
	cc.self = Server{
		Name:    config.Name,
		Id:      id,
		Address: config.AdvAddr,
	}

	if config.SuccessorCount == 0 {
		config.SuccessorCount = 3
	}
	cc.successorCount = config.SuccessorCount

	cc.srv = grpc.NewServer()
	cc.rpc = &rpcHandler{concord: cc}

	cc.hashFunc = config.HashFunc
	cc.hashBits = config.HashBits

	cc.bindAddr = config.BindAddr
	cc.advAddr = config.AdvAddr

	cc.rangeChangeCallback = config.OnRangeChange

	if config.LogHandler == nil {
		config.LogHandler = slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
	}

	cc.logger = slog.New(config.LogHandler).With(
		"name", cc.self.Name,
		"self_id", cc.self.Id,
		"self_address", cc.self.Address,
	)

	cc.clients = make(map[string]rpcClient)
	cc.stabilizeCtx, cc.stabilizeCancel = context.WithCancel(context.Background())

	// initialize finger table
	cc.initFingerTable()

	return cc
}

func (c *Concord) initFingerTable() {
	m := uint64(c.hashBits)
	c.finger = make([]fingerEntry, m)
	for i := uint64(0); i < m; i++ {
		// implicit mod 2^64 (thanks to uint64)
		c.finger[i] = fingerEntry{
			Start: c.self.Id + (1 << i),
			Node:  nil,
		}
	}
}

func (c *Concord) fillFingerTable(n *Server) {
	m := uint64(c.hashBits)
	for i := uint64(0); i < m; i++ {
		c.finger[i].Node = n
	}
}

func (c *Concord) fixFinger(ctx context.Context, idx uint) error {
	node, err := c.findSuccessor(ctx, c.finger[idx].Start)
	if err != nil {
		return fmt.Errorf("failed fixing finger %d: %w", idx, err)
	}

	c.finger[idx].Node = &node
	return nil
}

func (c *Concord) create() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.logger.Info("creating new cluster")

	c.successors = make([]Server, c.successorCount)
	for i := range c.successors {
		c.successors[i] = c.self
	}

	c.predecessor = &c.self
	c.updateRange(Range{c.self.Id, c.self.Id})

	c.fillFingerTable(&c.self)

	go c.stabilizeTask(c.stabilizeCtx)
	return nil
}

func allButLast[T any](slice []T) []T {
	if len(slice) == 0 {
		return []T{}
	}
	return slice[:len(slice)-1]
}

func truncate[T any](slice []T, ln int) []T {
	if ln == 0 {
		return []T{}
	}
	if len(slice) < ln {
		ln = len(slice)
	}
	return slice[:ln]
}

func tail[T any](slice []T) []T {
	if len(slice) == 0 {
		return []T{}
	}
	return slice[1:]
}

func head[T any](slice []T) []T {
	if len(slice) == 0 {
		return []T{}
	}
	return slice[:1]
}

// @todo: bootstrap from multiple nodes must be possible.
func (c *Concord) join(ctx context.Context, bootstrap string) error {
	c.logger.Info("joining cluster", "bootstrap", bootstrap)

	const JOIN_BACKOFF = time.Second

	retryTicker := time.NewTicker(time.Second)

	for {
		select {
		case <-ctx.Done():
			retryTicker.Stop()
			return fmt.Errorf("join cancelled")
		case <-retryTicker.C:
			cli, err := c.client(bootstrap)
			if err != nil {
				c.logger.Error("failed to connect to bootstrap node", "error", err)
				continue
			}
			successor, err := cli.FindSuccessor(ctx, c.self.Id)
			if err != nil {
				c.logger.Error("failed find successor, retrying", "error", err)
				continue
			}
			c.logger.Info("found successor", "successor", successor.Name)

			// @note: micro optimization.
			var r ring
			if successor.Address != bootstrap {
				cli, err = c.client(successor.Address)
				if err != nil {
					c.logger.Error("failed to connect to successor, retrying", "error", err)
					continue
				}
			}
			r, err = cli.GetRing(ctx)
			if err != nil {
				c.logger.Error("failed to get ring from successor, retrying", "error", err)
				continue
			}

			c.lock.Lock()
			if r.Predecessor == nil {
				c.logger.Error("successor has no predecessor, retrying")
				c.lock.Unlock()
				continue
			}

			// insert ourselves into the ring;
			c.successors = append([]Server{successor}, allButLast(r.Successors)...)
			c.predecessor = r.Predecessor

			c.logger.Info("joined cluster", "successor", c.successors[0].Name, "predecessor", c.predecessor.Name)

			c.updateRange(Range{c.predecessor.Id, c.self.Id})

			c.fillFingerTable(&successor)

			go c.stabilizeTask(c.stabilizeCtx)

			c.lock.Unlock()
			return nil
		}
	}
}

func (c *Concord) findSuccessor(ctx context.Context, id uint64) (Server, error) {
	c.lock.RLock()
	c.logger.Info("finding successor", "id", id, "successors", c.successors)

	if between(c.self.Id, id, c.successors[0].Id) {
		defer c.lock.RUnlock()
		return c.successors[0], nil
	}

	n := c.closestPreceedingNode(id)
	if n == c.self {
		defer c.lock.RUnlock()
		return c.self, nil
	}

	// forward request to closest preceeding node first; if fails (due to churn), try successors.
	contenders := append([]Server{n}, c.successors...)
	var lastErr error
	c.lock.RUnlock()

	for _, contender := range contenders {
		cli, err := c.client(contender.Address)
		if err != nil {
			continue
		}

		c.logger.Info("forwarding findSuccessor", "to", contender.Name, "id", id)
		succ, err := cli.FindSuccessor(ctx, id)
		if err == nil {
			return succ, nil
		}
		lastErr = err
	}
	return Server{}, lastErr
}

func (c *Concord) closestPreceedingNode(id uint64) Server {
	for i := int(c.hashBits - 1); i >= 0; i-- {
		if c.finger[i].Node != nil && between(c.self.Id, c.finger[i].Node.Id, id) {
			return *c.finger[i].Node
		}
	}
	return c.self
}

func (c *Concord) rectify(ctx context.Context, srv Server) {
	c.lock.Lock()
	defer c.lock.Unlock()
	// c.logger.Debug("rectifying", "srv", srv)
	if c.predecessor == nil || between(c.predecessor.Id, srv.Id, c.self.Id) {
		c.predecessor = &srv
		c.updateRange(Range{srv.Id, c.self.Id})
	} else {
		cli, _ := c.client(c.predecessor.Address)

		// query liveness from predecessor
		c.lock.Unlock()
		_, err := cli.GetRing(ctx)
		c.lock.Lock()

		if err != nil {
			c.predecessor = &srv
			c.updateRange(Range{c.predecessor.Id, c.self.Id})
		}
	}
}

func (c *Concord) stabilizeFromSuccessor(ctx context.Context) {
	for {
		c.lock.RLock()
		cli, _ := c.client(c.successors[0].Address)
		c.lock.RUnlock()
		r, err := cli.GetRing(ctx)

		c.lock.Lock()
		if err == nil {
			if uint(len(c.successors)) < c.successorCount {
				c.successors = append(head(c.successors), r.Successors...)
			} else {
				c.successors = append(head(c.successors), truncate(r.Successors, int(c.successorCount-1))...)
			}

			// check if a new successor to us has been added.
			newSucc := r.Predecessor
			if newSucc != nil && between(c.self.Id, newSucc.Id, c.successors[0].Id) {
				c.lock.Unlock()
				c.stabilizeFromPredecessor(ctx, *newSucc)
			} else {
				c.lock.Unlock()
			}

			go c.notifySuccessor(ctx)

			break
		} else {
			if len(c.successors) == 1 {
				c.logger.Info("failed to reach all successors; complete isolation")
				c.successors = []Server{c.self}
				c.predecessor = &c.self
				c.lock.Unlock()
				go c.notifySuccessor(ctx)
				return
			} else {
				c.successors = tail(c.successors)
			}
			c.lock.Unlock()
		}

		go c.notifySuccessor(ctx)
	}
}

func (c *Concord) stabilizeFromPredecessor(ctx context.Context, newSucc Server) {
	pcli, err := c.client(newSucc.Address)
	if err != nil {
		return
	}

	r2, err := pcli.GetRing(ctx)
	if err == nil {
		c.lock.Lock()
		c.successors = append([]Server{newSucc}, allButLast(r2.Successors)...)
		c.lock.Unlock()
	}
}

func (c *Concord) notifySuccessor(ctx context.Context) error {
	c.lock.RLock()
	cli, err := c.client(c.successors[0].Address)
	c.lock.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	err = cli.Notify(ctx, c.self)
	if err != nil {
		return fmt.Errorf("failed to notify: %w", err)
	}
	return nil
}

func (c *Concord) stabilizeTask(ctx context.Context) {
	ticker := time.NewTicker(StabilizeInterval)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			c.stabilizeFromSuccessor(ctx)

			fingerToFix := rand.UintN(c.hashBits)
			if err := c.fixFinger(ctx, fingerToFix); err != nil {
				c.logger.Warn(err.Error())
			}

			c.lock.RLock()
			c.logger.Debug("stabilized", "successor", c.successors[0].Name, "predecessor", c.predecessor.Name)
			c.lock.RUnlock()
		}
	}
}

func (c *Concord) updateRange(r Range) {
	c.interval = r
	if c.rangeChangeCallback != nil {
		c.rangeChangeCallback(r)
	}
}

// returns true if a < b < c where a ring is respected.
func between(a, b, c uint64) bool {
	if a < c {
		return a < b && b < c
	} else {
		return a < b || b < c
	}
}

func (c *Concord) client(addr string) (rpcClient, error) {
	c.clientsLock.Lock()
	defer c.clientsLock.Unlock()
	cli, ok := c.clients[addr]
	if !ok {

		if addr == c.advAddr {
			c.clients[addr] = newClientDispatch(c.rpc)
			return c.clients[addr], nil
		} else {
			cli, err := newClientGrpc(addr)
			if err != nil {
				return cli, err
			}
			c.clients[addr] = cli
			return cli, nil
		}
	}

	return cli, nil
}
