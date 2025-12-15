/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
)

func TestTshdEventsClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, addr := newMockTSHDEventsServiceServer(t)

	c := NewTshdEventsClient(func() (grpc.DialOption, error) {
		return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
	})

	// GetClient should timeout if client is not connected.
	timeoutCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	_, err := c.GetClient(timeoutCtx)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// Make 2 calls to GetClient to wait for a client connection.
	timeoutCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	type getClientRet struct {
		clt api.TshdEventsServiceClient
		err error
	}
	retC := make(chan getClientRet)
	for range 2 {
		go func() {
			clt, err := c.GetClient(timeoutCtx)
			retC <- getClientRet{
				clt: clt,
				err: err,
			}
		}()
	}

	// Connect client, GetClient calls should complete.
	err = c.Connect(addr)
	require.NoError(t, err)

	for range 2 {
		select {
		case <-timeoutCtx.Done():
			t.Error("timeout waiting for client connection")
		case ret := <-retC:
			require.NoError(t, ret.err)
			require.NotNil(t, ret.clt)
		}
	}

	// GetClient should complete immediately once connected.
	timeoutCtx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	_, err = c.GetClient(timeoutCtx)
	require.NoError(t, err)
}
