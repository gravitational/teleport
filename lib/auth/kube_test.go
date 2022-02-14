// Copyright 2021 Gravitational, Inc
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

package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/rand"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestProcessKubeCSR(t *testing.T) {
	t.Parallel()
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
		Traits:           map[string][]string{"trait a": []string{"b", "c"}},
		TeleportCluster:  s.clusterName.GetClusterName(),
	}
	subj, err := userID.Subject()
	require.NoError(t, err)

	pemCSR, err := newTestCSR(subj)
	require.NoError(t, err)
	csr := KubeCSR{
		Username:    username,
		ClusterName: s.clusterName.GetClusterName(),
		CSR:         pemCSR,
	}

	// CSR with unknown roles.
	_, err = s.a.ProcessKubeCSR(csr)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

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
	require.Equal(t, *gotUserID, wantUserID)
}

// newTestCSR creates and PEM-encodes an x509 CSR with given subject.
func newTestCSR(subj pkix.Name) ([]byte, error) {
	// Use math/rand to avoid blocking on system entropy.
	rng := rand.New(rand.NewSource(0))
	priv, err := rsa.GenerateKey(rng, 2048)
	if err != nil {
		return nil, err
	}
	x509CSR := &x509.CertificateRequest{
		Subject: subj,
	}
	derCSR, err := x509.CreateCertificateRequest(rng, x509CSR, priv)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: derCSR}), nil
}
