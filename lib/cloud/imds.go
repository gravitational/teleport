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

package cloud

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/imds"
)

// discoverInstanceMetadataTimeout is the maximum amount of time allowed
// to discover an instance metadata service. The timeout is short to
// minimize Teleport's startup time when it isn't running on any cloud
// instance. Checking for instance metadata typically takes less than 30ms.
const discoverInstanceMetadataTimeout = 500 * time.Millisecond

// DiscoverInstanceMetadata checks which cloud instance type Teleport is
// running on, if any.
func DiscoverInstanceMetadata(ctx context.Context, providers []func(ctx context.Context) (imds.Client, error)) (imds.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, discoverInstanceMetadataTimeout)
	defer cancel()

	c := make(chan imds.Client)
	clients := make([]imds.Client, 0, len(providers))
	for _, constructor := range providers {
		im, err := constructor(ctx)
		if err == nil {
			clients = append(clients, im)
		}
	}

	for _, client := range clients {
		go func() {
			if client.IsAvailable(ctx) {
				c <- client
			}
		}()
	}

	select {
	case client := <-c:
		return client, nil
	case <-ctx.Done():
		return nil, trace.NotFound("no instance metadata service found")
	}
}
