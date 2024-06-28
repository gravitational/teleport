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

package config

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
)

const (
	// mockProxyAddr is the address of the mock proxy server, used in tests
	mockProxyAddr = "tele.blackmesa.gov:443"

	// mockClusterName is the cluster name for the mock auth client, used in
	// tests
	mockClusterName = "tele.blackmesa.gov"
	// mockRemoteClusterName is the remote cluster name used for the mock auth
	// client
	mockRemoteClusterName = "tele.aperture.labs"
)

// fakeGetExecutablePath can be injected into outputs to ensure they output the
// same path in tests across multiple systems.
func fakeGetExecutablePath() (string, error) {
	return "/path/to/tbot", nil
}

// mockProvider is a minimal Bot impl that can be used in tests
type mockProvider struct {
	cfg                   *BotConfig
	proxyAddr             string
	remoteClusterName     string
	clusterName           string
	isTLSRouting          bool
	isALPNUpgradeRequired bool
}

func newMockProvider(cfg *BotConfig) *mockProvider {
	return &mockProvider{
		cfg:               cfg,
		proxyAddr:         mockProxyAddr,
		clusterName:       mockClusterName,
		remoteClusterName: mockRemoteClusterName,
		isTLSRouting:      true,
	}
}

func (p *mockProvider) IsALPNConnUpgradeRequired(
	ctx context.Context, addr string, insecure bool,
) (bool, error) {
	return p.isALPNUpgradeRequired, nil
}

func (p *mockProvider) GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error) {
	rc, err := types.NewRemoteCluster(p.remoteClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []types.RemoteCluster{rc}, nil
}

func (p *mockProvider) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
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

func (p *mockProvider) GetCertAuthorities(ctx context.Context, caType types.CertAuthType) ([]types.CertAuthority, error) {
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

func (p *mockProvider) ProxyPing(ctx context.Context) (*webclient.PingResponse, error) {
	return &webclient.PingResponse{
		ClusterName: p.clusterName,
		Proxy: webclient.ProxySettings{
			TLSRoutingEnabled: p.isTLSRouting,
			SSH: webclient.SSHProxySettings{
				PublicAddr: p.proxyAddr,
			},
		},
	}, nil
}

func (p *mockProvider) Config() *BotConfig {
	return p.cfg
}
