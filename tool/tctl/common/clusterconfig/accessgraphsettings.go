/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package clusterconfig

import (
	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
	"github.com/gravitational/teleport/lib/utils"
)

// AccessGraphSettings is a type to represent [clusterconfigpb.AcccessGraphSettings]
// which implements types.Resource and custom YAML (un)marshaling.
// This satisfies the expected YAML format for // the resource, which would be
// hard/impossible to do for the proto resource directly
type AccessGraphSettings struct {
	// ResourceHeader is embedded to implement types.Resource
	types.ResourceHeader
	// Spec is the specification
	Spec accessGraphSettingsSpec `json:"spec"`
}

// accessGraphSettingsSpec holds the AccessGraphSettings properties.
type accessGraphSettingsSpec struct {
	SecretsScanConfig string `json:"secrets_scan_config"`
}

// CheckAndSetDefaults sanity checks AccessGraphSettings fields to catch simple errors, and
// sets default values for all fields with defaults.
func (r *AccessGraphSettings) CheckAndSetDefaults() error {
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if r.Kind == "" {
		r.Kind = types.KindAccessGraphSettings
	} else if r.Kind != types.KindAccessGraphSettings {
		return trace.BadParameter("unexpected resource kind %q, must be %q", r.Kind, types.KindAccessGraphSettings)
	}
	if r.Version == "" {
		r.Version = types.V1
	} else if r.Version != types.V1 {
		return trace.BadParameter("unsupported resource version %q, %q is currently the only supported version", r.Version, types.V1)
	}
	if r.Metadata.Name == "" {
		r.Metadata.Name = types.MetaNameAccessGraphSettings
	} else if r.Metadata.Name != types.MetaNameAccessGraphSettings {
		return trace.BadParameter("access graph settings must have a name %q", types.MetaNameAccessGraphSettings)
	}

	if _, err := stringToSecretsScanConfig(r.Spec.SecretsScanConfig); err != nil {
		return trace.BadParameter("secrets_scan_config must be one of [enabled, disabled]")
	}

	return nil
}

// UnmarshalAccessGraphSettings parses a [*clusterconfigpb.AccessGraphSettings] in the [AccessGraphSettings]
// format which matches the expected YAML format for Teleport resources, sets default values, and
// converts to [*clusterconfigpb.AccessGraphSettings].
func UnmarshalAccessGraphSettings(raw []byte) (*clusterconfigpb.AccessGraphSettings, error) {
	var resource AccessGraphSettings
	if err := utils.FastUnmarshal(raw, &resource); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := resource.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	rec, err := resourceToProto(&resource)
	return rec, trace.Wrap(err)
}

// ProtoToResource converts a [*clusterconfigpb.AccessGraphSettings] into a [*AccessGraphSettings] which
// implements types.Resource and can be marshaled to YAML or JSON in a
// human-friendly format.
func ProtoToResource(set *clusterconfigpb.AccessGraphSettings) (*AccessGraphSettings, error) {
	conf, err := secretsScanConfigToString(set.Spec.SecretsScanConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r := &AccessGraphSettings{
		ResourceHeader: types.ResourceHeader{
			Kind:     set.Kind,
			Version:  set.Version,
			Metadata: legacy.FromHeaderMetadata(headerv1.FromMetadataProto(set.Metadata)),
		},
		Spec: accessGraphSettingsSpec{
			SecretsScanConfig: conf,
		},
	}
	return r, nil
}

func resourceToProto(r *AccessGraphSettings) (*clusterconfigpb.AccessGraphSettings, error) {
	secretsScanConfig, err := stringToSecretsScanConfig(r.Spec.SecretsScanConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clusterconfigpb.AccessGraphSettings{
		Kind:     r.Kind,
		SubKind:  r.SubKind,
		Version:  r.Version,
		Metadata: headerv1.ToMetadataProto(legacy.ToHeaderMetadata(r.Metadata)),
		Spec: &clusterconfigpb.AccessGraphSettingsSpec{
			SecretsScanConfig: secretsScanConfig,
		},
	}, nil
}

func secretsScanConfigToString(secretsScanConfig clusterconfigpb.AccessGraphSecretsScanConfig) (string, error) {
	switch secretsScanConfig {
	case clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED:
		return "disabled", nil
	case clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED:
		return "enabled", nil
	default:
		return "", trace.BadParameter("unexpected secrets scan config %q", secretsScanConfig)
	}
}

func stringToSecretsScanConfig(secretsScanConfig string) (clusterconfigpb.AccessGraphSecretsScanConfig, error) {
	switch secretsScanConfig {
	case "disabled", "off":
		return clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED, nil
	case "enabled", "on":
		return clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED, nil
	default:
		return clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_UNSPECIFIED, trace.BadParameter("secrets scan config must be one of [enabled, disabled]")
	}
}
