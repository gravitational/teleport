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

package utils

import (
	"context"
	"io"
	"os"

	"github.com/gravitational/trace"
)

// CombinedStdio reads from standard input and writes to standard output.
// Closing a CombinedStdio does nothing, successfully.
type CombinedStdio struct{}

// Read reads from [os.Stdin].
func (CombinedStdio) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

// Write writes to [os.Stdout].
func (CombinedStdio) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

func (CombinedStdio) Close() error {
	return nil
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
			// Cause(ctx) returns ctx.Err() if no cause is provided.
			return trace.Wrap(context.Cause(ctx))
		}
	}

	return trace.NewAggregate(errors...)
}
