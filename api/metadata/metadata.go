/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metadata

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport/api"
)

const (
	VersionKey                       = "version"
	SessionRecordingFormatContextKey = "session-recording-format"
)

// defaultMetadata returns the default metadata which will be added to all outgoing calls.
func defaultMetadata() map[string]string {
	return map[string]string{
		VersionKey: api.Version,
	}
}

// AddMetadataToContext returns a new context copied from ctx with the given
// raw metadata added. Metadata already set on the given context for any key
// will not be overridden, but new key/value pairs will always be added.
func AddMetadataToContext(ctx context.Context, raw map[string]string) context.Context {
	md := metadata.New(raw)
	if existingMd, ok := metadata.FromOutgoingContext(ctx); ok {
		for key, vals := range existingMd {
			md.Set(key, vals...)
		}
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// DisableInterceptors can be set on the client context with context.WithValue(ctx, DisableInterceptors{}, struct{}{})
// to stop the client interceptors from adding any metadata to the context (useful for testing).
type DisableInterceptors struct{}

// StreamServerInterceptor intercepts a gRPC client stream call and adds
// default metadata to the context.
func StreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if disable := stream.Context().Value(DisableInterceptors{}); disable == nil {
		header := metadata.New(defaultMetadata())
		grpc.SetHeader(stream.Context(), header)
	}
	return handler(srv, stream)
}

// UnaryServerInterceptor intercepts a gRPC server unary call and adds default
// metadata to the context.
func UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	if disable := ctx.Value(DisableInterceptors{}); disable == nil {
		header := metadata.New(defaultMetadata())
		grpc.SetHeader(ctx, header)
	}
	return handler(ctx, req)
}

// StreamClientInterceptor intercepts a gRPC client stream call and adds
// default metadata to the context.
func StreamClientInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	if disable := ctx.Value(DisableInterceptors{}); disable == nil {
		ctx = AddMetadataToContext(ctx, defaultMetadata())
	}
	return streamer(ctx, desc, cc, method, opts...)
}

// UnaryClientInterceptor intercepts a gRPC client unary call and adds default
// metadata to the context.
func UnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if disable := ctx.Value(DisableInterceptors{}); disable == nil {
		ctx = AddMetadataToContext(ctx, defaultMetadata())
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

// ClientVersionFromContext can be called from a gRPC server method to return
// the client version that was added to the gRPC metadata by
// StreamClientInterceptor or UnaryClientInterceptor on the client.
func ClientVersionFromContext(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}

	return VersionFromMetadata(md)
}

// VersionFromMetadata attempts to extract the standard version metadata value that is
// added to client and server headers by the interceptors in this package.
func VersionFromMetadata(md metadata.MD) (string, bool) {
	versionList := md.Get(VersionKey)
	if len(versionList) != 1 {
		return "", false
	}
	return versionList[0], true
}

// WithUserAgentFromTeleportComponent returns a grpc.DialOption that reports
// the Teleport component and the API version for user agent.
func WithUserAgentFromTeleportComponent(component string) grpc.DialOption {
	return grpc.WithUserAgent(fmt.Sprintf("%s/%s", component, api.Version))
}

// UserAgentFromContext returns the user agent from GRPC client metadata.
func UserAgentFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get("user-agent")
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, " ")
}

// WithSessionRecordingFormatContext returns a context.Context containing the
// format of the accessed session recording.
func WithSessionRecordingFormatContext(ctx context.Context, format string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, SessionRecordingFormatContextKey, format)
}

// SessionRecordingFormatFromContext returns the format of the accessed session
// recording (if present).
func SessionRecordingFormatFromContext(ctx context.Context) string {
	values := metadata.ValueFromIncomingContext(ctx, SessionRecordingFormatContextKey)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}
