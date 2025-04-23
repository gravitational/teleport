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

package common

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestWorkloadIdentityIssue(t *testing.T) {
	ctx := context.Background()

	role, err := types.NewRole("spiffe-issuer", types.RoleSpecV6{
		Allow: types.RoleConditions{
			SPIFFE: []*types.SPIFFERoleCondition{
				{
					Path:    "/*",
					IPSANs:  []string{"0.0.0.0/0"},
					DNSSANs: []string{"*"},
				},
			},
		},
	})
	require.NoError(t, err)
	s := newTestSuite(t, withRootConfigFunc(func(cfg *servicecfg.Config) {
		// reconfig the user to use the new role instead of the default ones
		// User is the second bootstrap resource.
		user, ok := cfg.Auth.BootstrapResources[1].(types.User)
		require.True(t, ok)
		user.AddRole(role.GetName())
		cfg.Auth.BootstrapResources[1] = user
		cfg.Auth.BootstrapResources = append(cfg.Auth.BootstrapResources, role)
	}),
	)

	homeDir, _ := mustLoginLegacy(t, s)
	temp := t.TempDir()
	err = Run(
		ctx,
		[]string{
			"svid",
			"issue",
			"--insecure",
			"--output", temp,
			"--svid-ttl", "10m",
			"--dns-san", "example.com",
			"--dns-san", "foo.example.com",
			"--ip-san", "10.0.0.1",
			"--ip-san", "10.1.0.1",
			"/foo/bar",
		},
		setHomePath(homeDir),
	)
	require.NoError(t, err)

	certPEM, err := os.ReadFile(filepath.Join(temp, "svid.pem"))
	require.NoError(t, err)
	certBlock, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err)
	require.Equal(t, "example.com", cert.DNSNames[0])
	require.Equal(t, "foo.example.com", cert.DNSNames[1])
	require.Equal(t, net.IP{10, 0, 0, 1}, cert.IPAddresses[0])
	require.Equal(t, net.IP{10, 1, 0, 1}, cert.IPAddresses[1])
	require.Equal(t, "spiffe://root/foo/bar", cert.URIs[0].String())
	// Sanity check we generated an ECDSA public key (test suite uses
	// balanced-v1 algorithm suite).
	require.IsType(t, &ecdsa.PublicKey{}, cert.PublicKey)

	keyPEM, err := os.ReadFile(filepath.Join(temp, "svid_key.pem"))
	require.NoError(t, err)
	keyBlock, _ := pem.Decode(keyPEM)
	privateKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	require.NoError(t, err)
	// Sanity check private key matches x509 cert subject.
	require.Implements(t, (*crypto.Signer)(nil), privateKey)
	require.Equal(t, cert.PublicKey, privateKey.(crypto.Signer).Public())

	bundlePEM, err := os.ReadFile(filepath.Join(temp, "svid_bundle.pem"))
	require.NoError(t, err)
	bundleBlock, _ := pem.Decode(bundlePEM)
	_, err = x509.ParseCertificate(bundleBlock.Bytes)
	require.NoError(t, err)
}

