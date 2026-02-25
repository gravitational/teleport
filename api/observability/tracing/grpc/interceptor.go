package grpc

import (
	"context"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	grpcmd "google.golang.org/grpc/metadata"
)

const requestIDHeaderKey = "teleport_request_id"

func idFromContext(ctx context.Context) (string, bool) {
	untypedID := ctx.Value("request_id")
	if untypedID == nil {
		return "", false
	}

	if id, ok := untypedID.(string); ok {
		return id, true
	}

	return "", false
}

func ClientInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if requestID, hasID := idFromContext(ctx); hasID {
		ctx = grpcmd.AppendToOutgoingContext(ctx, requestIDHeaderKey, requestID)
	}

	var headers grpcmd.MD
	opts = append(opts, grpc.Header(&headers))
	err := invoker(ctx, method, req, reply, cc, opts...)

	var requestID string
	if values, ok := headers[requestIDHeaderKey]; ok && len(values) > 0 {
		requestID = values[0]
	}
	return trace.Wrap(err, "request id: [%s]", requestID)
}

func ServerInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	var requestID string

	md, ok := grpcmd.FromIncomingContext(ctx)
	if ok {
		if values, ok := md[requestIDHeaderKey]; ok && len(values) > 0 {
			requestID = values[0]
		}
	}

	if requestID == "" {
		requestID = uuid.NewString()
	}

	ctxWithID := context.WithValue(ctx, "request_id", requestID)
	resp, err := handler(ctxWithID, req)
	outgoingHeaders := grpcmd.Pairs(requestIDHeaderKey, requestID)
	grpc.SendHeader(ctx, outgoingHeaders)

	return resp, trace.Wrap(err)
}
