/*
Copyright 2021 Gravitational, Inc.

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

package clusters

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Cluster describes user settings and access to various resources.
type Cluster struct {
	// URI is the cluster URI
	URI uri.ResourceURI
	// Name is the cluster name
	Name string

	// Log is a component logger
	Log logrus.FieldLogger
	// dir is the directory where cluster certificates are stored
	dir string
	// Status is the cluster status
	status client.ProfileStatus
	// client is the cluster Teleport client
	clusterClient *client.TeleportClient
	// clock is a clock for time-related operations
	clock clockwork.Clock
}

// Connected indicates if connection to the cluster can be established
func (c *Cluster) Connected() bool {
	return c.status.Name != "" && !c.status.IsExpired(c.clock)
}

// GetRoles returns currently logged-in user roles
func (c *Cluster) GetRoles(ctx context.Context) ([]*types.Role, error) {
	proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	roles := []*types.Role{}
	for _, name := range c.status.Roles {
		role, err := proxyClient.GetRole(ctx, name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, &role)
	}

	return roles, nil
}

// GetLoggedInUser returns currently logged-in user
func (c *Cluster) GetLoggedInUser() LoggedInUser {
	return LoggedInUser{
		Name:      c.status.Username,
		SSHLogins: c.status.Logins,
		Roles:     c.status.Roles,
	}
}

// LoggedInUser is the currently logged-in user
type LoggedInUser struct {
	// Name is the user name
	Name string
	// SSHLogins is the user sshlogins
	SSHLogins []string
	// Roles is the user roles
	Roles []string
}
