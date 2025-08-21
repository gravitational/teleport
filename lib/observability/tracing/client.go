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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otlpcommon "go.opentelemetry.io/proto/otlp/common/v1"
	otlp "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/gravitational/teleport/lib/observability/tracing/internal/tracetransform"
)

type noopClient struct{}

func (noopClient) Start(context.Context) error                               { return nil }
func (noopClient) Stop(context.Context) error                                { return nil }
func (noopClient) UploadTraces(context.Context, []*otlp.ResourceSpans) error { return nil }

// NewNoopClient returns an oltptrace.Client that does nothing
func NewNoopClient() otlptrace.Client {
	return noopClient{}
}

// NewStartedClient either returns the provided Config.Client or constructs
// a new client that is connected to the Config.ExporterURL with the
// appropriate TLS credentials. The client is started prior to returning to
// the caller.
func NewStartedClient(ctx context.Context, cfg Config) (otlptrace.Client, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()
	if err := clt.Start(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return clt, nil
}

// NewClient either returns the provided Config.Client or constructs
// a new client that is connected to the Config.ExporterURL with the
// appropriate TLS credentials. The returned client is not started,
// it will be started by the provider if passed to one.
func NewClient(cfg Config) (otlptrace.Client, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Client != nil {
		return cfg.Client, nil
	}

	var httpOptions []otlptracehttp.Option
	grpcOptions := []otlptracegrpc.Option{otlptracegrpc.WithDialOption(grpc.WithBlock())}

	if cfg.TLSConfig != nil {
		httpOptions = append(httpOptions, otlptracehttp.WithTLSClientConfig(cfg.TLSConfig.Clone()))
		grpcOptions = append(grpcOptions, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(cfg.TLSConfig.Clone())))
	} else {
		httpOptions = append(httpOptions, otlptracehttp.WithInsecure())
		grpcOptions = append(grpcOptions, otlptracegrpc.WithInsecure())
	}

	var traceClient otlptrace.Client
	switch cfg.exporterURL.Scheme {
	case "http", "https":
		httpOptions = append(httpOptions, otlptracehttp.WithEndpoint(cfg.Endpoint()))
		traceClient = otlptracehttp.NewClient(httpOptions...)
	case "grpc":
		grpcOptions = append(grpcOptions, otlptracegrpc.WithEndpoint(cfg.Endpoint()))
		traceClient = otlptracegrpc.NewClient(grpcOptions...)
	case "file":
		limit := DefaultFileLimit
		rawLimit := cfg.exporterURL.Query().Get("limit")
		if rawLimit != "" {
			convertedLimit, err := strconv.ParseUint(rawLimit, 10, 0)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			limit = convertedLimit
		}

		client, err := NewRotatingFileClient(cfg.Endpoint(), limit)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		traceClient = client
	default:
		return nil, trace.BadParameter("unsupported exporter scheme: %q", cfg.exporterURL.Scheme)
	}

	return traceClient, nil
}

var _ otlptrace.Client = (*RotatingFileClient)(nil)

// RotatingFileClient is an otlptrace.Client that writes traces to a file. It
// will automatically rotate files when they reach the configured limit. Each
// line in the file is a JSON-encoded otlp.Span.
type RotatingFileClient struct {
	dir     string
	limit   uint64
	written uint64

	lock sync.Mutex
	file *os.File
}

func fileName() string {
	return fmt.Sprintf("%d-*.trace", time.Now().UTC().UnixNano())
}

// DefaultFileLimit is the default file size limit used before
// rotating to a new traces file
const DefaultFileLimit uint64 = 1048576 * 100 // 100MB

// NewRotatingFileClient returns a new RotatingFileClient that will store files in the
// provided directory. The files will be rotated when they reach the provided limit.
func NewRotatingFileClient(dir string, limit uint64) (*RotatingFileClient, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, trace.ConvertSystemError(err)
	}

	f, err := os.CreateTemp(dir, fileName())
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	return &RotatingFileClient{
		dir:   dir,
		limit: limit,
		file:  f,
	}, nil
}

// Start is a noop needed to satisfy the otlptrace.Client interface.
func (f *RotatingFileClient) Start(ctx context.Context) error {
	return nil
}

// Stop closes the active file and sets it to nil to indicate to UploadTraces
// that no more traces should be written.
func (f *RotatingFileClient) Stop(ctx context.Context) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	err := f.file.Close()
	f.file = nil
	return trace.Wrap(err)
}

var ErrShutdown = errors.New("the client is shutdown")

// UploadTraces writes the provided spans to a file in the configured directory. If writing another span
// to the file would cause it to exceed the limit, then the file is first rotated before the write is
// attempted. In the event that Stop has already been called this will always return ErrShutdown.
func (f *RotatingFileClient) UploadTraces(ctx context.Context, protoSpans []*otlp.ResourceSpans) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.file == nil {
		return ErrShutdown
	}

	for _, span := range protoSpans {
		msg, err := protojson.Marshal(span)
		if err != nil {
			return trace.Wrap(err)
		}

		// Open a new file if this write would exceed the configured limit
		// *IF* we have already written data. Otherwise, we'll allow this
		// write to exceed the limit to prevent any empty files from existing.
		if uint64(len(msg))+f.written >= f.limit && f.written != 0 {
			newFile, err := os.CreateTemp(f.dir, fileName())
			if err != nil {
				return trace.ConvertSystemError(err)
			}

			var oldFile *os.File
			oldFile, f.file, f.written = f.file, newFile, 0
			_ = oldFile.Close()
		}

		msg = append(msg, '\n')
		n, err := f.file.Write(msg)
		f.written += uint64(n)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
	}

	return nil
}

