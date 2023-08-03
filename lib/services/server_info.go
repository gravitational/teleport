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

package services

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils"
)

// UnmarshalServerInfo unmarshals the ServerInfo resource from JSON.
func UnmarshalServerInfo(bytes []byte, opts ...MarshalOption) (types.ServerInfo, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing server info data")
	}

	var si types.ServerInfoV1
	if err := utils.FastUnmarshal(bytes, &si); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := si.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		si.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		si.SetExpiry(cfg.Expires)
	}
	if si.Metadata.Expires != nil {
		apiutils.UTC(si.Metadata.Expires)
	}

	return &si, nil
}

// MarshalServerInfo marshals the ServerInfo resource to JSON.
func MarshalServerInfo(si types.ServerInfo, opts ...MarshalOption) ([]byte, error) {
	if err := si.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch si := si.(type) {
	case *types.ServerInfoV1:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *si
			copy.SetResourceID(0)
			si = &copy
		}
		bytes, err := utils.FastMarshal(si)
		return bytes, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unrecognized server info version %T", si)
	}
}

// UnmarshalServerInfos unmarshals a list of ServerInfo resources.
func UnmarshalServerInfos(bytes []byte) ([]types.ServerInfo, error) {
	var serverInfos []types.ServerInfoV1

	err := utils.FastUnmarshal(bytes, &serverInfos)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]types.ServerInfo, len(serverInfos))
	for i, v := range serverInfos {
		out[i] = types.ServerInfo(&v)
	}

	return out, nil
}

// MarshalServerInfos marshals a list of ServerInfo resources.
func MarshalServerInfos(si []types.ServerInfo) ([]byte, error) {
	bytes, err := utils.FastMarshal(si)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return bytes, nil
}
