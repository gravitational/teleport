/*
Copyright 2023 Gravitational, Inc.

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

package reversetunnelclient

import (
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// ClusterGetter is an interface that defines GetRemoteCluster method
type ClusterGetter interface {
	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(clusterName string) (types.RemoteCluster, error)
}

// NewTunnelWithRoles returns new authorizing tunnel
func NewTunnelWithRoles(tunnel Tunnel, localCluster string, accessChecker services.AccessChecker, access ClusterGetter) *TunnelWithRoles {
	return &TunnelWithRoles{
		tunnel:        tunnel,
		localCluster:  localCluster,
		accessChecker: accessChecker,
		access:        access,
	}
}

// TunnelWithRoles authorizes requests
type TunnelWithRoles struct {
	tunnel Tunnel

	localCluster string

	// accessChecker is used to check RBAC permissions.
	accessChecker services.AccessChecker

	access ClusterGetter
}

// GetSites returns a list of connected remote sites
func (t *TunnelWithRoles) GetSites() ([]RemoteSite, error) {
	clusters, err := t.tunnel.GetSites()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]RemoteSite, 0, len(clusters))
	for _, cluster := range clusters {
		if t.localCluster == cluster.GetName() {
			out = append(out, cluster)
			continue
		}
		rc, err := t.access.GetRemoteCluster(cluster.GetName())
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			logrus.Warningf("Skipping dangling cluster %q, no remote cluster resource found.", cluster.GetName())
			continue
		}
		if err := t.accessChecker.CheckAccessToRemoteCluster(rc); err != nil {
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
	if t.localCluster == cluster.GetName() {
		return cluster, nil
	}
	rc, err := t.access.GetRemoteCluster(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := t.accessChecker.CheckAccessToRemoteCluster(rc); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}
	return cluster, nil
}
