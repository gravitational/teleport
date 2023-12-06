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

// streamCounterInterceptor is gRPC client stream interceptor that
// counts the number of current open streams for the purpose of
// gracefully shutdown a draining gRPC client.
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
