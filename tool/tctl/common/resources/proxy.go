// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

func proxyHandler() Handler {
	return Handler{
		getHandler:    getProxy,
		deleteHandler: deleteProxy,
		singleton:     false,
		mfaRequired:   false,
		description:   "The Proxy is responsible for routing connections within the cluster.",
	}
}

func getProxy(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	servers, err := client.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return &ServerCollection{servers: servers}, nil
	}
	for _, server := range servers {
		if server.GetName() == ref.Name || server.GetHostname() == ref.Name {
			return &ServerCollection{servers: []types.Server{server}}, nil
		}
	}
	return nil, trace.NotFound("proxy with ID %q not found", ref.Name)
}

func deleteProxy(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteProxy(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Proxy %q has been deleted\n", ref.Name)
	return nil
}
