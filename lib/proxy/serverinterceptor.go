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
	"errors"
	"io"

	clientapi "github.com/gravitational/teleport/api/client/proto"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errorStreamInterceptor is a GPRC stream interceptor that handles converting
// errors to the appropriate grpc status code.
func streamServerInterceptor(metrics *serverMetrics) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		requestReporter := newRequestReporter(metrics, info.FullMethod)
		wrapper := newStreamServerWrapper(stream, requestReporter)
		err := toGRPCError(handler(srv, wrapper))
		requestReporter.reportCall(checkError(err))
		return err
	}
}

// streamServerWrapper wraps around the embedded grpc.ServerStream
// and intercepts the RecvMsg and SendMsg method calls:
// - reporting metrics
// - converting errors to the appropriate grpc status.
type streamServerWrapper struct {
	grpc.ServerStream
	reporter *requestReporter
}

// newStreamServerWraper is a wrapper for grpc.ServerStream
func newStreamServerWrapper(s grpc.ServerStream, reporter *requestReporter) grpc.ServerStream {
	return &streamServerWrapper{
		ServerStream: s,
		reporter:     reporter,
	}
}

// SendMsg wraps around ServerStream.SendMsg and adds metrics reporting
func (s *streamServerWrapper) SendMsg(m interface{}) error {
	err := toGRPCError(s.ServerStream.SendMsg(m))
	s.reporter.reportMsgSent(err, getMessageSize(m))
	return err
}

// RecvMsg wraps around ServerStream.RecvMsg and adds metrics reporting
func (s *streamServerWrapper) RecvMsg(m interface{}) error {
	err := toGRPCError(s.ServerStream.RecvMsg(m))
	s.reporter.reportMsgReceived(err, getMessageSize(m))
	return err
}

func getMessageSize(message interface{}) int {
	f, ok := message.(*clientapi.Frame)
	if !ok {
		return 0
	}
	m := f.GetMessage()
	if m == nil {
		return 0
	}
	return m.Size()
}

// errorHandler converts trace errors to grpc errors with appropriate status codes.
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) {
		return status.Error(codes.OK, err.Error())
	}
	if trace.IsNotFound(err) {
		return status.Error(codes.NotFound, err.Error())
	}
	if trace.IsBadParameter(err) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

func checkError(err error) error {
	var callError error
	if status.Code(err) != codes.OK {
		callError = err
	}
	return callError
}
