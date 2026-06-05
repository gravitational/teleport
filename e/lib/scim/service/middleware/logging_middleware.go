package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
)

// LoggingMiddleware wraps a ResourceHandler to provide request/response logging
// for all SCIM operations. It logs endpoint details, payloads, and operation outcomes.
type LoggingMiddleware struct {
	LoggingMiddlewareConfig
}

// LoggingMiddlewareConfig contains the configuration for the logging middleware.
type LoggingMiddlewareConfig struct {
	// Log is the logger for recording SCIM operations.
	Log *slog.Logger
}

// checkAndSetDefaults validates the middleware configuration and sets default values.
func (l *LoggingMiddlewareConfig) checkAndSetDefaults() error {
	if l.Log == nil {
		l.Log = slog.Default()
	}
	return nil
}

// NewLoggingMiddleware creates a new logging middleware with the given configuration.
// The middleware will log all SCIM operations including endpoint details and payloads.
func NewLoggingMiddleware(config LoggingMiddlewareConfig) (*LoggingMiddleware, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &LoggingMiddleware{
		LoggingMiddlewareConfig: config,
	}, nil
}

// GetResourceMiddleware logs GetResource operations.
func (l *LoggingMiddleware) GetResourceMiddleware(ctx context.Context, req *pb.GetSCIMResourceRequest, next common.ResourceHandler) (resp *pb.Resource, err error) {
	log := newOperationLogger(l.Log, req.GetTarget(), "SCIMHandler/GetResource")

	log.InfoContext(ctx, "SCIM Request processing started")

	start := time.Now()
	resp, err = next.GetResource(ctx, req)
	duration := time.Since(start)

	log = log.With(slog.Duration("duration", duration))

	if err != nil {
		log.ErrorContext(ctx, "SCIM Request processing failed", "error", err)
		return nil, trace.Wrap(err)
	}

	log.InfoContext(ctx, "SCIM Request processed finished",
		slog.Group("response",
			slog.Any("resource", marshalProtoJSON(resp)),
		),
	)
	return resp, nil
}

// CreateResourceMiddleware logs CreateResource operations.
func (l *LoggingMiddleware) CreateResourceMiddleware(ctx context.Context, req *pb.CreateSCIMResourceRequest, next common.ResourceHandler) (resp *pb.Resource, err error) {
	log := newOperationLogger(l.Log, req.GetTarget(), "SCIMHandler/CreateResource")

	log.InfoContext(ctx, "SCIM Request processing started",
		slog.Group("request",
			slog.Any("resource", marshalProtoJSON(req.GetResource())),
		),
	)

	start := time.Now()
	resp, err = next.CreateResource(ctx, req)
	duration := time.Since(start)

	log = log.With(slog.Duration("duration", duration))

	if err != nil {
		log.ErrorContext(ctx, "SCIM Request processing failed", "error", err)
		return nil, trace.Wrap(err)
	}

	log.InfoContext(ctx, "SCIM Request processed finished",
		slog.Group("response",
			slog.Any("resource", marshalProtoJSON(resp)),
		),
	)
	return resp, nil
}

// ListResourcesMiddleware logs ListResources operations.
func (l *LoggingMiddleware) ListResourcesMiddleware(ctx context.Context, req *pb.ListSCIMResourcesRequest, next common.ResourceHandler) (resp *pb.ResourceList, err error) {
	log := newOperationLogger(l.Log, req.GetTarget(), "SCIMHandler/ListResources")

	log.InfoContext(ctx, "SCIM Request processing started",
		slog.Group("request",
			slog.String("filter", req.GetFilter()),
			slog.Uint64("start_index", req.GetPage().GetStartIndex()),
			slog.Uint64("count", req.GetPage().GetCount()),
		),
	)

	start := time.Now()
	resp, err = next.ListResources(ctx, req)
	duration := time.Since(start)

	log = log.With(slog.Duration("duration", duration))

	if err != nil {
		log.ErrorContext(ctx, "SCIM Request processing failed", "error", err)
		return nil, trace.Wrap(err)
	}

	log.InfoContext(ctx, "SCIM Request processed finished",
		slog.Group("response",
			slog.Int("total_results", int(resp.GetTotalResults())),
			slog.Int("items_per_page", int(resp.GetItemsPerPage())),
			slog.Int("start_index", int(resp.GetStartIndex())),
			slog.Int("resource_count", len(resp.GetResources())),
			slog.Any("resources", marshalProtoJSON(resp)),
		),
	)

	return resp, nil
}

