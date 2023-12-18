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
	if cfg.Revision != "" {
		si.SetRevision(cfg.Revision)
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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch si := si.(type) {
	case *types.ServerInfoV1:
		if err := si.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		bytes, err := utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, si))
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
	for i := range serverInfos {
		out[i] = types.ServerInfo(&serverInfos[i])
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
