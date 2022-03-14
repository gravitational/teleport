// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"io"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

type ctxKey struct{}

type (
	serviceKey    ctxKey
	methodKey     ctxKey
	remoteAddrKey ctxKey
	localAddrKey  ctxKey
)

// StatsHandler is for gRPC stats
type statsHandler struct {
	reporter *reporter
}

func newStatsHandler(m metrics) stats.Handler {
	return &statsHandler{
		reporter: newReporter(m),
	}
}

// TagConn implements per-Connection context tagging.
func (s *statsHandler) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	ctx = context.WithValue(ctx, remoteAddrKey{}, info.RemoteAddr.String())
	ctx = context.WithValue(ctx, localAddrKey{}, info.LocalAddr.String())
	return ctx
}

// HandleRPC implements per-Connection stats reporting.
func (s *statsHandler) HandleConn(ctx context.Context, connStats stats.ConnStats) {
	remoteAddr, _ := ctx.Value(remoteAddrKey{}).(string)
	localAddr, _ := ctx.Value(localAddrKey{}).(string)

	switch connStats.(type) {
	case *stats.ConnBegin:
		s.reporter.incConnection(localAddr, remoteAddr)
	case *stats.ConnEnd:
		s.reporter.decConnection(localAddr, remoteAddr)
	}
}

// TagRPC implements per-RPC context tagging.
func (s *statsHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	service, method := split(info.FullMethodName)
	ctx = context.WithValue(ctx, serviceKey{}, service)
	ctx = context.WithValue(ctx, methodKey{}, method)
	return ctx
}

// HandleRPC implements per-RPC stats reporting.
func (s *statsHandler) HandleRPC(ctx context.Context, rpcStats stats.RPCStats) {
	service, _ := ctx.Value(serviceKey{}).(string)
	method, _ := ctx.Value(methodKey{}).(string)

	switch rs := rpcStats.(type) {
	case *stats.InPayload:
		s.reporter.measureMessageReceived(service, method, float64(rs.WireLength))
	case *stats.OutPayload:
		s.reporter.measureMessageSent(service, method, float64(rs.WireLength))
	case *stats.Begin:
		s.reporter.incRPC(service, method)
	case *stats.End:
		code := codes.OK.String()
		if isError(rs.Error) {
			code = status.Code(rs.Error).String()
		}
		s.reporter.decRPC(service, method)
		s.reporter.countRPC(service, method, code)
		s.reporter.timeRPC(service, method, code, rs.EndTime.Sub(rs.BeginTime))
	}
}

// split splits a grpc request path into service and method strings
// request format /%s/%s
func split(request string) (string, string) {
	if i := strings.LastIndex(request, "/"); i >= 0 {
		return request[1:i], request[i+1:]
	}
	return "unknown", "unknown"
}

// isError returns false if the supplied error
// - is nil
// - has a codes.OK code
// - is io.EOF
func isError(err error) bool {
	if err == nil {
		return false
	}

	grpcErr := status.Convert(err)
	code := grpcErr.Code()
	if code == codes.OK {
		return false
	}

	eof := status.Convert(io.EOF)
	return code != eof.Code() || grpcErr.Message() != eof.Message()
}
