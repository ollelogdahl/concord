// concord is a chord core implementation in golang.
// It is fully resilient up to N (configurable) node failures.
//
// It includes callback for range changes so a DHT can be built on
// top of it.
package concord

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"google.golang.org/grpc"
)

type TLSConfig struct {
	ServerTLS tls.Config
	ClientTLS tls.Config
}

// The main configuration for the Concord service.
type Config struct {
	Name     string
	BindAddr string
	AdvAddr  string

	OnRangeChange func(Range)

	HashFunc func([]byte) uint64
	HashBits uint

	SuccessorCount uint
	LogHandler     slog.Handler

	TLS *TLSConfig
}

type Range struct {
	Start uint64
	End   uint64
}

type Server struct {
	Name    string
	Id      uint64
	Address string
}

type ring struct {
	Successors  []Server
	Predecessor *Server
}

type fingerEntry struct {
	Start uint64
	Node  *Server
}

// A handle to an instance of the Concord service.
type Concord struct {
	self           Server
	interval       Range
	successors     []Server
	predecessor    *Server
	finger         []fingerEntry
	successorCount uint

	bindAddr string
	advAddr  string

	lock    sync.RWMutex
	ln      net.Listener
	srv     *grpc.Server
	rpc     *rpcHandler
	started bool
	setup   bool

	clients connectionCache

	stabilizeCtx    context.Context
	stabilizeCancel context.CancelFunc

	hashFunc func([]byte) uint64
	hashBits uint

	rangeChangeCallback func(Range)

	logger *slog.Logger

	clientTLS *tls.Config
}

// Creates a new instance of the Concord service.
func New(config Config) *Concord {
	return newConcord(config)
}

// Returns the name of the Concord service.
func (c *Concord) Name() string {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.self.Name
}

// Returns the ID of the Concord service.
func (c *Concord) Id() uint64 {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.self.Id
}

// Returns the address of the Concord service.
func (c *Concord) Address() string {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.self.Address
}

// Starts the Concord service; listens for incoming connections.
func (c *Concord) Start() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.started {
		return fmt.Errorf("service already started")
	}
	c.started = true

	c.logger.Info("starting server", "bind", c.bindAddr, "address", c.self.Address)
	c.rpc.RegisterService(c.srv)

	ln, err := net.Listen("tcp", c.bindAddr)
	if err != nil {
		return err
	}
	c.ln = ln

	go func() {
		if err := c.srv.Serve(ln); err != nil {
			c.logger.Error("failed to serve", "error", err)
		}
	}()

	return nil
}

// Stops the Concord service.
func (c *Concord) Stop() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.setup {
		c.setup = false
		c.stabilizeCancel()
	}
	if c.started {
		c.srv.Stop()
		c.started = false
	}
	return nil
}

// Creates a new cluster. The Concord instance must be started before calling this method.
func (c *Concord) Create() error {
	return c.create()
}

// Joins an existing cluster. The Concord instance must be started before calling this method.
func (c *Concord) Join(ctx context.Context, bootstrapAddress string) error {
	return c.join(ctx, bootstrapAddress)
}

// Looks up the server responsible for the given key.
func (c *Concord) Lookup(key []byte) (Server, error) {
	return c.findSuccessor(context.Background(), c.hashFunc(key))
}

// Returns the list of successor servers.
func (c *Concord) Successors() []Server {
	c.lock.RLock()
	defer c.lock.RUnlock()

	// do a deep copy to ensure that the values are not modified.
	copiedSuccessors := make([]Server, len(c.successors))
	copy(copiedSuccessors, c.successors)

	return copiedSuccessors
}

// Returns the predecessor server.
func (c *Concord) Predecessor() (Server, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.predecessor == nil {
		return Server{}, false
	}
	return *c.predecessor, true
}

// Returns the range of keys managed by this server.
func (c *Concord) Range() Range {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.interval
}
