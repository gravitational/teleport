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

package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/defaults"
)

func TestClientVersionWithFeatures(t *testing.T) {
	t.Parallel()

	t.Run("no features", func(t *testing.T) {
		require.Equal(t, DefaultClientVersion, ClientVersionWithFeatures())
	})

	t.Run("with features", func(t *testing.T) {
		require.Equal(
			t,
			DefaultClientVersion+" "+InBandMFAFeature+",foov1",
			ClientVersionWithFeatures(InBandMFAFeature, "foov1"),
		)
	})
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
			clientVersion: VersionPrefix,
			wantVersion:   "",
			wantFeatures:  nil,
		},
		{
			name:          "prefix with version",
			clientVersion: VersionPrefix + "_19.1.2-dev",
			wantVersion:   "19.1.2-dev",
			wantFeatures:  nil,
		},
		{
			name:          "prefix with empty version after underscore",
			clientVersion: VersionPrefix + "_",
			wantVersion:   "",
			wantFeatures:  nil,
		},
		{
			name:          "prefix with features only",
			clientVersion: VersionPrefix + " " + "mfav1,foov1",
			wantVersion:   "",
			wantFeatures:  []string{"mfav1", "foov1"},
		},
		{
			name:          "prefix with version and features",
			clientVersion: VersionPrefix + "_19.1.2-dev" + " " + "mfav1,foov1=bar",
			wantVersion:   "19.1.2-dev",
			wantFeatures:  []string{"mfav1", "foov1=bar"},
		},
		{
			name:          "prefix with a space after the prefix but no version or features",
			clientVersion: VersionPrefix + " ",
			wantVersion:   "",
			wantFeatures:  nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			version, features, err := ParseClientVersion(tt.clientVersion)
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
			wantErr:       NonTeleportSSHVersionError{},
		},
		{
			name:          "invalid character",
			clientVersion: VersionPrefix + "_19.1.2-dev.1" + " " + "mfav1,foov1\xc3",
			wantErr: trace.BadParameter(
				"SSH client version contains invalid characters (only ASCII characters 32-126 are allowed)",
			),
		},
		{
			name:          "version without required underscore",
			clientVersion: VersionPrefix + "19.1.2-dev",
			wantErr: trace.BadParameter(
				"SSH client version must be prefixed with an underscore",
			),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			version, features, err := ParseClientVersion(tt.clientVersion)
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
			clientVersion: VersionPrefix + "_" + "19.1.2-dev" + " " + "mfav1,foov1",
			feature:       "mfav1",
			want:          true,
		},
		{
			name:          "unsupported feature",
			clientVersion: VersionPrefix + "_" + "19.1.2-dev" + " " + "mfav1,foov1",
			feature:       "bar",
			want:          false,
		},
		{
			name:          "no features advertised",
			clientVersion: VersionPrefix + "_" + "19.1.2-dev",
			feature:       "mfav1",
			want:          false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsFeatureSupported(tt.clientVersion, tt.feature)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsSSHFeatureSupportedErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		clientVersion string
		wantErr       error
	}{
		{
			name:          "invalid prefix",
			clientVersion: "SSH-2.0-OpenSSH_9.9.9",
			wantErr:       NonTeleportSSHVersionError{},
		},
		{
			name:          "version without required underscore",
			clientVersion: VersionPrefix + "19.1.2-dev",
			wantErr: trace.BadParameter(
				"SSH client version must be prefixed with an underscore",
			),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsFeatureSupported(tt.clientVersion, "mfav1")
			require.ErrorIs(t, err, tt.wantErr)
			require.False(t, got)
		})
	}
}

func TestPublicKeyAuthConfigAuthMethod(t *testing.T) {
	t.Parallel()

	signer := generateSigner(t)

	t.Run("dynamic signer", func(t *testing.T) {
		t.Parallel()

		config := PublicKeyAuthConfig{
			Signers: func() ([]ssh.Signer, error) {
				return []ssh.Signer{signer}, nil
			},
		}

		authMethod, err := config.authMethod()
		require.NoError(t, err)
		require.NotNil(t, authMethod)
	})
}

func TestPublicKeyAuthConfigAuthMethodErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		config  PublicKeyAuthConfig
		wantErr error
	}{
		{
			name:    "missing Signers callback",
			config:  PublicKeyAuthConfig{},
			wantErr: trace.BadParameter("public key auth requires Signers"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			authMethod, err := tt.config.authMethod()
			require.ErrorIs(t, err, tt.wantErr)
			require.Nil(t, authMethod)
		})
	}
}

func TestPublicKeyAuthConfigIsEmpty(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		config PublicKeyAuthConfig
		want   bool
	}{
		{
			name: "explicit nil Signers callback",
			config: PublicKeyAuthConfig{
				Signers: nil,
			},
			want: true,
		},
		{
			name: "Signers callback set",
			config: PublicKeyAuthConfig{
				Signers: func() ([]ssh.Signer, error) {
					return nil, nil
				},
			},
			want: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.config.IsEmpty())
		})
	}
}

