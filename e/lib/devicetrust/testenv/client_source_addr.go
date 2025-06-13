package testenv

import (
	"context"
	"net"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport/lib/authz"
)

const clientSourceAddrKey = "testenv.client-source-addr"

// WithOutgoingClientSourceAddr assigns addr as the client source address for
// RPCs using ctx.
func WithOutgoingClientSourceAddr(ctx context.Context, addr *net.TCPAddr) context.Context {
	return metadata.AppendToOutgoingContext(ctx, clientSourceAddrKey, addr.String())
}

func sourceAddrFromIncoming(ctx context.Context) (*net.TCPAddr, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, false
	}

	vals := md[clientSourceAddrKey]
	if len(vals) == 0 {
		return nil, false
	}

	addr := vals[0] // ip:port
	ip, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, false
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, false
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil {
		return nil, false
	}

	return &net.TCPAddr{
		IP:   parsedIP,
		Port: parsedPort,
	}, true
}

// clientSourceAddrUnaryInterceptor works in tandem with
// [WithOutgoingClientSourceAddr].
func clientSourceAddrUnaryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if sourceAddr, ok := sourceAddrFromIncoming(ctx); ok {
		ctx = authz.ContextWithClientSrcAddr(ctx, sourceAddr)
	}
	return handler(ctx, req)
}

// clientSourceAddrStreamInterceptor works in tandem with
// [WithOutgoingClientSourceAddr].
func clientSourceAddrStreamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	if sourceAddr, ok := sourceAddrFromIncoming(ctx); ok {
		ss = &ctxStream{
			ServerStream: ss,
			ctx:          authz.ContextWithClientSrcAddr(ctx, sourceAddr),
		}
	}

	return handler(srv, ss)
}

// ctxStream lets us override the context of a gprc.ServerStream.
type ctxStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *ctxStream) Context() context.Context {
	return s.ctx
}
