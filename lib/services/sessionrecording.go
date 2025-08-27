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
	"slices"
	"strings"

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

const (
	KeyTypeAWS      = "aws_kms"
	KeyTypeGCP      = "gcp_kms"
	KeyTypePKCS11   = "pkcs11"
	KeyTypeSoftware = "software"
)

var errRecordingEncryptionWithFIPS = &trace.BadParameterError{Message: `non-FIPS compliant session recording setting: "encryption" must not be enabled`}
var errManualKeyManagementInCloud = &trace.BadParameterError{Message: `"manual_key_management" configuration is unsupported in Teleport Cloud`}

// ValidateSessionRecordingConfig checks that the state of a [SessionRecordingConfig] meets constraints.
func ValidateSessionRecordingConfig(cfg types.SessionRecordingConfig, fips, cloud bool) error {
	if !slices.Contains(types.SessionRecordingModes, cfg.GetMode()) {
		return trace.BadParameter("session recording mode must be one of %v; got %q", strings.Join(types.SessionRecordingModes, ","), cfg.GetMode())
	}

	encryptionCfg := cfg.GetEncryptionConfig()
	if encryptionCfg == nil || !encryptionCfg.Enabled {
		return nil
	}

	if fips && encryptionCfg.Enabled {
		return trace.Wrap(errRecordingEncryptionWithFIPS)
	}

	manualKeyManagement := encryptionCfg.ManualKeyManagement
	if manualKeyManagement == nil || !manualKeyManagement.Enabled {
		return nil
	}

	if cloud {
		return trace.Wrap(errManualKeyManagementInCloud)
	}

	if len(manualKeyManagement.ActiveKeys) == 0 {
		return trace.BadParameter("at least one active key must be configured when using manually managed encryption keys")
	}

	for _, label := range manualKeyManagement.ActiveKeys {
		switch strings.ToLower(label.Type) {
		case KeyTypeAWS, KeyTypeGCP, KeyTypePKCS11, KeyTypeSoftware:
		default:
			return trace.BadParameter("invalid key type %q found for active manually managed key", label.Type)
		}
	}

	for _, label := range manualKeyManagement.RotatedKeys {
		switch strings.ToLower(label.Type) {
		case KeyTypeAWS, KeyTypeGCP, KeyTypePKCS11, KeyTypeSoftware:
		default:
			return trace.BadParameter("invalid key type %q found for rotated manually managed key", label.Type)
		}
	}

	return nil
}
