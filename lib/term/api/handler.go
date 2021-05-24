package api

import (
	"net/http"

	"github.com/gravitational/teleport/lib/term/proto"
	"github.com/improbable-eng/grpc-web/go/grpcweb"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// NewGRPCHandler returns a new instance of GRPC handler
func NewGRPCHandler(handler http.Handler) http.Handler {
	grpcServer := grpc.NewServer()
	options := []grpcweb.Option{
		grpcweb.WithWebsockets(true),
	}

	proto.RegisterTickServiceServer(grpcServer, &Handler{})
	wrappedGrpcServer := grpcweb.WrapServer(grpcServer, options...)

	return &GRPCHandler{
		log: log.WithFields(log.Fields{
			trace.Component: "grpc",
		}),
		httpHandler:    handler,
		webGRPCHandler: wrappedGrpcServer,
		grpcHandler:    grpcServer,
	}
}

// GRPCHandler is GPRC handler middleware
type GRPCHandler struct {
	log *log.Entry
	// httpHandler is a server serving HTTP API
	httpHandler http.Handler
	// webGRPCHandler is golang GRPC handler
	// that uses web sockets as a transport and
	// is used by the UI
	webGRPCHandler *grpcweb.WrappedGrpcServer
	// grpcHandler is a GPRC standard handler
	grpcHandler *grpc.Server
}

// ServeHTTP dispatches requests based on the request type
func (g *GRPCHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.webGRPCHandler.IsGrpcWebSocketRequest(r) || g.webGRPCHandler.IsGrpcWebRequest(r) {
		g.webGRPCHandler.ServeHTTP(w, r)
	} else {
		g.httpHandler.ServeHTTP(w, r)
	}
}
