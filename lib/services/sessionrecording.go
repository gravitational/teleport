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
		return nil, trace.BadParameter(err.Error())
	}

	err = recConfig.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		recConfig.SetResourceID(cfg.ID)
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
	if err := recConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch recConfig := recConfig.(type) {
	case *types.SessionRecordingConfigV2:
		if version := recConfig.GetVersion(); version != types.V2 {
			return nil, trace.BadParameter("mismatched session recording config version %v and type %T", version, recConfig)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *recConfig
			copy.SetResourceID(0)
			copy.SetRevision("")
			recConfig = &copy
		}
		return utils.FastMarshal(recConfig)
	default:
		return nil, trace.BadParameter("unrecognized session recording config version %T", recConfig)
	}
}
