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
	"fmt"
	"io"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errorStreamInterceptor is a GPRC stream interceptor that handles converting
// errors to the appropriate grpc status code.
func streamServerInterceptor(metrics *serverMetrics) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		reporter := newServerReporter(info.FullMethod, metrics)
		wrapper := newStreamServerWrapper(stream, reporter)
		err := toGRPCError(handler(srv, wrapper))
		// TODO Naji: check if io.EOF is returned here
		fmt.Printf("------------------------------------ server interceptor error: %+v\n", err)
		reporter.reportCall(err)
		return err
	}
}

// streamServerWrapper wraps around the embedded grpc.ServerStream
// and intercepts the RecvMsg and SendMsg method calls:
// - reporting metrics
// - converting errors to the appropriate grpc status.
type streamServerWrapper struct {
	grpc.ServerStream
	reporter reporter
}

// newStreamServerWraper is a wrapper for grpc.ServerStream
func newStreamServerWrapper(s grpc.ServerStream, rep reporter) grpc.ServerStream {
	return &streamServerWrapper{
		ServerStream: s,
		reporter:     rep,
	}
}

// SendMsg wraps around ServerStream.SendMsg and adds metrics reporting
func (s *streamServerWrapper) SendMsg(m interface{}) error {
	err := toGRPCError(s.ServerStream.SendMsg(m))
	s.reporter.reportMsgSent(err)
	return err
}

// RecvMsg wraps around ServerStream.RecvMsg and adds metrics reporting
func (s *streamServerWrapper) RecvMsg(m interface{}) error {
	err := toGRPCError(s.ServerStream.RecvMsg(m))
	// TODO Naji: check if io.EOF is returned here
	fmt.Printf("------------------------------------ server interceptor receive error: %+v\n", err)
	s.reporter.reportMsgReceived(err)
	return err
}

// streamClientInterceptor is GPRC stream interceptor that handles:
// - reporting / metrics.
// - converting errors to the appropriate grpc status code.
func streamClientInterceptor(metrics *clientMetrics) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		reporter := newClientReporter(method, metrics)

		s, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			reporter.reportCall(toGRPCError(err))
			return nil, err
		}
		return newStreamClientWrapper(s, reporter), nil
	}
}

// streamClientWrapper wraps around the embedded grpc.ClientStream
// and intercepts the RecvMsg and SendMsg method calls:
// - reporting metrics
// - converting errors to the appropriate grpc status.
type streamClientWrapper struct {
	grpc.ClientStream
	reporter reporter
}

// newStreamClientWrapper is a wrapper for grpc.ClientStream
func newStreamClientWrapper(s grpc.ClientStream, rep reporter) grpc.ClientStream {
	return &streamClientWrapper{
		ClientStream: s,
		reporter:     rep,
	}
}

// SendMsg wraps around ClientStream.SendMsg and adds metrics reporting
func (s *streamClientWrapper) SendMsg(m interface{}) error {
	err := toGRPCError(s.ClientStream.SendMsg(m))
	s.reporter.reportMsgSent(err)
	return err
}

// RecvMsg wraps around ClientStream.RecvMsg and adds metrics reporting
func (s *streamClientWrapper) RecvMsg(m interface{}) error {
	err := toGRPCError(s.ClientStream.RecvMsg(m))
	s.reporter.reportMsgReceived(err)
	if err == nil {
		return nil
	}

	var callError error
	// TODO Naji: test this
	if err != io.EOF {
		callError = err
	}
	s.reporter.reportCall(callError)

	return err
>>>>>>> fd1919ea6 (proxy client first pass)
}

// errorHandler converts trace errors to grpc errors with appropriate status codes.
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if trace.IsNotFound(err) {
		return status.Error(codes.NotFound, err.Error())
	}
	if trace.IsBadParameter(err) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}
