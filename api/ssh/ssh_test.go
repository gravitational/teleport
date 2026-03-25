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

	expected := ssh.VersionPrefix + api.Version + " mfav1"
	require.Equal(t, expected, ssh.ClientVersion())
}

func TestParseSSHClientVersion(t *testing.T) {
	t.Parallel()

	version, features, err := ssh.ParseClientVersion(ssh.VersionPrefix + "19.1.2-dev.1+meta mfav1,foo")
	require.NoError(t, err)
	require.Equal(t, []string{"mfav1", "foo"}, features)

	wantVersion, err := semver.NewVersion("19.1.2-dev.1+meta")
	require.NoError(t, err)
	require.Equal(t, wantVersion, version)
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
