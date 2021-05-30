package api

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/term/proto"

	"github.com/gravitational/trace"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Config struct {
	TLS      *tls.Config
	Listener net.Listener
}

// New returns unstarted server
func New(cfg Config) (*Server, error) {
	// web handler serves classic HTTP 1.0 UI
	webHandler, err := newWebHandler()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	grpcHandler := &Handler{}

	// this is the server setup to serve over websocket transport
	webGRPCServer := grpc.NewServer()
	options := []grpcweb.Option{
		grpcweb.WithWebsockets(true),
	}

	proto.RegisterTickServiceServer(webGRPCServer, grpcHandler)
	wrappedGrpcServer := grpcweb.WrapServer(webGRPCServer, options...)

	// this is the server setup to serve over HTTP2/0 mTLS transport
	opts := []grpc.ServerOption{
		grpc.Creds(&httplib.TLSCreds{
			Config: cfg.TLS,
		}),
	}
	grpcServer := grpc.NewServer(opts...)
	proto.RegisterTickServiceServer(grpcServer, grpcHandler)

	s := &Server{
		cfg: cfg,
		log: log.WithFields(log.Fields{
			trace.Component: "grpc",
		}),
		httpHandler:   webHandler,
		webServer:     wrappedGrpcServer,
		webGRPCServer: webGRPCServer,
		grpcServer:    grpcServer,
	}
	s.httpServer = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: defaults.DefaultDialTimeout,
	}
	return s, nil
}

// Server is GPRC handler middleware
type Server struct {
	cfg        Config
	log        *log.Entry
	httpServer *http.Server
	// httpHandler is a server serving HTTP API
	httpHandler http.Handler
	// webServer is golang GRPC handler
	// that uses web sockets as a transport and
	// is used by the UI
	webServer *grpcweb.WrappedGrpcServer
	// webGRPCServer is a GPRC server setup for websockets
	webGRPCServer *grpc.Server
	// grpcServer is a GPRC server setup for classic HTTP2/0
	grpcServer *grpc.Server
}

// ServeHTTP dispatches requests based on the request type
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.webServer.IsGrpcWebSocketRequest(r) || s.webServer.IsGrpcWebRequest(r) {
		s.webServer.ServeHTTP(w, r)
	} else {
		s.httpHandler.ServeHTTP(w, r)
	}
}

func (s *Server) Serve() error {
	mux, err := multiplexer.NewTLSListener(multiplexer.TLSListenerConfig{
		Listener: tls.NewListener(s.cfg.Listener, s.cfg.TLS),
		ID:       "term",
	})
	if err != nil {
		return trace.Wrap(err)
	}

	errC := make(chan error, 2)
	go func() {
		err := mux.Serve()
		log.WithError(err).Warningf("Mux serve failed.")
	}()
	go func() {
		errC <- s.httpServer.Serve(mux.HTTP())
	}()
	go func() {
		errC <- s.grpcServer.Serve(mux.HTTP2())
	}()
	errors := []error{}
	for i := 0; i < 2; i++ {
		errors = append(errors, <-errC)
	}
	return trace.NewAggregate(errors...)
}

// Close closes TLS server non-gracefully - terminates in flight connections
func (s *Server) Close() error {
	errC := make(chan error, 2)
	go func() {
		errC <- s.httpServer.Close()
	}()
	go func() {
		s.grpcServer.Stop()
		errC <- nil
	}()
	errors := []error{}
	for i := 0; i < 2; i++ {
		errors = append(errors, <-errC)
	}
	return trace.NewAggregate(errors...)
}

// Shutdown shuts down TLS server
func (s *Server) Shutdown(ctx context.Context) error {
	errC := make(chan error, 2)
	go func() {
		errC <- s.httpServer.Shutdown(ctx)
	}()
	go func() {
		s.grpcServer.GracefulStop()
		errC <- nil
	}()
	errors := []error{}
	for i := 0; i < 2; i++ {
		errors = append(errors, <-errC)
	}
	return trace.NewAggregate(errors...)
}
