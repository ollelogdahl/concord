package concord

import (
	"context"
	"crypto/tls"

	"github.com/ollelogdahl/concord/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type rpcHandler struct {
	rpc.UnimplementedChordServiceServer
	concord *Concord
}

func (r *rpcHandler) RegisterService(srv *grpc.Server) {
	rpc.RegisterChordServiceServer(srv, r)
}

func (r *rpcHandler) FindSuccessor(ctx context.Context, req *rpc.FindReq) (*rpc.FindResp, error) {
	s, err := r.concord.findSuccessor(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	resp := &rpc.FindResp{}
	resp.Server = convertServerToProto(&s)

	return resp, nil
}

func (r *rpcHandler) GetRing(ctx context.Context, _ *emptypb.Empty) (*rpc.Ring, error) {
	succ := r.concord.Successors()
	pred, ok := r.concord.Predecessor()

	protoSuccs := make([]*rpc.Server, len(succ))
	for i, s := range succ {
		protoSuccs[i] = convertServerToProto(&s)
	}

	resp := &rpc.Ring{
		Successors: protoSuccs,
	}

	if ok {
		resp.Predecessor = convertServerToProto(&pred)
	}

	return resp, nil
}

func (r *rpcHandler) Notify(ctx context.Context, srv *rpc.Server) (*emptypb.Empty, error) {
	r.concord.rectify(ctx, *convertProtoToServer(srv))

	return &emptypb.Empty{}, nil
}

type rpcClient interface {
	FindSuccessor(ctx context.Context, id uint64) (Server, error)
	GetRing(ctx context.Context) (ring, error)
	Notify(ctx context.Context, srv Server) error
}

type rpcClientGrpc struct {
	conn *grpc.ClientConn
	cli  rpc.ChordServiceClient
}

type rpcClientDispatch struct {
	hnd *rpcHandler
}

func (c *Concord) newClientGrpc(addr string) (rpcClient, error) {

	var creds credentials.TransportCredentials
	if c.capool == nil {
		creds = insecure.NewCredentials()
	} else {
		tlsConf := &tls.Config{
			RootCAs:      c.capool,
			ServerName:   addr,
			Certificates: []tls.Certificate{c.cert},
		}

		creds = credentials.NewTLS(tlsConf)
	}

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return &rpcClientGrpc{}, err
	}

	cli := rpc.NewChordServiceClient(conn)

	return &rpcClientGrpc{
		conn: conn,
		cli:  cli,
	}, nil
}

func newClientDispatch(hnd *rpcHandler) rpcClient {
	return &rpcClientDispatch{
		hnd: hnd,
	}
}

func (c *rpcClientGrpc) FindSuccessor(ctx context.Context, id uint64) (Server, error) {
	req := rpc.FindReq{Id: id}
	resp, err := c.cli.FindSuccessor(ctx, &req)
	if err != nil {
		return Server{}, err
	}

	return *convertProtoToServer(resp.Server), nil
}
func (c *rpcClientGrpc) GetRing(ctx context.Context) (ring, error) {
	resp, err := c.cli.GetRing(ctx, &emptypb.Empty{})
	if err != nil {
		return ring{}, err
	}

	r := ring{
		Predecessor: convertProtoToServer(resp.Predecessor),
	}

	r.Successors = make([]Server, len(resp.Successors))
	for i := range resp.Successors {
		r.Successors[i] = *convertProtoToServer(resp.Successors[i])
	}

	return r, nil
}
func (c *rpcClientGrpc) Notify(ctx context.Context, srv Server) error {
	req := convertServerToProto(&srv)

	_, err := c.cli.Notify(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (c *rpcClientDispatch) FindSuccessor(ctx context.Context, id uint64) (Server, error) {
	req := rpc.FindReq{Id: id}
	resp, err := c.hnd.FindSuccessor(ctx, &req)
	if err != nil {
		return Server{}, err
	}

	return *convertProtoToServer(resp.Server), nil
}
func (c *rpcClientDispatch) GetRing(ctx context.Context) (ring, error) {
	resp, err := c.hnd.GetRing(ctx, &emptypb.Empty{})
	if err != nil {
		return ring{}, err
	}

	r := ring{}
	if resp.Predecessor != nil {
		r.Predecessor = convertProtoToServer(resp.Predecessor)
	}

	r.Successors = make([]Server, len(resp.Successors))
	for i := range resp.Successors {
		r.Successors[i] = *convertProtoToServer(resp.Successors[i])
	}

	return r, nil
}
func (c *rpcClientDispatch) Notify(ctx context.Context, srv Server) error {
	req := convertServerToProto(&srv)

	_, err := c.hnd.Notify(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func convertServerToProto(server *Server) *rpc.Server {
	if server == nil {
		return nil
	}
	return &rpc.Server{
		Id:      server.Id,
		Name:    server.Name,
		Address: server.Address,
	}
}

func convertProtoToServer(server *rpc.Server) *Server {
	if server == nil {
		return nil
	}
	return &Server{
		Id:      server.Id,
		Name:    server.Name,
		Address: server.Address,
	}
}
