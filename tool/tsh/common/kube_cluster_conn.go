/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"context"
	"sync"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/client"
)

// clusterConn is a shared cluster connection,
// dialed on demand and closed when the last holder releases it,
// so none is held while idle.
type clusterConn struct {
	dialer clusterDialer
	conn   kubeCertClient
	// holders counts the in-flight operations sharing conn.
	holders int
	mu      sync.Mutex
}

type clusterDialer interface {
	DialCluster(ctx context.Context) (kubeCertClient, error)
}

func newClusterConn(tc *client.TeleportClient) *clusterConn {
	return &clusterConn{dialer: teleportClusterDialer{tc: tc}}
}

// teleportClusterDialer dials a cluster connection through a TeleportClient, with relogin if needed.
type teleportClusterDialer struct {
	tc *client.TeleportClient
}

func (d teleportClusterDialer) DialCluster(ctx context.Context) (kubeCertClient, error) {
	var clusterClient *client.ClusterClient
	if err := client.RetryWithRelogin(ctx, d.tc, func() error {
		ctx, cancel := context.WithTimeout(ctx, apidefaults.DefaultIOTimeout)
		defer cancel()

		var err error
		clusterClient, err = d.tc.ConnectToCluster(ctx)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return clusterClient, nil
}

// Acquire returns the shared cluster connection and a release function,
// dialing when no operation holds a connection.
// Concurrent and back-to-back operations share one connection.
func (c *clusterConn) Acquire(ctx context.Context) (kubeCertClient, func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		conn, err := c.dialer.DialCluster(ctx)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		c.conn = conn
	}
	c.holders++

	var once sync.Once // release is idempotent, like a context.CancelFunc
	release := func() {
		once.Do(func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.holders--
			if c.holders > 0 {
				return
			}
			if err := c.conn.Close(); err != nil {
				logger.WarnContext(ctx, "Failed to close cluster connection", "error", err)
			}
			c.conn = nil
		})
	}
	return c.conn, release, nil
}
