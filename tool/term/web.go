package main

import (
	"context"
	"net"
	"net/http"

	"github.com/gravitational/teleport/lib/term/api"

	"github.com/gravitational/trace"
)

// runWebServer runs web server
func runWebServer(ctx context.Context, listenAddr, certPath, keyPath string) error {
	if err := api.InitSelfSignedHTTPSCert(certPath, keyPath); err != nil {
		return trace.Wrap(err)
	}
	handler, err := api.New()
	if err != nil {
		return trace.Wrap(err)
	}

	server := &http.Server{
		Addr:    listenAddr,
		Handler: handler,
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	go server.ServeTLS(listener, certPath, keyPath)

	select {
	case <-ctx.Done():
		log.Infof("Server shutting down on signal")
		return server.Close()
	}
}
