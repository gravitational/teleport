/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package tracing

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/gravitational/trace"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	otlp "go.opentelemetry.io/proto/otlp/trace/v1"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/defaults"
)

// Collector is a simple in memory implementation of an OpenTelemetry Collector
// that is used for testing purposes.
type Collector struct {
	grpcLn     net.Listener
	httpLn     net.Listener
	grpcServer *grpc.Server
	httpServer *http.Server
	coltracepb.TraceServiceServer
	tlsConfing *tls.Config

	spanLock      sync.RWMutex
	spans         []*otlp.ScopeSpans
	resourceSpans []*otlp.ResourceSpans
	exportedC     chan struct{}
}

// CollectorConfig configures how a Collector should be created.
type CollectorConfig struct {
	TLSConfig *tls.Config
}

// NewCollector creates a new Collector based on the provided config.
func NewCollector(cfg CollectorConfig) (*Collector, error) {
	grpcLn, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httpLn, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var tlsConfig *tls.Config
	creds := insecure.NewCredentials()
	if cfg.TLSConfig != nil {
		tlsConfig = cfg.TLSConfig.Clone()
		creds = credentials.NewTLS(tlsConfig)
	}

	c := &Collector{
		grpcLn:     grpcLn,
		httpLn:     httpLn,
		grpcServer: grpc.NewServer(grpc.Creds(creds), grpc.MaxConcurrentStreams(defaults.GRPCMaxConcurrentStreams)),
		tlsConfing: tlsConfig,
		exportedC:  make(chan struct{}, 1),
	}

	c.httpServer = &http.Server{
		Handler:           c,
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		WriteTimeout:      apidefaults.DefaultIOTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
		TLSConfig:         tlsConfig.Clone(),
	}

	coltracepb.RegisterTraceServiceServer(c.grpcServer, c)

	return c, nil
}

// ClientTLSConfig returns the tls.Config that clients should
// use to connect to the collector.
func (c *Collector) ClientTLSConfig() *tls.Config {
	if c.tlsConfing == nil {
		return nil
	}

	return c.tlsConfing.Clone()
}

// GRPCAddr is the ExporterURL that the gRPC server is listening on.
func (c *Collector) GRPCAddr() string {
	return "grpc://" + c.grpcLn.Addr().String()
}

// HTTPAddr is the ExporterURL that the HTTP server is listening on.
func (c *Collector) HTTPAddr() string {
	return "http://" + c.httpLn.Addr().String()
}

// HTTPSAddr is the ExporterURL that the HTTPS server is listening on.
func (c *Collector) HTTPSAddr() string {
	return "https://" + c.httpLn.Addr().String()
}

// Start starts the gRPC and HTTP servers.
func (c *Collector) Start() error {
	var g errgroup.Group

	g.Go(func() error {
		var err error
		if c.httpServer.TLSConfig != nil {
			err = c.httpServer.ServeTLS(c.httpLn, "", "")
		} else {
			err = c.httpServer.Serve(c.httpLn)
		}

		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return trace.Wrap(err)
	})

	g.Go(func() error {
		err := c.grpcServer.Serve(c.grpcLn)
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return trace.Wrap(err)
	})

	return g.Wait()
}

// Shutdown stops the gRPC and HTTP servers.
func (c *Collector) Shutdown(ctx context.Context) error {
	c.grpcServer.Stop()
	return trace.Wrap(c.httpServer.Shutdown(ctx))
}

// ServeHTTP is the HTTP handler for the Collector.
func (c *Collector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/v1/traces" {
		rawRequest, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		req := &coltracepb.ExportTraceServiceRequest{}
		if err := proto.Unmarshal(rawRequest, req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resp, err := c.Export(r.Context(), req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		rawResponse, err := proto.Marshal(resp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(rawResponse)
	}
}

// Export is the grpc handler for the Collector.
func (c *Collector) Export(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	c.spanLock.Lock()
	defer c.spanLock.Unlock()

	c.resourceSpans = append(c.resourceSpans, req.ResourceSpans...)
	for _, span := range req.ResourceSpans {
		c.spans = append(c.spans, span.ScopeSpans...)
	}

	select {
	case c.exportedC <- struct{}{}:
	default:
	}

	return &coltracepb.ExportTraceServiceResponse{}, nil
}

func (c *Collector) WaitForExport() {
	<-c.exportedC
}

// Spans returns all collected spans and resets the collector
func (c *Collector) Spans() []*otlp.ScopeSpans {
	c.spanLock.Lock()
	defer c.spanLock.Unlock()
	spans := make([]*otlp.ScopeSpans, len(c.spans))
	copy(spans, c.spans)

	c.spans = nil
	c.resourceSpans = nil

	return spans
}

// ResourceSpans returns all collected resource spans and resets the collector.
func (c *Collector) ResourceSpans() []*otlp.ResourceSpans {
	c.spanLock.Lock()
	defer c.spanLock.Unlock()
	resourceSpans := make([]*otlp.ResourceSpans, len(c.resourceSpans))
	copy(resourceSpans, c.resourceSpans)

	c.spans = nil
	c.resourceSpans = nil

	return resourceSpans
}