func TestWorkloadIdentityIssueX509(t *testing.T) {
	ctx := context.Background()

	role, err := types.NewRole("workload-identity-issuer", types.RoleSpecV6{
		Allow: types.RoleConditions{
			WorkloadIdentityLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			Rules: []types.Rule{
				types.NewRule(types.KindWorkloadIdentity, services.RO()),
			},
		},
	})
	require.NoError(t, err)
	s := newTestSuite(t, withRootConfigFunc(func(cfg *servicecfg.Config) {
		// reconfig the user to use the new role instead of the default ones
		// User is the second bootstrap resource.
		user, ok := cfg.Auth.BootstrapResources[1].(types.User)
		require.True(t, ok)
		user.AddRole(role.GetName())
		cfg.Auth.BootstrapResources[1] = user
		cfg.Auth.BootstrapResources = append(cfg.Auth.BootstrapResources, role)
	}))
	setWorkloadIdentityX509CAOverride(ctx, t, s.root)

	_, err = s.root.GetAuthServer().Services.UpsertWorkloadIdentity(
		ctx,
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:   "my-workload-identity",
				Labels: map[string]string{},
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/test",
				},
			},
		},
	)
	require.NoError(t, err)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		_, err := s.root.GetAuthServer().Cache.GetWorkloadIdentity(ctx, "my-workload-identity")
		require.NoError(collect, err)
	}, time.Second*5, 100*time.Millisecond)

	homeDir, _ := mustLoginLegacy(t, s)
	temp := t.TempDir()
	err = Run(
		ctx,
		[]string{
			"workload-identity",
			"issue-x509",
			"--insecure",
			"--output", temp,
			"--credential-ttl", "10m",
			"--name-selector", "my-workload-identity",
		},
		setHomePath(homeDir),
	)
	require.NoError(t, err)

	certPEM, err := os.ReadFile(filepath.Join(temp, "svid.pem"))
	require.NoError(t, err)
	certs, err := tlsca.ParseCertificatePEMs(certPEM)
	require.NoError(t, err)
	// the override includes a chain with a single certificate
	require.Len(t, certs, 2)
	require.Equal(t, "spiffe://root/test", certs[0].URIs[0].String())
	// Sanity check we generated an ECDSA public key (test suite uses
	// balanced-v1 algorithm suite).
	require.IsType(t, (*ecdsa.PublicKey)(nil), certs[0].PublicKey)
	keyPEM, err := os.ReadFile(filepath.Join(temp, "svid_key.pem"))
	require.NoError(t, err)
	keyBlock, _ := pem.Decode(keyPEM)
	privateKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	require.NoError(t, err)
	// Sanity check private key matches x509 cert subject.
	require.Implements(t, (*crypto.Signer)(nil), privateKey)
	require.Equal(t, certs[0].PublicKey, privateKey.(crypto.Signer).Public())

	bundlePEM, err := os.ReadFile(filepath.Join(temp, "svid_bundle.pem"))
	require.NoError(t, err)
	bundleBlock, _ := pem.Decode(bundlePEM)
	_, err = x509.ParseCertificate(bundleBlock.Bytes)
	require.NoError(t, err)
}

func setWorkloadIdentityX509CAOverride(ctx context.Context, t *testing.T, process *service.TeleportProcess) {
	const loadKeysFalse = false
	spiffeCA, err := process.GetAuthServer().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: "root",
		Type:       types.SPIFFECA,
	}, loadKeysFalse)
	require.NoError(t, err)

	spiffeCAX509KeyPairs := spiffeCA.GetTrustedTLSKeyPairs()
	require.Len(t, spiffeCAX509KeyPairs, 1)
	spiffeCACert, err := tlsca.ParseCertificatePEM(spiffeCAX509KeyPairs[0].Cert)
	require.NoError(t, err)

	// this is a bit of a hack: by adding the self-signed CA certificate to the
	// override chain we distribute a nonempty chain that we can test for, but
	// all validations will continue working and it's technically not a broken
	// intermediate chain (just a bit of a useless one)

	// (this is an unsynced write but we know that nothing is issuing
	// certificates just yet)
	process.GetAuthServer().SetWorkloadIdentityX509CAOverrideGetter(&staticOverrideGetter{chain: [][]byte{spiffeCACert.Raw}})
}

type staticOverrideGetter struct {
	chain [][]byte
}

var _ services.WorkloadIdentityX509CAOverrideGetter = (*staticOverrideGetter)(nil)

// GetWorkloadIdentityX509CAOverride implements [services.WorkloadIdentityX509CAOverrideGetter].
func (m *staticOverrideGetter) GetWorkloadIdentityX509CAOverride(ctx context.Context, name string, ca *tlsca.CertAuthority) (*tlsca.CertAuthority, [][]byte, error) {
	return ca, m.chain, nil
}
