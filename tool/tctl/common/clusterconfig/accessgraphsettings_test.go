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

package clusterconfig

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

func TestUnmarshalAccessGraphSettings(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc          string
		input         string
		errorContains string
		expected      *clusterconfigpb.AccessGraphSettings
	}{
		{
			desc: "disabled",
			input: `---
kind: access_graph_settings
version: v1
metadata:
  name: access-graph-settings
spec:
  secrets_scan_config: "disabled"
`,
			expected: &clusterconfigpb.AccessGraphSettings{
				Version: types.V1,
				Kind:    types.KindAccessGraphSettings,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAccessGraphSettings,
				},
				Spec: &clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
				},
			},
		},
		{
			desc: "off",
			input: `---
kind: access_graph_settings
version: v1
metadata:
  name: access-graph-settings
spec:
  secrets_scan_config: "off"
`,
			expected: &clusterconfigpb.AccessGraphSettings{
				Version: types.V1,
				Kind:    types.KindAccessGraphSettings,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAccessGraphSettings,
				},
				Spec: &clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
				},
			},
		},
		{
			desc: "enabled",
			input: `---
kind: access_graph_settings
version: v1
metadata:
  name: access-graph-settings
spec:
  secrets_scan_config: "enabled"
`,
			expected: &clusterconfigpb.AccessGraphSettings{
				Version: types.V1,
				Kind:    types.KindAccessGraphSettings,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAccessGraphSettings,
				},
				Spec: &clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED,
				},
			},
		},
		{
			desc: "on",
			input: `---
kind: access_graph_settings
version: v1
metadata:
  name: access-graph-settings
spec:
  secrets_scan_config: "on"
`,
			expected: &clusterconfigpb.AccessGraphSettings{
				Version: types.V1,
				Kind:    types.KindAccessGraphSettings,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAccessGraphSettings,
				},
				Spec: &clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED,
				},
			},
		},
		{
			desc: "invalid settings",
			input: `---
kind: access_graph_settings
version: v1
metadata:
  name: access-graph-settings
spec:
  secrets_scan_config: "invalidasd"
`,
			errorContains: "secrets_scan_config must be one of [enabled, disabled]",
		},
		{
			desc: "wrong name",
			input: `---
kind: access_graph_settings
version: v1
metadata:
  name: access
spec:
  secrets_scan_config: "on"
`,
			errorContains: "access graph settings must have a name \"access-graph-settings\"",
		},
		{
			desc: "wrong version",
			input: `---
kind: access_graph_settings
version: v2
metadata:
  name: access-graph-settings
spec:
  secrets_scan_config: "on"
`,
			errorContains: "unsupported resource version",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Mimic tctl resource command by using the same decoder and
			// initially unmarshalling into services.UnknownResource
			reader := strings.NewReader(tc.input)
			decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)
			var raw services.UnknownResource
			err := decoder.Decode(&raw)
			require.NoError(t, err)
			require.Equal(t, types.KindAccessGraphSettings, raw.Kind)

			out, err := UnmarshalAccessGraphSettings(raw.Raw)
			if tc.errorContains != "" {
				require.ErrorContains(t, err, tc.errorContains, "error from UnmarshalAccessGraphSettings does not contain the expected string")
				return
			}
			require.NoError(t, err, "UnmarshalAccessGraphSettings returned unexpected error")

			require.Empty(t, cmp.Diff(tc.expected, out, protocmp.Transform()), "unmarshalled data does not match what was expected")
		})
	}
}

func TestProtoToResource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc          string
		expected      *AccessGraphSettings
		errorContains string
		input         *clusterconfigpb.AccessGraphSettings
	}{
		{
			desc: "disabled",
			expected: &AccessGraphSettings{
				ResourceHeader: types.ResourceHeader{
					Kind:     types.KindAccessGraphSettings,
					Version:  types.V1,
					Metadata: types.Metadata{Name: types.MetaNameAccessGraphSettings},
				},
				Spec: accessGraphSettingsSpec{
					SecretsScanConfig: "disabled",
				},
			},
			input: &clusterconfigpb.AccessGraphSettings{
				Version: types.V1,
				Kind:    types.KindAccessGraphSettings,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAccessGraphSettings,
				},
				Spec: &clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
				},
			},
		},
		{
			desc: "enabled",
			expected: &AccessGraphSettings{
				ResourceHeader: types.ResourceHeader{
					Kind:     types.KindAccessGraphSettings,
					Version:  types.V1,
					Metadata: types.Metadata{Name: types.MetaNameAccessGraphSettings},
				},
				Spec: accessGraphSettingsSpec{
					SecretsScanConfig: "enabled",
				},
			},
			input: &clusterconfigpb.AccessGraphSettings{
				Version: types.V1,
				Kind:    types.KindAccessGraphSettings,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAccessGraphSettings,
				},
				Spec: &clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED,
				},
			},
		},
		{
			desc:          "incorrect data",
			errorContains: "unexpected secrets scan config",
			input: &clusterconfigpb.AccessGraphSettings{
				Version: types.V1,
				Kind:    types.KindAccessGraphSettings,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAccessGraphSettings,
				},
				Spec: &clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: 5,
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {

			out, err := ProtoToResource(tc.input)
			if tc.errorContains != "" {
				require.ErrorContains(t, err, tc.errorContains, "error from ProtoToResource does not contain the expected string")
				return
			}
			require.NoError(t, err, "ProtoToResource returned unexpected error")

			require.Empty(t, cmp.Diff(tc.expected, out, protocmp.Transform()))
		})
	}
}
