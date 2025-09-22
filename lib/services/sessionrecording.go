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
	"github.com/gravitational/teleport/lib/utils"
)

// IsRecordAtProxy returns true if recording is sync or async at proxy.
func IsRecordAtProxy(mode string) bool {
	return mode == types.RecordAtProxy || mode == types.RecordAtProxySync
}

// IsRecordSync returns true if recording is sync for proxy or node.
func IsRecordSync(mode string) bool {
	return mode == types.RecordAtProxySync || mode == types.RecordAtNodeSync
}

// UnmarshalSessionRecordingConfig unmarshals the SessionRecordingConfig resource from JSON.
func UnmarshalSessionRecordingConfig(bytes []byte, opts ...MarshalOption) (types.SessionRecordingConfig, error) {
	var recConfig types.SessionRecordingConfigV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := utils.FastUnmarshal(bytes, &recConfig); err != nil {
		return nil, trace.BadParameter("%s", err)
	}

	err = recConfig.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Revision != "" {
		recConfig.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		recConfig.SetExpiry(cfg.Expires)
	}
	return &recConfig, nil
}

// MarshalSessionRecordingConfig marshals the SessionRecordingConfig resource to JSON.
func MarshalSessionRecordingConfig(recConfig types.SessionRecordingConfig, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch recConfig := recConfig.(type) {
	case *types.SessionRecordingConfigV2:
		if err := recConfig.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		if version := recConfig.GetVersion(); version != types.V2 {
			return nil, trace.BadParameter("mismatched session recording config version %v and type %T", version, recConfig)
		}
		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, recConfig))
	default:
		return nil, trace.BadParameter("unrecognized session recording config version %T", recConfig)
	}
}
