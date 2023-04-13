/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
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

// mockAuth is a minimal fake auth client, used in tests
type mockAuth struct {
	auth.ClientI

	clusterName       string
	remoteClusterName string
	proxyAddr         string
	t                 *testing.T
}

func (m *mockAuth) GetDomainName(ctx context.Context) (string, error) {
	return m.clusterName, nil
}

func (m *mockAuth) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	cn, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: m.clusterName,
		ClusterID:   "aa-bb-cc",
	})
	require.NoError(m.t, err)
	return cn, nil
}

func (m *mockAuth) GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error) {
	rc, err := types.NewRemoteCluster(m.remoteClusterName)
	require.NoError(m.t, err)
	return []types.RemoteCluster{rc}, nil
}

func (m *mockAuth) Ping(ctx context.Context) (proto.PingResponse, error) {
	require.NotNil(m.t, ctx)
	return proto.PingResponse{
		ProxyPublicAddr: m.proxyAddr,
	}, nil
}

func (m *mockAuth) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error) {
	require.NotNil(m.t, ctx)
	require.Contains(
		m.t,
		[]string{m.clusterName, m.remoteClusterName},
		id.DomainName,
	)
	require.False(m.t, loadKeys)

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
	require.NoError(m.t, err)
	return ca, nil
}

func (m *mockAuth) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error) {
	require.NotNil(m.t, ctx)
	require.False(m.t, loadKeys)

	// We'll just wrap GetCertAuthority()'s dummy CA.
	ca, err := m.GetCertAuthority(ctx, types.CertAuthID{
		// Just pretend to be whichever type of CA was requested.
		Type:       caType,
		DomainName: m.clusterName,
	}, loadKeys, opts...)
	require.NoError(m.t, err)

	return []types.CertAuthority{ca}, nil
}

func newMockAuth(t *testing.T) *mockAuth {
	return &mockAuth{
		t:                 t,
		clusterName:       mockClusterName,
		proxyAddr:         mockProxyAddr,
		remoteClusterName: mockRemoteClusterName,
	}
}

func (m *mockAuth) Close() error {
	return nil
}

// mockBot is a minimal Bot impl that can be used in tests
type mockBot struct {
	cfg  *BotConfig
	auth auth.ClientI
}

func (b *mockBot) AuthPing(ctx context.Context) (*proto.PingResponse, error) {
	ping, err := b.auth.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return &ping, err
}

func (b *mockBot) ProxyPing(ctx context.Context) (*webclient.PingResponse, error) {
	return &webclient.PingResponse{}, nil
}

func (b *mockBot) GetCertAuthorities(ctx context.Context, caType types.CertAuthType) ([]types.CertAuthority, error) {
	return b.auth.GetCertAuthorities(ctx, caType, false)
}

func (b *mockBot) AuthenticatedUserClientFromIdentity(ctx context.Context, id *identity.Identity) (auth.ClientI, error) {
	return b.auth, nil
}

func (b *mockBot) Config() *BotConfig {
	return b.cfg
}

func newMockBot(cfg *BotConfig, auth auth.ClientI) *mockBot {
	return &mockBot{
		cfg:  cfg,
		auth: auth,
	}
}

// identRequest is a function used to add additional requests to an identity in
// getTestIdent.
type identRequest func(id *tlsca.Identity)

// kubernetesRequest requests a Kubernetes cluster.
func kubernetesRequest(k8sCluster string) identRequest {
	return func(id *tlsca.Identity) {
		id.KubernetesCluster = k8sCluster
	}
}

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
	}, certs, identity.DestinationKinds()...)
	require.NoError(t, err)

	return ident
}
