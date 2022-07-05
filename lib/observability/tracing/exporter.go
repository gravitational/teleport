package tracing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	otlp "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var _ otlptrace.Client = (*noopClient)(nil)

type noopClient struct{}

func (n noopClient) Start(context.Context) error {
	return nil
}

func (n noopClient) Stop(context.Context) error {
	return nil
}

func (n noopClient) UploadTraces(context.Context, []*otlp.ResourceSpans) error {
	return nil
}

// NewNoopClient returns an oltptrace.Client that does nothing
func NewNoopClient() otlptrace.Client {
	return &noopClient{}
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
		httpOptions = append(httpOptions, otlptracehttp.WithEndpoint(cfg.ExporterURL[len(cfg.exporterURL.Scheme)+3:]))
		traceClient = otlptracehttp.NewClient(httpOptions...)
	case "grpc":
		grpcOptions = append(grpcOptions, otlptracegrpc.WithEndpoint(cfg.ExporterURL[len(cfg.exporterURL.Scheme)+3:]))
		traceClient = otlptracegrpc.NewClient(grpcOptions...)
	default:
		return nil, trace.BadParameter("unsupported exporter scheme: %q", cfg.exporterURL.Scheme)
	}

	return traceClient, nil
}

// NewOTLPExporter returns a new exporter that exports spans via an otlptrace.Client.
func NewOTLPExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	traceClient, err := NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()
	exporter, err := otlptrace.New(ctx, traceClient)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return nil, trace.ConnectionProblem(err, "failed to connect to tracing exporter %s: %v", cfg.ExporterURL, err)
	case err != nil:
		return nil, trace.NewAggregate(err, traceClient.Stop(context.Background()))
	}

	return exporter, nil
}

type RotatingFileExporter struct {
	dir   string
	limit int

	lock    sync.Mutex
	written int
	f       *os.File
}

func NewRotatingFileExporter(dir string, limit int) (*RotatingFileExporter, error) {
	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, trace.ConvertSystemError(err)
	}

	f, err := os.CreateTemp(dir, fmt.Sprintf("%d-trace-*.json", time.Now().UTC().UnixNano()))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	return &RotatingFileExporter{
		dir:   dir,
		limit: limit,
		f:     f,
	}, nil

}

func (f *RotatingFileExporter) Write(p []byte) (int, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	n, err := f.f.Write(p)
	f.written += n
	if err != nil {
		return n, trace.Wrap(err)
	}

	if f.written < f.limit {
		return n, nil
	}

	newFile, err := os.CreateTemp(f.dir, fmt.Sprintf("%d-trace-*.json", time.Now().UTC().UnixNano()))
	if err != nil {
		return n, trace.ConvertSystemError(err)
	}

	var oldFile *os.File
	oldFile, f.f = f.f, newFile
	_ = oldFile.Close()
	f.written = 0

	return n, nil
}

func (f *RotatingFileExporter) Close() error {
	f.lock.Lock()
	f.lock.Unlock()
	return trace.Wrap(f.f.Close())
}

// wrappedExporter is a sdktrace.SpanExporter wrapper that is used to ensure that any
// io.Closer that are provided to NewExporter are closed when the Exporter is
// Shutdown.
type wrappedExporter struct {
	closer   io.Closer
	exporter sdktrace.SpanExporter
}

// ExportSpans forwards the spans to the wrapped exporter.
func (w *wrappedExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	return w.exporter.ExportSpans(ctx, spans)
}

// Shutdown shuts down the wrapped exporter and closes the client.
func (w *wrappedExporter) Shutdown(ctx context.Context) error {
	return trace.NewAggregate(w.exporter.Shutdown(ctx), w.closer.Close())
}

// NewFileExporter returns a new exporter that exports spans to a file.
func NewFileExporter(dir string) (sdktrace.SpanExporter, error) {
	f, err := NewRotatingFileExporter(dir, 100000000)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	exporter, err := stdouttrace.New(stdouttrace.WithWriter(f))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &wrappedExporter{
		exporter: exporter,
		closer:   f,
	}, nil
}

// NewExporter returns a new exporter that is configured per the provided Config.
func NewExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case cfg.exporterURL.Scheme == "http",
		cfg.exporterURL.Scheme == "https",
		cfg.exporterURL.Scheme == "grpc":
		return NewOTLPExporter(ctx, cfg)
	case cfg.exporterURL.Scheme == "file":
		return NewFileExporter(cfg.exporterURL.Path)
	default:
		return nil, trace.BadParameter("unsupported exporter scheme: %q", cfg.exporterURL.Scheme)
	}
}
