/*
Copyright 2020 Gravitational, Inc.

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

package reversetunnel

import (
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewTunnelWithRoles returns new authorizing tunnel
func NewTunnelWithRoles(tunnel Tunnel, roles services.RoleSet, ap auth.AccessPoint) *TunnelWithRoles {
	return &TunnelWithRoles{
		tunnel: tunnel,
		roles:  roles,
		ap:     ap,
	}
}

// TunnelWithRoles authorizes requests
type TunnelWithRoles struct {
	tunnel Tunnel

	// roles is a set of roles used to check RBAC permissions.
	roles services.RoleSet

	ap auth.AccessPoint
}

// GetSites returns a list of connected remote sites
func (t *TunnelWithRoles) GetSites() ([]RemoteSite, error) {
	clusters, err := t.tunnel.GetSites()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]RemoteSite, 0, len(clusters))
	for _, cluster := range clusters {
		if _, ok := cluster.(*localSite); ok {
			out = append(out, cluster)
			continue
		}
		rc, err := t.ap.GetRemoteCluster(cluster.GetName())
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			logrus.Warningf("Skipping dangling cluster %q, no remote cluster resource found.", cluster.GetName())
			continue
		}
		if err := t.roles.CheckAccessToRemoteCluster(rc); err != nil {
			if !trace.IsAccessDenied(err) {
				return nil, trace.Wrap(err)
			}
			continue
		}
		out = append(out, cluster)
	}
	return out, nil
}

// GetSite returns remote site this node belongs to
func (t *TunnelWithRoles) GetSite(clusterName string) (RemoteSite, error) {
	cluster, err := t.tunnel.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, ok := cluster.(*localSite); ok {
		return cluster, nil
	}
	rc, err := t.ap.GetRemoteCluster(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := t.roles.CheckAccessToRemoteCluster(rc); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}
	return cluster, nil
}
