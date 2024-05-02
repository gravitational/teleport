// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package common

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/client"
)

// vnetAppProvider implement [vnet.AppProvider] in order to provide the necessary methods to log in to apps
// and get clients able to list apps in all clusters in all current profiles.
type vnetAppProvider struct {
	clientStore *client.Store
}

func newVnetAppProvider(cf *CLIConf) (*vnetAppProvider, error) {
	clientStore := client.NewFSClientStore(cf.HomePath)

	return &vnetAppProvider{
		clientStore: clientStore,
	}, nil
}

// ListProfiles lists the names of all profiles saved for the user.
func (p *vnetAppProvider) ListProfiles() ([]string, error) {
	return p.clientStore.ListProfiles()
}

// GetProfile returns the named profile for the user.
func (p *vnetAppProvider) GetProfile(profileName string) (*profile.Profile, error) {
	return p.clientStore.GetProfile(profileName)
}

// GetCachedClient returns a [*client.ClusterClient] for the given profile and leaf cluster.
// [leafClusterName] may be empty when requesting a client for the root cluster.
// TODO: cache clients across calls.
func (p *vnetAppProvider) GetCachedClient(ctx context.Context, profileName, leafClusterName string) (*client.ClusterClient, error) {
	cfg := &client.Config{
		ClientStore: p.clientStore,
	}
	if err := cfg.LoadProfile(p.clientStore, profileName); err != nil {
		return nil, trace.Wrap(err, "loading client profile")
	}
	if leafClusterName != "" {
		cfg.SiteName = leafClusterName
	}
	tc, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err, "creating new client")
	}

	clusterClient, err := tc.ConnectToCluster(ctx)
	return clusterClient, trace.Wrap(err)
}
