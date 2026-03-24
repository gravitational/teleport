// Copyright 2025 Gravitational, Inc.
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

package api

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
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

	v, err := semver.NewVersion(Version)
	require.NoError(t, err)
	require.Equal(t, SemVer(), v)
}

func TestSSHClientVersion(t *testing.T) {
	t.Parallel()

	expected := "SSH-2.0-Teleport_" + Version + " mfav1"
	require.Equal(t, expected, SSHClientVersion())
}

func TestParseSSHClientVersion(t *testing.T) {
	t.Parallel()

	version, features, err := ParseSSHClientVersion("SSH-2.0-Teleport_19.1.2-dev.1+meta mfav1,foo")
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
				sshVersionPrefix,
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
			version, features, err := ParseSSHClientVersion(tt.clientVersion)
			require.ErrorIs(t, err, tt.wantErr)
			require.Nil(t, version)
			require.Nil(t, features)
		})
	}
}
