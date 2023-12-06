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

package peer

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

func newStatsHandler(r *reporter) stats.Handler {
	return &statsHandler{
		reporter: r,
	}
}

// TagConn implements per-Connection context tagging.
func (s *statsHandler) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	ctx = context.WithValue(ctx, remoteAddrKey{}, info.RemoteAddr.String())
	ctx = context.WithValue(ctx, localAddrKey{}, info.LocalAddr.String())
	return ctx
}

// HandleConn implements per-Connection stats reporting.
func (s *statsHandler) HandleConn(ctx context.Context, connStats stats.ConnStats) {
	// client connection stats are monitored by the monitor() function in client.go
	if connStats.IsClient() {
		return
	}

	remoteAddr, _ := ctx.Value(remoteAddrKey{}).(string)
	localAddr, _ := ctx.Value(localAddrKey{}).(string)

	switch connStats.(type) {
	case *stats.ConnBegin:
		s.reporter.incConnection(localAddr, remoteAddr, "SERVER_CONN")
	case *stats.ConnEnd:
		s.reporter.decConnection(localAddr, remoteAddr, "SERVER_CONN")
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
