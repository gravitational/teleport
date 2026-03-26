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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/ssh"
	"github.com/gravitational/trace"
)

func TestClientVersionWithFeatures(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		features []string
		want     string
	}{
		{
			name: "no features",
			want: ssh.VersionPrefix + "_" + api.Version,
		},
		{
			name:     "single feature",
			features: []string{ssh.InBandMFAFeature},
			want:     ssh.VersionPrefix + "_" + api.Version + " " + ssh.InBandMFAFeature,
		},
		{
			name:     "multiple features",
			features: []string{ssh.InBandMFAFeature, "foo"},
			want:     ssh.VersionPrefix + "_" + api.Version + " " + ssh.InBandMFAFeature + ",foo",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ssh.ClientVersionWithFeatures(tt.features...))
		})
	}
}

func TestParseSSHClientVersion(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		clientVersion string
		wantVersion   string
		wantFeatures  []string
	}{
		{
			name:          "prefix only",
			clientVersion: ssh.VersionPrefix,
			wantVersion:   "",
			wantFeatures:  nil,
		},
		{
			name:          "prefix with version",
			clientVersion: ssh.VersionPrefix + "_19.1.2-dev.1+meta",
			wantVersion:   "19.1.2-dev.1+meta",
			wantFeatures:  nil,
		},
		{
			name:          "prefix with empty version after underscore",
			clientVersion: ssh.VersionPrefix + "_",
			wantVersion:   "",
			wantFeatures:  nil,
		},
		{
			name:          "prefix with features only",
			clientVersion: ssh.VersionPrefix + " " + "mfav1,foo",
			wantVersion:   "",
			wantFeatures:  []string{"mfav1", "foo"},
		},
		{
			name:          "prefix with version and features",
			clientVersion: ssh.VersionPrefix + "_19.1.2-dev.1+meta" + " " + "mfav1,foo=bar",
			wantVersion:   "19.1.2-dev.1+meta",
			wantFeatures:  []string{"mfav1", "foo=bar"},
		},
		{
			name:          "prefix with version without underscore",
			clientVersion: ssh.VersionPrefix + "19.1.2-dev.1+meta",
			wantVersion:   "19.1.2-dev.1+meta",
			wantFeatures:  nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			version, features, err := ssh.ParseClientVersion(tt.clientVersion)
			require.NoError(t, err)
			require.Equal(t, tt.wantVersion, version)
			require.Equal(t, tt.wantFeatures, features)
		})
	}
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
			name:          "invalid feature character",
			clientVersion: ssh.VersionPrefix + "_19.1.2-dev.1" + " " + "mfav1,foo	look to the left to see a tab lol",
			wantErr:       trace.BadParameter("SSH client version contain invalid characters %q", '\t'),
		},
		{
			name:          "non ascii feature character",
			clientVersion: ssh.VersionPrefix + "_19.1.2-dev.1" + " " + "mfav1,foo\xc3",
			wantErr:       trace.BadParameter("SSH client version contain invalid characters %q", 0xc3),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			version, features, err := ssh.ParseClientVersion(tt.clientVersion)
			require.ErrorIs(t, err, tt.wantErr)
			require.Empty(t, version)
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
			clientVersion: ssh.VersionPrefix + "_" + "19.1.2-dev.1+meta" + " " + "mfav1,foo",
			feature:       "mfav1",
			want:          true,
		},
		{
			name:          "unsupported feature",
			clientVersion: ssh.VersionPrefix + "_" + "19.1.2-dev.1+meta" + " " + "mfav1,foo",
			feature:       "bar",
			want:          false,
		},
		{
			name:          "no features advertised",
			clientVersion: ssh.VersionPrefix + "_" + "19.1.2-dev.1+meta",
			feature:       "mfav1",
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
