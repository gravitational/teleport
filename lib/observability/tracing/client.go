package tracing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otlp "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const DefaultFileLimit uint64 = 1024 * 1024 * 1024

var _ otlptrace.Client = (*noopClient)(nil)

type noopClient struct{}

func (n noopClient) Start(context.Context) error                               { return nil }
func (n noopClient) Stop(context.Context) error                                { return nil }
func (n noopClient) UploadTraces(context.Context, []*otlp.ResourceSpans) error { return nil }

// NewNoopClient returns an oltptrace.Client that does nothing
func NewNoopClient() otlptrace.Client {
	return &noopClient{}
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

type writeCounter struct {
	io.WriteCloser
	written uint64
}

func newWriteCounter(w io.WriteCloser) *writeCounter {
	return &writeCounter{
		WriteCloser: w,
	}
}

func (c *writeCounter) Write(p []byte) (n int, err error) {
	n, err = c.WriteCloser.Write(p)
	c.written += uint64(n)
	return n, err
}

var _ otlptrace.Client = (*RotatingFileClient)(nil)

type RotatingFileClient struct {
	dir       string
	limit     uint64
	marshaler jsonpb.Marshaler

	lock   sync.Mutex
	writer *writeCounter
}

func NewRotatingFileClient(dir string, limit uint64) (*RotatingFileClient, error) {
	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, trace.ConvertSystemError(err)
	}

	f, err := os.CreateTemp(dir, fmt.Sprintf("%d-trace-*.json", time.Now().UTC().UnixNano()))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	return &RotatingFileClient{
		dir:    dir,
		limit:  limit,
		writer: newWriteCounter(f),
	}, nil
}

func (f *RotatingFileClient) Start(ctx context.Context) error {
	return nil
}

func (f *RotatingFileClient) Stop(ctx context.Context) error {
	f.lock.Lock()
	f.lock.Unlock()
	return trace.Wrap(f.writer.Close())
}

func (f *RotatingFileClient) UploadTraces(ctx context.Context, protoSpans []*otlp.ResourceSpans) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	for _, span := range protoSpans {
		if err := f.marshaler.Marshal(f.writer, span); err != nil {
			return trace.Wrap(err)
		}

		if f.writer.written >= f.limit {
			newFile, err := os.CreateTemp(f.dir, fmt.Sprintf("%d-trace-*.json", time.Now().UTC().UnixNano()))
			if err != nil {
				return trace.ConvertSystemError(err)
			}

			var oldWriter io.Closer
			oldWriter, f.writer = f.writer, newWriteCounter(newFile)
			_ = oldWriter.Close()
		}
	}

	return nil
}
