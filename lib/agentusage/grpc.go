/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package agentusage

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
)

const metadataKey = "teleport-agent-usage-bin"

// UnaryServerInterceptor reads evidence from incoming gRPC metadata into the
// handler context.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(incomingContext(ctx), req)
	}
}

// StreamServerInterceptor reads evidence from incoming gRPC metadata into the
// stream context.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, evidenceStream{ServerStream: ss, ctx: incomingContext(ss.Context())})
	}
}

type evidenceStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s evidenceStream) Context() context.Context {
	return s.ctx
}

func incomingContext(ctx context.Context) context.Context {
	values := metadata.ValueFromIncomingContext(ctx, metadataKey)
	if len(values) != 1 {
		return ctx
	}
	return ContextWithEvidence(ctx, parse([]byte(values[0])))
}

// UnaryClientInterceptor forwards evidence from the context as outgoing gRPC
// metadata.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(outgoingContext(ctx), method, req, reply, cc, opts...)
	}
}

// StreamClientInterceptor forwards evidence from the context as outgoing gRPC
// metadata.
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(outgoingContext(ctx), desc, cc, method, opts...)
	}
}

func outgoingContext(ctx context.Context) context.Context {
	encoded := encode(FromContext(ctx))
	if len(encoded) == 0 {
		return ctx
	}
	return withOutgoingEvidence(ctx, encoded)
}

func withOutgoingEvidence(ctx context.Context, encoded []byte) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)
	if md == nil {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	md.Set(metadataKey, string(encoded))
	return metadata.NewOutgoingContext(ctx, md)
}

// DialOptions returns interceptors that add the evidence as gRPC metadata to
// every RPC on the connection. If the caller adds UnaryClientInterceptor or
// StreamClientInterceptor after these options, evidence in the request context
// replaces this value.
func DialOptions(evidence *prehogv1a.AgentUsageEvidence) []grpc.DialOption {
	encoded := encode(evidence)
	if len(encoded) == 0 {
		return nil
	}
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(withOutgoingEvidence(ctx, encoded), method, req, reply, cc, opts...)
		}),
		grpc.WithChainStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(withOutgoingEvidence(ctx, encoded), desc, cc, method, opts...)
		}),
	}
}
