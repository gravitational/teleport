// Copyright 2023 Gravitational, Inc
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

package grpc

import (
	"context"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryLoggingInterceptor creates an interceptor that logs gRPC errors.
func UnaryLoggingInterceptor(logger log.FieldLogger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		start := time.Now()
		resp, err = handler(ctx, req)
		if err != nil {
			logRPCError(ctx, logger, err, logInfo{
				fullMethod: info.FullMethod,
				methodType: "unary",
				start:      start,
			})
		}
		return resp, err
	}
}

// StreamLoggingInterceptor creates an interceptor that logs gRPC errors.
func StreamLoggingInterceptor(logger log.FieldLogger) grpc.StreamServerInterceptor {
	return func(server interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(server, stream)
		if err != nil {
			methodType := "bidi_stream"
			if !info.IsClientStream {
				methodType = "server_stream"
			}
			logRPCError(stream.Context(), logger, err, logInfo{
				fullMethod: info.FullMethod,
				methodType: methodType,
				start:      start,
			})
		}
		return err
	}
}

type logInfo struct {
	fullMethod string
	methodType string
	start      time.Time
}

func logRPCError(ctx context.Context, logger log.FieldLogger, err error, info logInfo) {
	var service, method string
	if tmp := strings.Split(info.fullMethod, "/"); len(tmp) == 3 {
		service = tmp[1]
		method = tmp[2]
	} else { // Shouldn't happen, but let's play it safe.
		service = info.fullMethod
		method = info.fullMethod
	}

	// Log format and fields loosely based on go-grpc-middleware/v2/interceptors/logging.
	fields := log.Fields{
		"grpc.code":        status.Code(err),
		"grpc.component":   "server",
		"grpc.error":       err.Error(),
		"grpc.method":      method,
		"grpc.method_type": info.methodType,
		"grpc.service":     service,
		"grpc.start_time":  info.start.Format(time.RFC3339),
		"grpc.time_ms":     time.Since(info.start).Milliseconds(),
		"protocol":         "grpc",
	}
	if deadline, ok := ctx.Deadline(); ok {
		fields["grpc.request.deadline"] = deadline.Format(time.RFC3339)
	}
	logger.
		WithFields(fields).
		Debug("finished call")
}