// UpdateResourceMiddleware logs UpdateResource operations.
func (l *LoggingMiddleware) UpdateResourceMiddleware(ctx context.Context, req *pb.UpdateSCIMResourceRequest, next common.ResourceHandler) (resp *pb.Resource, err error) {
	log := newOperationLogger(l.Log, req.GetTarget(), "SCIMHandler/UpdateResource")

	log.InfoContext(ctx, "SCIM Request processing started",
		slog.Group("request",
			slog.Any("resource", marshalProtoJSON(req.GetResource())),
		),
	)

	start := time.Now()
	resp, err = next.UpdateResource(ctx, req)
	duration := time.Since(start)

	log = log.With(slog.Duration("duration", duration))

	if err != nil {
		log.ErrorContext(ctx, "SCIM Request processing failed", "error", err)
		return nil, trace.Wrap(err)
	}

	log.InfoContext(ctx, "SCIM Request processed finished",
		slog.Group("response",
			slog.Any("resource", marshalProtoJSON(resp)),
		),
	)
	return resp, nil
}

// DeleteResourceMiddleware logs DeleteResource operations.
func (l *LoggingMiddleware) DeleteResourceMiddleware(ctx context.Context, req *pb.DeleteSCIMResourceRequest, next common.ResourceHandler) (err error) {
	log := newOperationLogger(l.Log, req.GetTarget(), "SCIMHandler/DeleteResource")

	log.InfoContext(ctx, "SCIM Request processing started")

	start := time.Now()
	err = next.DeleteResource(ctx, req)
	duration := time.Since(start)

	log = log.With(slog.Duration("duration", duration))

	if err != nil {
		log.ErrorContext(ctx, "SCIM Request processing failed", "error", err)
		return trace.Wrap(err)
	}

	log.InfoContext(ctx, "SCIM Request processed finished")
	return nil
}

// PatchResourceMiddleware logs PatchResource operations.
func (l *LoggingMiddleware) PatchResourceMiddleware(ctx context.Context, req *pb.PatchSCIMResourceRequest, next common.ResourceHandler) (resp *pb.Resource, err error) {
	log := newOperationLogger(l.Log, req.GetTarget(), "SCIMHandler/PatchResource")

	log.InfoContext(ctx, "SCIM Request processing started",
		slog.Group("request",
			slog.Any("payload", marshalProtoJSON(req.GetPayload())),
		),
	)

	start := time.Now()
	resp, err = next.PatchResource(ctx, req)
	duration := time.Since(start)

	log = log.With(slog.Duration("duration", duration))

	if err != nil {
		log.ErrorContext(ctx, "SCIM Request processing failed", "error", err)
		return nil, trace.Wrap(err)
	}

	log.InfoContext(ctx, "SCIM Request processed finished",
		slog.Group("response",
			slog.Any("resource", marshalProtoJSON(resp)),
		),
	)
	return resp, nil
}

func marshalProtoJSON(v proto.Message) json.RawMessage {
	if v == nil {
		return nil
	}
	jsonBytes, err := protojson.Marshal(v)
	if err != nil {
		return json.RawMessage(fmt.Sprintf(`{"error": "marshaling failed: %v"}`, err))
	}
	return jsonBytes
}

func newOperationLogger(log *slog.Logger, target *pb.RequestTarget, operation string) *slog.Logger {
	return log.With(
		slog.String("op", operation),
		// TODO:(smallinsky) Leverage trace ID/request ID propagation.
		// The trace ID/Request ID should be included in responses to enable client-side correlation.
		slog.String("request_id", uuid.New().String()),
		slog.String("resource_id", target.GetResourceId()),
		slog.String("plugin_id", target.GetPluginId()),
		slog.String("resource_type", target.GetResourceType()),
	)
}
