// Copyright 2026 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh_test

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/ssh"
)

func TestClientVersion(t *testing.T) {
	t.Parallel()

	expected := ssh.VersionPrefix + api.Version
	require.Equal(t, expected, ssh.DefaultClientVersion)
}

func TestClientVersionWithFeatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		features []string
		want     string
	}{
		{
			name: "no features",
			want: ssh.VersionPrefix + api.Version,
		},
		{
			name:     "single feature",
			features: []string{ssh.InBandMFAFeature},
			want:     ssh.VersionPrefix + api.Version + " " + ssh.InBandMFAFeature,
		},
		{
			name:     "multiple features",
			features: []string{ssh.InBandMFAFeature, "foo"},
			want:     ssh.VersionPrefix + api.Version + " " + ssh.InBandMFAFeature + ",foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ssh.ClientVersionWithFeatures(tt.features...))
		})
	}
}

func TestParseSSHClientVersion(t *testing.T) {
	t.Parallel()

	wantVersion, err := semver.NewVersion("19.1.2-dev.1+meta")
	require.NoError(t, err)

	t.Run("with features", func(t *testing.T) {
		version, features, err := ssh.ParseClientVersion(ssh.VersionPrefix + "19.1.2-dev.1+meta mfav1,foo")
		require.NoError(t, err)
		require.Equal(t, []string{"mfav1", "foo"}, features)
		require.Equal(t, wantVersion, version)
	})

	t.Run("without features", func(t *testing.T) {
		version, features, err := ssh.ParseClientVersion(ssh.VersionPrefix + "19.1.2-dev.1+meta")
		require.NoError(t, err)
		require.Nil(t, features)
		require.Equal(t, wantVersion, version)
	})
}

func TestParseSSHClientVersionErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		clientVersion string
		wantErr       error
	}{
		{
			name:          "invalid prefix",
			clientVersion: "SSH-2.0-OpenSSH_9.9.9",
			wantErr:       ssh.NonTeleportSSHVersionError{},
		},
		{
			name:          "missing version",
			clientVersion: "SSH-2.0-Teleport_",
			wantErr: trace.BadParameter(
				"invalid version %q: missing Teleport version",
				"SSH-2.0-Teleport_",
			),
		},
		{
			name:          "unexpected whitespace",
			clientVersion: "SSH-2.0-Teleport_19.1.2  mfav1",
			wantErr: trace.BadParameter(
				"invalid version %q: unexpected whitespace",
				"SSH-2.0-Teleport_19.1.2  mfav1",
			),
		},
		{
			name:          "too many fields",
			clientVersion: "SSH-2.0-Teleport_19.1.2 mfav1 extra",
			wantErr: trace.BadParameter(
				"invalid version %q: expected \"<version>\" or \"<version> <feature[,feature...]>\"",
				"SSH-2.0-Teleport_19.1.2 mfav1 extra",
			),
		},
		{
			name:          "invalid version",
			clientVersion: "SSH-2.0-Teleport_not-a-semantic-version mfav1",
			wantErr: func() error {
				_, err := semver.NewVersion("not-a-semantic-version")
				require.Error(t, err)

				return trace.BadParameter(
					"invalid version %q: invalid semantic version %q: %v",
					"SSH-2.0-Teleport_not-a-semantic-version mfav1",
					"not-a-semantic-version",
					err,
				)
			}(),
		},
		{
			name:          "empty feature name",
			clientVersion: "SSH-2.0-Teleport_19.1.2 mfav1,",
			wantErr: trace.BadParameter(
				"invalid version %q: empty feature name in %q",
				"SSH-2.0-Teleport_19.1.2 mfav1,",
				"mfav1,",
			),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			version, features, err := ssh.ParseClientVersion(tt.clientVersion)
			require.ErrorIs(t, err, tt.wantErr)
			require.Nil(t, version)
			require.Nil(t, features)
		})
	}
}

func TestIsSSHFeatureSupported(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		clientVersion string
		feature       string
		want          bool
	}{
		{
			name:          "supported feature",
			clientVersion: ssh.VersionPrefix + "19.1.2-dev.1+meta mfav1,foo",
			feature:       "mfav1",
			want:          true,
		},
		{
			name:          "unsupported feature",
			clientVersion: ssh.VersionPrefix + "19.1.2-dev.1+meta mfav1,foo",
			feature:       "bar",
			want:          false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ssh.IsFeatureSupported(tt.clientVersion, tt.feature)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsSSHFeatureSupportedErrors(t *testing.T) {
	t.Parallel()

	got, err := ssh.IsFeatureSupported("SSH-2.0-OpenSSH_9.9.9", "mfav1")
	require.ErrorIs(t, err, ssh.NonTeleportSSHVersionError{})
	require.False(t, got)
}
