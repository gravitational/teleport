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
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/golden"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

type templateKubernetesAuthMock struct {
	auth.ClientI
	clusterName string
	t           *testing.T
}

func (m *templateKubernetesAuthMock) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	cn, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: m.clusterName,
		ClusterID:   "aa-bb-cc",
	})
	require.NoError(m.t, err)
	return cn, nil
}

func (m *templateKubernetesAuthMock) Ping(ctx context.Context) (proto.PingResponse, error) {
	require.NotNil(m.t, ctx)
	return proto.PingResponse{
		ProxyPublicAddr: "tele.blackmesa.gov:443",
	}, nil
}

func (m *templateKubernetesAuthMock) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error) {
	require.NotNil(m.t, ctx)
	require.Equal(m.t, types.HostCA, caType)
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
				{
					PrivateKey: []byte(fixtures.SSHCAPrivateKey),
					PublicKey:  []byte(fixtures.SSHCAPublicKey),
				},
			},
		},
	})
	require.NoError(m.t, err)
	return []types.CertAuthority{ca}, nil
}

type templateKubernetesMockBot struct {
	mockAuth *templateKubernetesAuthMock
}

func (t *templateKubernetesMockBot) Client() auth.ClientI {
	return t.mockAuth
}

func (t *templateKubernetesMockBot) AuthPing(ctx context.Context) (*proto.PingResponse, error) {
	ping, err := t.mockAuth.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return &ping, err
}

func (t *templateKubernetesMockBot) ProxyPing(ctx context.Context) (*webclient.PingResponse, error) {
	return &webclient.PingResponse{}, nil
}

func (t *templateKubernetesMockBot) GetCertAuthorities(ctx context.Context, caType types.CertAuthType) ([]types.CertAuthority, error) {
	return t.mockAuth.GetCertAuthorities(ctx, caType, false)
}

// getTestIdent returns a mostly-valid bot Identity without starting up an
// entire Teleport server instance.
func getTestIdent(t *testing.T, username string, k8sCluster string) *identity.Identity {
	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, sshPublicKey, err := native.GenerateKeyPair()
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
		Username:          username,
		KubernetesUsers:   []string{"foo"},
		KubernetesGroups:  []string{"bar"},
		RouteToCluster:    "teleport.localhost.localdomain",
		KubernetesCluster: k8sCluster,
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
		RouteToCluster:    "teleport.localhost.localdomain",
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

// TestTemplateKubernetesRender renders a Kubernetes template and compares it
// to the saved golden result.
func TestTemplateKubernetesRender(t *testing.T) {
	dir := t.TempDir()
	mockAuth := &templateKubernetesAuthMock{
		t:           t,
		clusterName: "teleport.localhost.localdomain",
	}
	mockBot := &templateKubernetesMockBot{
		mockAuth: mockAuth,
	}
	template := TemplateKubernetes{
		getExecutablePath: func() (string, error) {
			return "tbot", nil
		},
	}
	require.NoError(t, template.CheckAndSetDefaults())

	k8sCluster := "example"
	dest := &DestinationConfig{
		DestinationMixin: DestinationMixin{
			Directory: &DestinationDirectory{
				Path:     dir,
				Symlinks: botfs.SymlinksInsecure,
				ACLs:     botfs.ACLOff,
			},
		},
		KubernetesCluster: &KubernetesCluster{
			ClusterName: k8sCluster,
		},
	}

	ident := getTestIdent(t, "bot-test", k8sCluster)

	err := template.Render(context.Background(), mockBot, ident, dest)
	require.NoError(t, err)

	kubeconfigBytes, err := os.ReadFile(filepath.Join(dir, template.Path))
	require.NoError(t, err)

	kubeconfigBytes = bytes.ReplaceAll(kubeconfigBytes, []byte(dir), []byte("/test/dir"))

	if golden.ShouldSet() {
		golden.SetNamed(t, "kubeconfig.yaml", kubeconfigBytes)
	}

	require.Equal(
		t, string(golden.GetNamed(t, "kubeconfig.yaml")), string(kubeconfigBytes),
	)
}
