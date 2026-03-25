// Copyright 2026 Gravitational, Inc.
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

package api_test

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api"
)

// this is a static assertion that the shape of semver.Version hasn't changed
// under our nose, since we can't inspect unexported fields in our equality
// check and we'd potentially miss out on a discrepancy between
// semver.New(Version) and our manually assembled semver.Version
var _ semver.Version = struct {
	Major      int64
	Minor      int64
	Patch      int64
	PreRelease semver.PreRelease
	Metadata   string
}{}

func TestSemVersion(t *testing.T) {
	t.Parallel()

	v, err := semver.NewVersion(api.Version)
	require.NoError(t, err)
	require.Equal(t, api.SemVer(), v)
}

func TestSSHClientVersion(t *testing.T) {
	t.Parallel()

	expected := api.SSHVersionPrefix + api.Version + " mfav1"
	require.Equal(t, expected, api.SSHClientVersion())
}

func TestParseSSHClientVersion(t *testing.T) {
	t.Parallel()

	version, features, err := api.ParseSSHClientVersion(api.SSHVersionPrefix + "19.1.2-dev.1+meta mfav1,foo")
	require.NoError(t, err)
	require.Equal(t, []string{"mfav1", "foo"}, features)

	wantVersion, err := semver.NewVersion("19.1.2-dev.1+meta")
	require.NoError(t, err)
	require.Equal(t, wantVersion, version)
}

func TestParseSSHClientVersionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		clientVersion string
		wantErr       error
	}{
		{
			name:          "invalid prefix",
			clientVersion: "SSH-2.0-OpenSSH_9.9.9",
			wantErr: trace.BadParameter(
				"invalid version %q: expected %q prefix",
				"SSH-2.0-OpenSSH_9.9.9",
				api.SSHVersionPrefix,
			),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, features, err := api.ParseSSHClientVersion(tt.clientVersion)
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
			clientVersion: api.SSHVersionPrefix + "19.1.2-dev.1+meta mfav1,foo",
			feature:       "mfav1",
			want:          true,
		},
		{
			name:          "unsupported feature",
			clientVersion: api.SSHVersionPrefix + "19.1.2-dev.1+meta mfav1,foo",
			feature:       "bar",
			want:          false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := api.IsSSHFeatureSupported(tt.clientVersion, tt.feature)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsSSHFeatureSupportedErrors(t *testing.T) {
	t.Parallel()

	got, err := api.IsSSHFeatureSupported("SSH-2.0-OpenSSH_9.9.9", "mfav1")
	require.ErrorIs(
		t,
		err,
		trace.BadParameter(
			"invalid version %q: expected %q prefix",
			"SSH-2.0-OpenSSH_9.9.9",
			api.SSHVersionPrefix,
		),
	)
	require.False(t, got)
}