func TestClientConfigIsEmpty(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		config ClientConfig
		want   bool
	}{
		{
			name:   "empty config",
			config: ClientConfig{},
			want:   true,
		},
		{
			name: "User set",
			config: ClientConfig{
				User: "alice",
			},
			want: false,
		},
		{
			name: "PublicKeyAuth set",
			config: ClientConfig{
				PublicKeyAuth: PublicKeyAuthConfig{
					Signers: func() ([]ssh.Signer, error) {
						return nil, nil
					},
				},
			},
			want: false,
		},
		{
			name: "HostKeyCallback set",
			config: ClientConfig{
				HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint: gosec // This is a test.
			},
			want: false,
		},
		{
			name: "BannerCallback set",
			config: ClientConfig{
				BannerCallback: func(string) error { return nil },
			},
			want: true,
		},
		{
			name: "HostKeyAlgorithms set",
			config: ClientConfig{
				HostKeyAlgorithms: []string{ssh.KeyAlgoED25519},
			},
			want: true,
		},
		{
			name: "HostKeyAlgorithms empty but allocated",
			config: ClientConfig{
				HostKeyAlgorithms: []string{},
			},
			want: true,
		},
		{
			name: "Timeout set",
			config: ClientConfig{
				Timeout: time.Second,
			},
			want: true,
		},
		{
			name: "negative timeout set",
			config: ClientConfig{
				Timeout: -time.Second,
			},
			want: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.config.IsEmpty())
		})
	}
}

func TestClientConfigSSHClientConfigSetsDefaults(t *testing.T) {
	t.Parallel()

	signer := generateSigner(t)

	cfg := ClientConfig{
		User: "alice",
		PublicKeyAuth: PublicKeyAuthConfig{
			Signers: func() ([]ssh.Signer, error) {
				return []ssh.Signer{signer}, nil
			},
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint: gosec // This is a test.
	}

	sshConfig, err := cfg.sshClientConfig()
	require.NoError(t, err)
	require.Equal(t, DefaultClientVersion, sshConfig.ClientVersion)
	require.Len(t, sshConfig.Auth, 1)
	require.Equal(t, defaults.DefaultIOTimeout, sshConfig.Timeout)
}

func TestClientConfigSSHClientConfigClonesAlgorithmsAndPreservesFields(t *testing.T) {
	t.Parallel()

	signer := generateSigner(t)

	algos := []string{ssh.KeyAlgoED25519, ssh.KeyAlgoRSA}

	bannerCallback := func(string) error { return nil }

	timeout := -time.Second

	cfg := ClientConfig{
		SSHConfig: ssh.Config{
			RekeyThreshold: 123,
		},
		User: "alice",
		PublicKeyAuth: PublicKeyAuthConfig{
			Signers: func() ([]ssh.Signer, error) {
				return []ssh.Signer{signer}, nil
			},
		},
		HostKeyCallback:   ssh.InsecureIgnoreHostKey(), //nolint: gosec // This is a test.
		BannerCallback:    bannerCallback,
		HostKeyAlgorithms: algos,
		Timeout:           timeout,
	}

	sshConfig, err := cfg.sshClientConfig()
	require.NoError(t, err)
	require.Equal(t, uint64(123), sshConfig.RekeyThreshold)
	require.Equal(t, timeout, sshConfig.Timeout)
	require.Equal(t, algos, sshConfig.HostKeyAlgorithms)
	require.NotNil(t, sshConfig.BannerCallback)
	require.NoError(t, sshConfig.BannerCallback("test banner"))

	algos[0] = "mutated"
	require.Equal(
		t,
		ssh.KeyAlgoED25519,
		sshConfig.HostKeyAlgorithms[0],
		"HostKeyAlgorithms should not affected by mutations",
	)
}

func TestClientConfigSSHClientConfigReturnsValidationErrors(t *testing.T) {
	t.Parallel()

	signer := generateSigner(t)

	for _, tt := range []struct {
		name    string
		config  func() ClientConfig
		wantErr error
	}{
		{
			name: "missing user",
			config: func() ClientConfig {
				return ClientConfig{
					PublicKeyAuth: PublicKeyAuthConfig{
						Signers: func() ([]ssh.Signer, error) {
							return []ssh.Signer{signer}, nil
						},
					},
					HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint: gosec // This is a test.
				}
			},
			wantErr: trace.BadParameter("config User must be set"),
		},
		{
			name: "missing host key callback",
			config: func() ClientConfig {
				return ClientConfig{
					User: "alice",
					PublicKeyAuth: PublicKeyAuthConfig{
						Signers: func() ([]ssh.Signer, error) {
							return []ssh.Signer{signer}, nil
						},
					},
				}
			},
			wantErr: trace.BadParameter("config HostKeyCallback must be set"),
		},
		{
			name: "missing public key auth",
			config: func() ClientConfig {
				return ClientConfig{
					User:            "alice",
					HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint: gosec // This is a test.
				}
			},
			wantErr: trace.BadParameter("public key auth requires Signers"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.config().sshClientConfig()
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestClientWrappedFuncsEarlyReturnsOnValidationErrors(t *testing.T) {
	t.Parallel()

	conn1, conn2 := net.Pipe()
	t.Cleanup(func() {
		conn1.Close()
		conn2.Close()
	})

	cfg := ClientConfig{}

	client, err := Dial(t.Context(), "tcp", "127.0.0.1:0", cfg)
	require.ErrorIs(t, err, trace.BadParameter("config User must be set"))
	require.Nil(t, client)

	client, err = NewClient(t.Context(), conn1, "127.0.0.1:0", cfg)
	require.ErrorIs(t, err, trace.BadParameter("config User must be set"))
	require.Nil(t, client)

	sshConn, chans, reqs, err := NewClientConn(t.Context(), conn1, "127.0.0.1:0", cfg)
	require.ErrorIs(t, err, trace.BadParameter("config User must be set"))
	require.Nil(t, sshConn)
	require.Nil(t, chans)
	require.Nil(t, reqs)
}

func generateSigner(t *testing.T) ssh.Signer {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	signer, err := ssh.NewSignerFromSigner(privateKey)
	require.NoError(t, err)

	return signer
}