// bufferedClient is an [otlptrace.Client] implementation that buffers uploaded
// spans in memory until a real client and all resource attributes are available.
type bufferedClient struct {
	logger *slog.Logger

	// mu synchronizes access to all below fields.
	mu sync.Mutex

	bufferedSpans [][]*otlp.ResourceSpans

	// client and closer will not be modified and are safe to reference without
	// locking after waitingForClient is false.
	waitingForClient bool
	client           otlptrace.Client
	closer           io.Closer
	started          bool

	// delayedResourceAttrs will not be modified and is safe to reference
	// without locking after waitingForDelayedResourceAttrs is false.
	waitingForDelayedResourceAttrs bool
	delayedResourceAttrs           []*otlpcommon.KeyValue
}

func newBufferedClient(cfg Config) (*bufferedClient, error) {
	bufClient := &bufferedClient{
		logger:                         cfg.Logger,
		waitingForClient:               cfg.WaitForDelayedClient,
		waitingForDelayedResourceAttrs: cfg.WaitForDelayedResourceAttrs,
	}
	if !cfg.WaitForDelayedClient {
		var err error
		bufClient.client, err = NewClient(cfg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return bufClient, nil
}

// Start implements [otlptrace.Client.Start].
func (b *bufferedClient) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started = true
	if b.client != nil {
		return trace.Wrap(b.client.Start(ctx))
	}
	return nil
}

// Stop implements [otlptrace.Client.Stop].
func (b *bufferedClient) Stop(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	var (
		stopErr  error
		closeErr error
	)
	if b.client != nil {
		stopErr = trace.Wrap(b.client.Stop(ctx))
	}
	if b.closer != nil {
		closeErr = trace.Wrap(b.closer.Close())
	}
	b.bufferedSpans = nil
	return trace.NewAggregate(stopErr, closeErr)
}

// UploadTraces implements [otlptrace.Client.UploadTraces]. It may be called
// concurrently. If the client is ready, the traces are immediately uploaded.
// Otherwise, the traces are buffered in memory until the delayed client or
// resource attributes are provided.
func (b *bufferedClient) UploadTraces(ctx context.Context, protoSpans []*otlp.ResourceSpans) error {
	b.mu.Lock()
	if b.ready() {
		b.mu.Unlock()
		return b.uploadTracesToClient(ctx, protoSpans, b.delayedResourceAttrs)
	}
	b.bufferedSpans = append(b.bufferedSpans, protoSpans)
	const maxBufferedSpanBatches = 32
	if len(b.bufferedSpans) > maxBufferedSpanBatches {
		// nil out the pointer in the backing array before reslicing so the GC
		// can reclaim the memory, these batches of spans could be relatively large.
		b.bufferedSpans[0] = nil
		b.bufferedSpans = b.bufferedSpans[1:]
	}
	b.mu.Unlock()
	return nil
}

// uploadTracesToClient must only be called after b is ready (both
// waitingForClient and waitingForDelayedResourceAttrs are false), no locking
// is required.
func (b *bufferedClient) uploadTracesToClient(ctx context.Context, protoSpans []*otlp.ResourceSpans, delayedResourceAttrs []*otlpcommon.KeyValue) error {
	// First append any delayed resource attributes to the spans.
	for _, protoSpan := range protoSpans {
		if protoSpan.Resource == nil {
			continue
		}
		protoSpan.Resource.Attributes = append(protoSpan.Resource.Attributes, delayedResourceAttrs...)
	}
	// Upload the spans with all attributes to the real client.
	return trace.Wrap(b.client.UploadTraces(ctx, protoSpans))
}

func (b *bufferedClient) setDelayedResourceAttrs(ctx context.Context, attrs []attribute.KeyValue) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.waitingForDelayedResourceAttrs {
		return trace.BadParameter("delayed resource attributes cannot be set more than once")
	}
	// Transform the attributes to the format required for use in uploadTracesToClient.
	b.delayedResourceAttrs = tracetransform.KeyValues(attrs)
	b.waitingForDelayedResourceAttrs = false
	b.uploadBufferedSpansIfReady(ctx)
	return nil
}

func (b *bufferedClient) setClient(ctx context.Context, client ClosableClient) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.waitingForClient {
		return trace.BadParameter("client cannot be set more than once")
	}
	b.client = client
	b.closer = client
	b.waitingForClient = false
	if b.started {
		if err := client.Start(ctx); err != nil {
			return trace.Wrap(err)
		}
	}
	b.uploadBufferedSpansIfReady(ctx)
	return nil
}

// uploadBufferedSpansIfReady is called when either delayed resource attributes
// or client is provided. If the client is now ready, it uploads all the
// buffered spans to the real client in a background goroutine.
func (b *bufferedClient) uploadBufferedSpansIfReady(ctx context.Context) {
	if b.ready() {
		// b.bufferedSpans will not be used after b becomes ready.
		// Setting the slice to nil should allow the GC to reclaim the memory
		// after all buffered spans have been uploaded and the goroutine exits.
		bufferedSpans := b.bufferedSpans
		b.bufferedSpans = nil
		go func() {
			// Spans are buffered and uploaded in their original chunk sizes as
			// passed to UploadTraces to avoid exceeding the maximum size of a
			// single message.
			for _, spans := range bufferedSpans {
				if err := b.uploadTracesToClient(ctx, spans, b.delayedResourceAttrs); err != nil {
					b.logger.WarnContext(ctx, "Failed to upload chunk of buffered spans", "error", err)
				}
			}
		}()
	}
}

func (b *bufferedClient) ready() bool {
	return !b.waitingForDelayedResourceAttrs && !b.waitingForClient
}
