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

package auth

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestProcessKubeCSR(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.K8s: {Enabled: true}, // test requires kube feature is enabled
			},
		},
	})

	s := newAuthSuite(t)
	const (
		username = "bob"
		roleA    = "user:bob"
		roleB    = "requestable"
	)

	// Requested user identity, presented in CSR Subject.
	userID := tlsca.Identity{
		Username:         username,
		Groups:           []string{roleA, roleB},
		Usage:            []string{"usage a", "usage b"},
		Principals:       []string{"principal a", "principal b"},
		KubernetesGroups: []string{"k8s group a", "k8s group b"},
		Traits:           map[string][]string{"trait a": {"b", "c"}},
		TeleportCluster:  s.clusterName.GetClusterName(),
	}
	subj, err := userID.Subject()
	require.NoError(t, err)

	pemCSR, err := newTestCSR(subj)
	require.NoError(t, err)
	csr := authclient.KubeCSR{
		Username:    username,
		ClusterName: s.clusterName.GetClusterName(),
		CSR:         pemCSR,
	}

	// CSR with unknown roles.
	_, err = s.a.ProcessKubeCSR(csr)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "got: %v", err)

	// Create the user and allow it to request the additional role.
	_, err = CreateUserRoleAndRequestable(s.a, username, roleB)
	require.NoError(t, err)

	// CSR with allowed, known roles.
	resp, err := s.a.ProcessKubeCSR(csr)
	require.NoError(t, err)

	cert, err := tlsca.ParseCertificatePEM(resp.Cert)
	require.NoError(t, err)
	// Note: we could compare cert.Subject with subj here directly.
	// However, because pkix.Name encoding/decoding isn't symmetric (ExtraNames
	// before encoding becomes Names after decoding), they wouldn't match.
	// Therefore, convert back to Identity, which handles this oddity and
	// should match.
	gotUserID, err := tlsca.FromSubject(cert.Subject, time.Time{})
	require.NoError(t, err)

	wantUserID := userID
	// Auth server should overwrite the Usage field and enforce UsageKubeOnly.
	wantUserID.Usage = []string{teleport.UsageKubeOnly}
	require.Equal(t, wantUserID, *gotUserID)
}

// newTestCSR creates and PEM-encodes an x509 CSR with given subject.
func newTestCSR(subj pkix.Name) ([]byte, error) {
	priv, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return nil, err
	}
	x509CSR := &x509.CertificateRequest{
		Subject: subj,
	}
	derCSR, err := x509.CreateCertificateRequest(rand.Reader, x509CSR, priv)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: derCSR}), nil
}
