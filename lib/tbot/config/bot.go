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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// Bot is an interface covering various public tbot.Bot methods to circumvent
// import cycle issues.
type Bot interface {
	// AuthPing pings the auth server and returns the (possibly cached) response.
	AuthPing(ctx context.Context) (*proto.PingResponse, error)

	// ProxyPing returns a (possibly cached) ping response from the Teleport proxy.
	// Note that it relies on the auth server being configured with a sane proxy
	// public address.
	ProxyPing(ctx context.Context) (*webclient.PingResponse, error)

	// GetCertAuthorities returns the possibly cached CAs of the given type and
	// requests them from the server if unavailable.
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType) ([]types.CertAuthority, error)

	// Client retrieves the current auth client.
	Client() auth.ClientI

	// AuthenticatedUserClientFromIdentity returns a client backed by a specific
	// identity.
	AuthenticatedUserClientFromIdentity(ctx context.Context, id *identity.Identity, authServer string) (auth.ClientI, error)

	// Config returns the current bot config
	Config() *BotConfig
}

const (
	// mockProxyAddr is the address of the mock proxy server, used in tests
	mockProxyAddr = "tele.blackmesa.gov:443"

	// mockClusterName is the cluster name for the mock auth client, used in
	// tests
	mockClusterName = "tele.blackmesa.gov"
)

// mockAuth is a minimal fake auth client, used in tests
type mockAuth struct {
	auth.ClientI

	clusterName string
	proxyAddr   string
	t           *testing.T
}

func (m *mockAuth) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	cn, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: m.clusterName,
		ClusterID:   "aa-bb-cc",
	})
	require.NoError(m.t, err)
	return cn, nil
}

func (m *mockAuth) Ping(ctx context.Context) (proto.PingResponse, error) {
	require.NotNil(m.t, ctx)
	return proto.PingResponse{
		ProxyPublicAddr: m.proxyAddr,
	}, nil
}

func (m *mockAuth) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error) {
	require.NotNil(m.t, ctx)
	require.Equal(m.t, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: m.clusterName,
	}, id)
	require.False(m.t, loadKeys)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: m.clusterName,
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
	require.Equal(m.t, types.HostCA, caType)
	require.False(m.t, loadKeys)

	// We'll just wrap GetCertAuthority()'s dummy CA.
	ca, err := m.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: m.clusterName,
	}, loadKeys, opts...)
	require.NoError(m.t, err)

	return []types.CertAuthority{ca}, nil
}

func newMockAuth(t *testing.T) *mockAuth {
	return &mockAuth{
		t:           t,
		clusterName: mockClusterName,
		proxyAddr:   mockProxyAddr,
	}
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

func (b *mockBot) Client() auth.ClientI {
	return b.auth
}

func (b *mockBot) AuthenticatedUserClientFromIdentity(ctx context.Context, id *identity.Identity, authServer string) (auth.ClientI, error) {
	return nil, trace.NotImplemented("not implemented")
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
