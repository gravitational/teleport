/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package apiserver

import (
	"context"
	"log/slog"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWithUnaryErrorHandling(t *testing.T) {
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	interceptor := withUnaryErrorHandling(slog.New(slog.DiscardHandler))

	t.Run("passes through successful response", func(t *testing.T) {
		handler := func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		}

		resp, err := interceptor(t.Context(), nil, info, handler)
		require.NoError(t, err)
		require.Equal(t, "ok", resp)
	})

	t.Run("converts trace error to gRPC status", func(t *testing.T) {
		handler := func(ctx context.Context, req any) (any, error) {
			return nil, trace.NotFound("missing")
		}

		resp, err := interceptor(t.Context(), nil, info, handler)
		require.Nil(t, resp)
		require.Equal(t, codes.NotFound, status.Code(err))
	})
}

func TestWithStreamErrorHandling(t *testing.T) {
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/Method"}
	interceptor := withStreamErrorHandling(slog.New(slog.DiscardHandler))

	t.Run("passes through successful response", func(t *testing.T) {
		handler := func(srv any, stream grpc.ServerStream) error {
			return nil
		}

		err := interceptor(nil, &fakeServerStream{ctx: t.Context()}, info, handler)
		require.NoError(t, err)
	})

	t.Run("converts trace error to gRPC status", func(t *testing.T) {
		handler := func(srv any, stream grpc.ServerStream) error {
			return trace.NotFound("missing")
		}

		err := interceptor(nil, &fakeServerStream{ctx: t.Context()}, info, handler)
		require.Equal(t, codes.NotFound, status.Code(err))
	})
}

func TestWithUnaryPanicRecovery(t *testing.T) {
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	interceptor := withUnaryPanicRecovery(slog.New(slog.DiscardHandler))

	handler := func(ctx context.Context, req any) (any, error) {
		panic("oh no")
	}

	resp, err := interceptor(t.Context(), nil, info, handler)
	require.Nil(t, resp)
	require.Equal(t, codes.Internal, status.Code(err))
	require.ErrorContains(t, err, "handler panic")
}

func TestWithStreamPanicRecovery(t *testing.T) {
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/Method"}
	interceptor := withStreamPanicRecovery(slog.New(slog.DiscardHandler))

	handler := func(srv any, stream grpc.ServerStream) error {
		panic("oh no")
	}

	err := interceptor(nil, &fakeServerStream{ctx: t.Context()}, info, handler)
	require.Equal(t, codes.Internal, status.Code(err))
	require.ErrorContains(t, err, "handler panic")
}

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *fakeServerStream) Context() context.Context { return s.ctx }
