// Copyright 2022 Gravitational, Inc.
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
	"sync"

	"github.com/gravitational/trace/trail"
	"google.golang.org/grpc"
)

// streamWrapper wraps around the embedded grpc.ClientStream
// and intercepts the RecvMsg method calls decreading the number of
// streams counter.
type streamWrapper struct {
	grpc.ClientStream
	wg   *sync.WaitGroup
	once sync.Once
}

func (s *streamWrapper) CloseSend() error {
	err := s.ClientStream.CloseSend()
	s.decreaseCounter()
	return err
}

func (s *streamWrapper) SendMsg(m interface{}) error {
	err := s.ClientStream.SendMsg(m)
	if err != nil {
		s.decreaseCounter()
	}
	return err
}

func (s *streamWrapper) RecvMsg(m interface{}) error {
	err := s.ClientStream.RecvMsg(m)
	if err != nil {
		s.decreaseCounter()
	}
	return err
}

func (s *streamWrapper) decreaseCounter() {
	s.once.Do(func() {
		s.wg.Done()
	})
}

// streamCounterInterceptor is GPRC client stream interceptor that
// counts the number of current open streams for the purpose of
// gracefully shutdown a draining grpc client.
func streamCounterInterceptor(wg *sync.WaitGroup) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		s, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			return nil, trail.ToGRPC(err)
		}
		wg.Add(1)
		return &streamWrapper{
			ClientStream: s,
			wg:           wg,
		}, nil
	}
}
