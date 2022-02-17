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

	"google.golang.org/grpc"
)

// streamClientInterceptor is GPRC stream interceptor that handles:
// - reporting / metrics.
// - converting errors to the appropriate grpc status code.
func streamClientInterceptor(metrics *clientMetrics) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		requestReporter := newRequestReporter(metrics, method)

		s, err := streamer(ctx, desc, cc, method, opts...)
		grpcErr := toGRPCError(err)
		if grpcErr != nil {
			requestReporter.reportCall(grpcErr)
			return nil, grpcErr
		}
		return newStreamClientWrapper(s, requestReporter), nil
	}
}

// streamClientWrapper wraps around the embedded grpc.ClientStream
// and intercepts the RecvMsg and SendMsg method calls:
// - reporting metrics
// - converting errors to the appropriate grpc status.
type streamClientWrapper struct {
	grpc.ClientStream
	reporter *requestReporter
}

// newStreamClientWrapper is a wrapper for grpc.ClientStream
func newStreamClientWrapper(s grpc.ClientStream, reporter *requestReporter) grpc.ClientStream {
	return &streamClientWrapper{
		ClientStream: s,
		reporter:     reporter,
	}
}

// SendMsg wraps around ClientStream.SendMsg and adds metrics reporting
func (s *streamClientWrapper) SendMsg(m interface{}) error {
	err := toGRPCError(s.ClientStream.SendMsg(m))
	s.reporter.reportMsgSent(err, getMessageSize(m))
	return err
}

// RecvMsg wraps around ClientStream.RecvMsg and adds metrics reporting
func (s *streamClientWrapper) RecvMsg(m interface{}) error {
	err := toGRPCError(s.ClientStream.RecvMsg(m))
	s.reporter.reportMsgReceived(err, getMessageSize(m))
	if err == nil {
		return nil
	}
	s.reporter.reportCall(checkError(err))
	return err
}
