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
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tlsca"
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
	cfg               *BotConfig
	proxyAddr         string
	remoteClusterName string
	clusterName       string
}

func newMockProvider(cfg *BotConfig) *mockProvider {
	return &mockProvider{
		cfg:               cfg,
		proxyAddr:         mockProxyAddr,
		clusterName:       mockClusterName,
		remoteClusterName: mockRemoteClusterName,
	}
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

func (p *mockProvider) AuthPing(_ context.Context) (*proto.PingResponse, error) {
	return &proto.PingResponse{
		ProxyPublicAddr: p.proxyAddr,
		ClusterName:     p.clusterName,
	}, nil
}

func (p *mockProvider) SignX509SVIDs(
	ctx context.Context,
	in *machineidv1pb.SignX509SVIDsRequest,
	opts ...grpc.CallOption,
) (*machineidv1pb.SignX509SVIDsResponse, error) {
	return nil, nil
}

func (p *mockProvider) GenerateHostCert(
	ctx context.Context, req *trustpb.GenerateHostCertRequest,
) (*trustpb.GenerateHostCertResponse, error) {
	// We could generate a cert easily enough here, but the template generates a
	// random key each run so the resulting cert will change too.
	// The CA fixture isn't even a cert but we never examine it, so it'll do the
	// job.
	return &trustpb.GenerateHostCertResponse{SshCertificate: []byte(fixtures.SSHCAPublicKey)}, nil
}

func (p *mockProvider) ProxyPing(ctx context.Context) (*webclient.PingResponse, error) {
	return &webclient.PingResponse{
		ClusterName: p.clusterName,
		Proxy: webclient.ProxySettings{
			TLSRoutingEnabled: true,
			SSH: webclient.SSHProxySettings{
				PublicAddr: p.proxyAddr,
			},
		},
	}, nil
}

func (p *mockProvider) Config() *BotConfig {
	return p.cfg
}

// identRequest is a function used to add additional requests to an identity in
// getTestIdent.
type identRequest func(id *tlsca.Identity)

// getTestIdent returns a mostly-valid bot Identity without starting up an
// entire Teleport server instance.
func getTestIdent(t *testing.T, username string, reqs ...identRequest) *identity.Identity {
	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	sshPrivateKey, err := ssh.ParseRawPrivateKey(privateKey)
	require.NoError(t, err)

	tlsPublicKeyPEM, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivateKey)
	require.NoError(t, err)

	tlsPublicKey, err := tlsca.ParsePublicKeyPEM(tlsPublicKeyPEM)
	require.NoError(t, err)

	// Note: it'd be nice to make this more universally useful in our tests at
	// some point.
	clock := clockwork.NewFakeClock()
	notAfter := clock.Now().Add(time.Hour)
	id := tlsca.Identity{
		Username:         username,
		KubernetesUsers:  []string{"foo"},
		KubernetesGroups: []string{"bar"},
		RouteToCluster:   mockClusterName,
	}
	for _, req := range reqs {
		req(&id)
	}
	subject, err := id.Subject()
	require.NoError(t, err)
	certBytes, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: tlsPublicKey,
		Subject:   subject,
		NotAfter:  notAfter,
	})
	require.NoError(t, err)

	caSigner, err := ssh.ParsePrivateKey([]byte(fixtures.SSHCAPrivateKey))
	require.NoError(t, err)
	ta := testauthority.New()
	sshCertBytes, err := ta.GenerateUserCert(services.UserCertParams{
		CASigner:          caSigner,
		PublicUserKey:     sshPublicKey,
		Username:          username,
		CertificateFormat: constants.CertificateFormatStandard,
		TTL:               time.Minute,
		AllowedLogins:     []string{"foo"},
		RouteToCluster:    mockClusterName,
	})

	require.NoError(t, err)

	certs := &proto.Certs{
		SSH:        sshCertBytes,
		TLS:        certBytes,
		TLSCACerts: [][]byte{[]byte(fixtures.TLSCACertPEM)},
		SSHCACerts: [][]byte{[]byte(fixtures.SSHCAPublicKey)},
	}

	ident, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: privateKey,
		PublicKeyBytes:  tlsPublicKeyPEM,
	}, certs)
	require.NoError(t, err)

	return ident
}
