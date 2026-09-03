// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package authclient_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authcatest"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestVerifyPeerCertificate verifies that VerifyPeerCertificate rejects certs
// where the claimed cluster name doesn't match the issuing CA's cluster,
// or where the CA type (host vs user) doesn't match the cert's role type.
// Covers all combinations of local/remote clusters, user/system roles,
// and cluster name forgery in the cert subject.
func TestVerifyPeerCertificate(t *testing.T) {
	t.Parallel()

	const (
		localCluster  = "local"
		remoteCluster = "remote"
	)

	localHostCA, err := authcatest.NewCA(types.HostCA, localCluster)
	require.NoError(t, err)
	remoteHostCA, err := authcatest.NewCA(types.HostCA, remoteCluster)
	require.NoError(t, err)
	localUserCA, err := authcatest.NewCA(types.UserCA, localCluster)
	require.NoError(t, err)
	remoteUserCA, err := authcatest.NewCA(types.UserCA, remoteCluster)
	require.NoError(t, err)

	caMap := buildCAMap(t, localHostCA, localUserCA, remoteHostCA, remoteUserCA)

	localUser := tlsca.Identity{
		Username:        "alice",
		Groups:          []string{"devs"},
		TeleportCluster: localCluster,
	}
	localSystem := tlsca.Identity{
		Username:        "node",
		Groups:          []string{string(types.RoleNode)},
		TeleportCluster: localCluster,
	}
	remoteUser := tlsca.Identity{
		Username:        "alice",
		Groups:          []string{"devs"},
		TeleportCluster: remoteCluster,
	}
	remoteSystem := tlsca.Identity{
		Username:        "node",
		Groups:          []string{string(types.RoleNode)},
		TeleportCluster: remoteCluster,
	}

	tests := []struct {
		desc    string
		peer    *x509.Certificate
		wantErr bool
	}{
		// --- Cluster name mismatch: cert claims local but signed by remote CA ---
		{
			desc:    "local user signed by remote user CA",
			peer:    genCert(t, remoteUserCA, localUser, localCluster),
			wantErr: true,
		},
		{
			desc:    "local system role signed by remote host CA",
			peer:    genCert(t, remoteHostCA, localSystem, localCluster),
			wantErr: true,
		},
		// --- Cluster name mismatch: cert claims remote but signed by local CA ---
		{
			desc:    "remote user signed by local user CA",
			peer:    genCert(t, localUserCA, remoteUser, remoteCluster),
			wantErr: true,
		},
		{
			desc:    "remote system role signed by local host CA",
			peer:    genCert(t, localHostCA, remoteSystem, remoteCluster),
			wantErr: true,
		},
		// --- CA type mismatch: wrong CA type for role ---
		{
			desc:    "local user signed by local host CA",
			peer:    genCert(t, localHostCA, localUser, localCluster),
			wantErr: true,
		},
		{
			desc:    "local system role signed by local user CA",
			peer:    genCert(t, localUserCA, localSystem, localCluster),
			wantErr: true,
		},
		{
			desc:    "remote user signed by remote host CA",
			peer:    genCert(t, remoteHostCA, remoteUser, remoteCluster),
			wantErr: true,
		},
		{
			desc:    "remote system role signed by remote user CA",
			peer:    genCert(t, remoteUserCA, remoteSystem, remoteCluster),
			wantErr: true,
		},
		// --- Cluster name forgery in cert subject ---
		{
			desc:    "local user cert with remote cluster name in subject signed by local user CA",
			peer:    genCert(t, localUserCA, localUser, remoteCluster),
			wantErr: true,
		},
		{
			desc:    "local system cert with remote cluster name in subject signed by local host CA",
			peer:    genCert(t, localHostCA, localSystem, remoteCluster),
			wantErr: true,
		},
		{
			desc:    "remote user cert with local cluster name in subject signed by remote user CA",
			peer:    genCert(t, remoteUserCA, remoteUser, localCluster),
			wantErr: true,
		},
		{
			desc:    "remote system cert with local cluster name in subject signed by remote host CA",
			peer:    genCert(t, remoteHostCA, remoteSystem, localCluster),
			wantErr: true,
		},
		// --- Valid: correct CA type and matching cluster ---
		{
			desc:    "local user signed by local user CA",
			peer:    genCert(t, localUserCA, localUser, localCluster),
			wantErr: false,
		},
		{
			desc:    "local system role signed by local host CA",
			peer:    genCert(t, localHostCA, localSystem, localCluster),
			wantErr: false,
		},
		{
			desc:    "remote user signed by remote user CA",
			peer:    genCert(t, remoteUserCA, remoteUser, remoteCluster),
			wantErr: false,
		},
		{
			desc:    "remote system role signed by remote host CA",
			peer:    genCert(t, remoteHostCA, remoteSystem, remoteCluster),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			verify := authclient.VerifyPeerCertificate(caMap)
			err := verify(nil, [][]*x509.Certificate{{tt.peer}})
			if tt.wantErr {
				require.ErrorContains(t, err, "access denied: invalid client certificate")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestVerifyPeerCertificateEmptyChains verifies that the function handles empty or nil verified chains gracefully.
// This is important for VerifyClientCertIfGiven mode (used by the kube proxy) where no client cert may be presented.
func TestVerifyPeerCertificateEmptyChains(t *testing.T) {
	t.Parallel()
	verify := authclient.VerifyPeerCertificate(authclient.HostAndUserCAInfo{})
	require.NoError(t, verify(nil, nil))
	require.NoError(t, verify(nil, [][]*x509.Certificate{}))
	require.NoError(t, verify(nil, [][]*x509.Certificate{{}}))
}

// TestWithClusterCAs_SetsVerifyPeerCertificate verifies that WithClusterCAs
// sets VerifyPeerCertificate on the returned TLS config, and that the verifier
// correctly rejects a forged cert (user identity signed by host CA) while
// accepting a valid one (system role signed by host CA).
func TestWithClusterCAs_SetsVerifyPeerCertificate(t *testing.T) {
	t.Parallel()

	const clusterName = "test-cluster"

	hostCA, err := authcatest.NewCA(types.HostCA, clusterName)
	require.NoError(t, err)
	userCA, err := authcatest.NewCA(types.UserCA, clusterName)
	require.NoError(t, err)

	getter := &fakeCAGetter{
		cas: map[types.CertAuthType][]types.CertAuthority{
			types.HostCA: {hostCA},
			types.UserCA: {userCA},
		},
	}

	baseTLS := &tls.Config{}
	getConfig := authclient.WithClusterCAs(baseTLS, getter, clusterName, slog.Default())

	cfg, err := getConfig(&tls.ClientHelloInfo{})
	require.NoError(t, err)
	require.NotNil(t, cfg.VerifyPeerCertificate, "WithClusterCAs must set VerifyPeerCertificate")

	// Verify it rejects a forged cert: user identity signed by host CA.
	forgedCert := genCert(t, hostCA, tlsca.Identity{
		Username:        "admin",
		Groups:          []string{"admins"},
		TeleportCluster: clusterName,
	}, clusterName)

	err = cfg.VerifyPeerCertificate(nil, [][]*x509.Certificate{{forgedCert}})
	require.ErrorContains(t, err, "access denied: invalid client certificate")

	// Verify it accepts a valid cert: system role signed by host CA.
	validCert := genCert(t, hostCA, tlsca.Identity{
		Username:        "node",
		Groups:          []string{string(types.RoleNode)},
		TeleportCluster: clusterName,
	}, clusterName)

	err = cfg.VerifyPeerCertificate(nil, [][]*x509.Certificate{{validCert}})
	require.NoError(t, err)
}

// fakeCAGetter implements authclient.CAGetter for testing.
type fakeCAGetter struct {
	cas map[types.CertAuthType][]types.CertAuthority
}

func (f *fakeCAGetter) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	for _, ca := range f.cas[id.Type] {
		if ca.GetClusterName() == id.DomainName {
			return ca, nil
		}
	}
	return nil, &notFoundError{msg: "ca not found"}
}

func (f *fakeCAGetter) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error) {
	return f.cas[caType], nil
}

type notFoundError struct {
	msg string
}

func (e *notFoundError) Error() string         { return e.msg }
func (e *notFoundError) IsNotFoundError() bool { return true }

// buildCAMap creates a HostAndUserCAInfo map from test CAs.
func buildCAMap(t *testing.T, cas ...types.CertAuthority) authclient.HostAndUserCAInfo {
	t.Helper()

	caMap := make(authclient.HostAndUserCAInfo, len(cas))
	for _, ca := range cas {
		for _, kp := range ca.GetTrustedTLSKeyPairs() {
			cert, err := tlsca.ParseCertificatePEM(kp.Cert)
			require.NoError(t, err)
			info := caMap[string(cert.RawSubject)]
			switch ca.GetType() {
			case types.HostCA:
				info.IsHostCA = true
			case types.UserCA:
				info.IsUserCA = true
			}
			caMap[string(cert.RawSubject)] = info
		}
	}
	return caMap
}

// genCert generates a test certificate signed by the given CA with the specified identity and cluster name override.
func genCert(t *testing.T, ca types.CertAuthority, id tlsca.Identity, clusterName string) *x509.Certificate {
	t.Helper()

	tlsKeyPairs := ca.GetTrustedTLSKeyPairs()
	require.Len(t, tlsKeyPairs, 1)

	signer, err := tlsca.FromKeys(tlsKeyPairs[0].Cert, tlsKeyPairs[0].Key)
	require.NoError(t, err)

	priv, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	id.TeleportCluster = clusterName
	subj, err := id.Subject()
	require.NoError(t, err)

	pemCert, err := signer.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: priv.Public(),
		Subject:   subj,
		NotAfter:  time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	block, rest := pem.Decode(pemCert)
	require.NotNil(t, block)
	require.Empty(t, rest)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	return cert
}
