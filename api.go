// concord is a chord core implementation in golang.
// It is fully resilient up to N (configurable) node failures.
//
// It includes callback for range changes so a DHT can be built on
// top of it.
package concord

import (
	"context"
	"log/slog"
	"net"

	"google.golang.org/grpc"
)

// The main configuration for the Concord service.
type Config struct {
	Name     string
	BindAddr string
	AdvAddr  string

	OnRangeChange func(Range) error

	HashFunc func([]byte) uint64
	HashBits uint

	LogHandler slog.Handler
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
	self        Server
	interval    Range
	successors  []Server
	predecessor *Server
	finger      []fingerEntry

	bindAddr string
	advAddr  string

	ln  net.Listener
	srv *grpc.Server
	rpc *rpcHandler

	clients map[string]rpcClient

	stabilizeCtx    context.Context
	stabilizeCancel context.CancelFunc

	hashFunc func([]byte) uint64
	hashBits uint

	logger *slog.Logger
}

// Creates a new instance of the Concord service.
func New(config Config) *Concord {
	return newConcord(config)
}

// Returns the name of the Concord service.
func (c *Concord) Name() string {
	return c.self.Name
}

// Returns the ID of the Concord service.
func (c *Concord) Id() uint64 {
	return c.self.Id
}

// Returns the address of the Concord service.
func (c *Concord) Address() string {
	return c.self.Address
}

// Starts the Concord service; listens for incoming connections.
func (c *Concord) Start() error {
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
	c.stabilizeCancel()
	c.srv.Stop()
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
func (c *Concord) Lookup(key []byte) (*Server, error) {
	return c.findSuccessor(context.Background(), c.hashFunc(key))
}

// Returns the list of successor servers.
func (c *Concord) Successors() []Server {
	return c.successors
}

// Returns the predecessor server.
func (c *Concord) Predecessor() *Server {
	return c.predecessor
}

// Returns the range of keys managed by this server.
func (c *Concord) Range() Range {
	return c.interval
}
