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

package services

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// UnmarshalRemoteCluster unmarshals the RemoteCluster resource from JSON.
func UnmarshalRemoteCluster(bytes []byte, opts ...MarshalOption) (types.RemoteCluster, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var cluster types.RemoteClusterV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	if err := utils.FastUnmarshal(bytes, &cluster); err != nil {
		return nil, trace.Wrap(err)
	}

	err = cluster.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		cluster.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		cluster.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		cluster.SetExpiry(cfg.Expires)
	}

	return &cluster, nil
}

// MarshalRemoteCluster marshals the RemoteCluster resource to JSON.
func MarshalRemoteCluster(remoteCluster types.RemoteCluster, opts ...MarshalOption) ([]byte, error) {
	if err := remoteCluster.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch remoteCluster := remoteCluster.(type) {
	case *types.RemoteClusterV3:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *remoteCluster
			copy.SetResourceID(0)
			copy.SetRevision("")
			remoteCluster = &copy
		}
		return utils.FastMarshal(remoteCluster)
	default:
		return nil, trace.BadParameter("unrecognized remote cluster version %T", remoteCluster)
	}
}
