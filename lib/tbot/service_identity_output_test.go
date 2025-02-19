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

package tbot

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/ssh"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

const (
	// mockProxyAddr is the address of the mock proxy server, used in tests
	mockProxyAddr    = "tele.blackmesa.gov:443"
	mockProxySSHAddr = "tele.blackmesa.gov:3023"
	// mockRemoteClusterName is the remote cluster name used for the mock auth
	// client
	mockRemoteClusterName = "tele.aperture.labs"
)

type mockCertAuthorityGetter struct {
	remoteClusterName string
	clusterName       string
}

func (p *mockCertAuthorityGetter) GetCertAuthority(
	ctx context.Context, id types.CertAuthID, loadKeys bool,
) (types.CertAuthority, error) {
	if !slices.Contains([]string{p.clusterName, p.remoteClusterName}, id.DomainName) {
		return nil, trace.NotFound("specified id %q not found", id)
	}
	if loadKeys {
		return nil, trace.BadParameter("unexpected loading of key")
	}

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		// Pretend to be the correct type.
		Type:        id.Type,
		ClusterName: id.DomainName,
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert: []byte(fixtures.TLSCACertPEM),
					Key:  []byte(fixtures.TLSCAKeyPEM),
				},
			},
			SSH: []*types.SSHKeyPair{
				// Two of these to ensure that both are written to known hosts
				{
					PrivateKey: []byte(fixtures.SSHCAPrivateKey),
					PublicKey:  []byte(fixtures.SSHCAPublicKey),
				},
				{
					PrivateKey: []byte(fixtures.SSHCAPrivateKey),
					PublicKey:  []byte(fixtures.SSHCAPublicKey),
				},
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ca, nil
}

func (p *mockCertAuthorityGetter) GetCertAuthorities(ctx context.Context, caType types.CertAuthType) ([]types.CertAuthority, error) {
	// We'll just wrap GetCertAuthority()'s dummy CA.
	ca, err := p.GetCertAuthority(ctx, types.CertAuthID{
		// Just pretend to be whichever type of CA was requested.
		Type:       caType,
		DomainName: p.clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []types.CertAuthority{ca}, nil
}

type mockALPNConnTester struct {
	isALPNUpgradeRequired bool
}

func (p *mockALPNConnTester) isUpgradeRequired(ctx context.Context, addr string, insecure bool) (bool, error) {
	return p.isALPNUpgradeRequired, nil
}

func Test_renderSSHConfig(t *testing.T) {
	tests := []struct {
		Name        string
		Version     string
		Env         map[string]string
		TLSRouting  bool
		ALPNUpgrade bool
	}{
		{
			Name:       "legacy OpenSSH",
			Version:    "6.5.0",
			TLSRouting: true,
		},
		{
			Name:       "latest OpenSSH",
			Version:    "9.0.0",
			TLSRouting: true,
		},
		{
			Name:       "latest OpenSSH no tls routing",
			Version:    "9.0.0",
			TLSRouting: false,
		},
		{
			Name:        "latest OpenSSH with alpn upgrade",
			Version:     "9.0.0",
			ALPNUpgrade: true,
			TLSRouting:  true,
		},
		{
			Name:    "latest OpenSSH with legacy proxycommand",
			Version: "9.0.0",
			Env: map[string]string{
				sshConfigProxyModeEnv: "legacy",
			},
			TLSRouting: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			dir := t.TempDir()

			// identity is passed in, but not used.
			dest := &config.DestinationDirectory{
				Path:     dir,
				Symlinks: botfs.SymlinksInsecure,
				ACLs:     botfs.ACLOff,
			}

			err := renderSSHConfig(
				context.Background(),
				utils.NewSlogLoggerForTests(),
				&proxyPingResponse{
					PingResponse: &webclient.PingResponse{
						ClusterName: mockClusterName,
						Proxy: webclient.ProxySettings{
							TLSRoutingEnabled: tc.TLSRouting,
							SSH: webclient.SSHProxySettings{
								PublicAddr:    mockProxyAddr,
								SSHPublicAddr: mockProxySSHAddr,
							},
						},
					},
				},
				[]string{mockClusterName, mockRemoteClusterName},
				dest,
				&mockCertAuthorityGetter{
					remoteClusterName: mockRemoteClusterName,
					clusterName:       mockClusterName,
				},
				fakeGetExecutablePath,
				func() (*semver.Version, error) {
					return semver.New(tc.Version), nil
				},
				func(key string) string {
					if tc.Env == nil {
						return ""
					}
					return tc.Env[key]
				},
				&mockALPNConnTester{
					isALPNUpgradeRequired: tc.ALPNUpgrade,
				},
				&config.BotConfig{},
			)
			require.NoError(t, err)

			replaceTestDir := func(b []byte) []byte {
				return bytes.ReplaceAll(b, []byte(dir), []byte("/test/dir"))
			}

			knownHostBytes, err := os.ReadFile(filepath.Join(dir, ssh.KnownHostsName))
			require.NoError(t, err)
			knownHostBytes = replaceTestDir(knownHostBytes)
			sshConfigBytes, err := os.ReadFile(filepath.Join(dir, ssh.ConfigName))
			require.NoError(t, err)
			sshConfigBytes = replaceTestDir(sshConfigBytes)
			if golden.ShouldSet() {
				golden.SetNamed(t, "known_hosts", knownHostBytes)
				golden.SetNamed(t, "ssh_config", sshConfigBytes)
			}
			require.Equal(
				t, string(golden.GetNamed(t, "known_hosts")), string(knownHostBytes),
			)
			require.Equal(
				t, string(golden.GetNamed(t, "ssh_config")), string(sshConfigBytes),
			)

			// TODO(noah): In v17, we can move these assertions into the main
			// block as the legacy proxycommand mode will be removed.
			if tc.Env[sshConfigProxyModeEnv] != "legacy" {
				for clusterType, clusterName := range map[string]string{
					"local":  mockClusterName,
					"remote": mockRemoteClusterName,
				} {
					clusterKnownHostBytes, err := os.ReadFile(
						filepath.Join(dir, fmt.Sprintf("%s.%s", clusterName, ssh.KnownHostsName)),
					)
					require.NoError(t, err)
					clusterKnownHostBytes = replaceTestDir(clusterKnownHostBytes)
					clusterSSHConfigBytes, err := os.ReadFile(
						filepath.Join(dir, fmt.Sprintf("%s.%s", clusterName, ssh.ConfigName)),
					)
					require.NoError(t, err)
					clusterSSHConfigBytes = replaceTestDir(clusterSSHConfigBytes)

					configGolden := fmt.Sprintf("%s_cluster_ssh_config", clusterType)
					knownHostsGolden := fmt.Sprintf("%s_cluster_known_hosts", clusterType)
					if golden.ShouldSet() {
						golden.SetNamed(t, knownHostsGolden, clusterKnownHostBytes)
						golden.SetNamed(t, configGolden, clusterSSHConfigBytes)
					}
					require.Equal(
						t, string(golden.GetNamed(t, knownHostsGolden)), string(clusterKnownHostBytes),
					)
					require.Equal(
						t, string(golden.GetNamed(t, configGolden)), string(clusterSSHConfigBytes),
					)
				}
			}
		})
	}
}
