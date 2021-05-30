package main

import (
	"context"
	"net"

	"github.com/gravitational/teleport/lib/term/api"

	"github.com/gravitational/trace"
)

// runWebServer runs web server
func runWebServer(ctx context.Context, listenAddr, certPath, keyPath string) error {
	if err := api.InitSelfSignedHTTPSCert(certPath, keyPath); err != nil {
		return trace.Wrap(err)
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	tlsConfig, err := api.LoadTLSConfig(certPath, keyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	server, err := api.New(api.Config{
		Listener: listener,
		TLS:      tlsConfig,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	errC := make(chan error, 1)
	go func() {
		if err := server.Serve(); err != nil {
			errC <- err
		}
	}()

	select {
	case err := <-errC:
		log.WithError(err).Infof("Server exited.")
		return err
	case <-ctx.Done():
		log.Infof("Server shutting down on signal")
		return server.Close()
	}
}
