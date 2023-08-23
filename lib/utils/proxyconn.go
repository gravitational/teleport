/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"io"

	"github.com/gravitational/trace"
)

// CombinedReadWriteCloser wraps an [io.ReadCloser] and an [io.WriteCloser] to
// implement [io.ReadWriteCloser]. Reads are performed on the [io.ReadCloser] and
// writes are performed on the [io.WriteCloser]. Closing will return the
// aggregated errors of both.
type CombinedReadWriteCloser struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (o CombinedReadWriteCloser) Read(p []byte) (int, error) {
	return o.r.Read(p)
}

func (o CombinedReadWriteCloser) Write(p []byte) (int, error) {
	return o.w.Write(p)
}

func (o CombinedReadWriteCloser) Close() error {
	return trace.NewAggregate(o.r.Close(), o.w.Close())
}

// CombineReadWriteCloser creates a CombinedReadWriteCloser from the provided
// [io.ReadCloser] and [io.WriteCloser] that implements [io.ReadWriteCloser]
func CombineReadWriteCloser(r io.ReadCloser, w io.WriteCloser) CombinedReadWriteCloser {
	return CombinedReadWriteCloser{
		r: r,
		w: w,
	}
}

// ProxyConn launches a double-copy loop that proxies traffic between the
// provided client and server connections.
//
// Exits when one or both copies stop, or when the context is canceled, and
// closes both connections.
func ProxyConn(ctx context.Context, client, server io.ReadWriteCloser) error {
	errCh := make(chan error, 2)

	defer server.Close()
	defer client.Close()

	go func() {
		defer server.Close()
		defer client.Close()
		_, err := io.Copy(server, client)
		errCh <- err
	}()

	go func() {
		defer server.Close()
		defer client.Close()
		_, err := io.Copy(client, server)
		errCh <- err
	}()

	var errors []error
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			if err != nil && !IsOKNetworkError(err) {
				errors = append(errors, err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return trace.NewAggregate(errors...)
}
