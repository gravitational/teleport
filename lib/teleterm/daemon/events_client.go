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
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
)

// TshdEventsClient holds a lazily loaded [api.TshdEventsServiceClient].
type TshdEventsClient struct {
	client api.TshdEventsServiceClient
	// connectedChan is closed once the client is connected
	connectedChan chan struct{}
	// connectMu is used during connection to prevent a race between callers.
	connectMu sync.Mutex

	// credsFn lazily creates creds for the tshd events server ran by the Electron app.
	// This is to ensure that the server public key is written to the disk under the
	// expected location by the time we get around to creating the client.
	credsFn CreateTshdEventsClientCredsFunc
}

func NewTshdEventsClient(credsFn CreateTshdEventsClientCredsFunc) *TshdEventsClient {
	return &TshdEventsClient{
		credsFn:       credsFn,
		connectedChan: make(chan struct{}),
	}
}

// Connect connects to the given server address.
func (c *TshdEventsClient) Connect(serverAddress string) error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	select {
	case <-c.connectedChan:
		// already connected, no-op.
		return nil
	default:
	}

	withCreds, err := c.credsFn()
	if err != nil {
		return trace.Wrap(err)
	}

	conn, err := grpc.NewClient(serverAddress, withCreds)
	if err != nil {
		return trace.Wrap(err)
	}

	// Successfully connected set the client and signal to any waiters.
	c.client = api.NewTshdEventsServiceClient(conn)
	close(c.connectedChan)
	return nil
}

// GetClient retrieves the lazily loaded client. If the client is not yet loaded,
// this method will wait until it is loaded, the given context is closed, or it
// times out.
func (c *TshdEventsClient) GetClient(ctx context.Context) (api.TshdEventsServiceClient, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	select {
	case <-c.connectedChan:
		return c.client, nil
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err(), "tshd events client has not been initialized yet")
	}
}
