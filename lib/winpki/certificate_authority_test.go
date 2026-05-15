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

package winpki

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/subca"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
	"github.com/gravitational/teleport/lib/tlscatest"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestCertificateStoreClient_Update(t *testing.T) {
	t.Parallel()

	const clusterName = "zarquon"
	const domain = "LLAMA"
	const caCRL = "<insert CA CRL here>"
	const overrideCRL = "<insert override CRL here>"

	// Prepare a CA. (Only the relevant bits.)
	caKeyPEM, caCertPEM, err := tlscatest.GenerateSelfSignedCA(tlscatest.GenerateCAConfig{
		ClusterName: clusterName,
	})
	require.NoError(t, err)
	caCert, err := tlsutils.ParseCertificatePEM(caCertPEM)
	require.NoError(t, err)
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.WindowsCA,
		ClusterName: clusterName,
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert:    caCertPEM,
					Key:     caKeyPEM,
					KeyType: types.PrivateKeyType_RAW,
					CRL:     []byte(caCRL),
				},
			},
		},
	})
	require.NoError(t, err)

	// Prepare a CA override. (Only the relevant bits.)
	overrideRoot, err := subcaenv.NewSelfSignedCA(nil /* params */)
	require.NoError(t, err)
	overrideCA, err := overrideRoot.NewIntermediateCA(&subcaenv.CAParams{
		Pub: caCert.PublicKey,
		Template: &x509.Certificate{
			Subject: pkix.Name{
				Organization: caCert.Subject.Organization,
				CommonName:   "Llama Windows CA", // customized CN.
			},
			NotAfter: caCert.NotAfter,
		},
	})
	require.NoError(t, err)
	overrideCert, overrideCertPEM := overrideCA.Cert, overrideCA.CertPEM
	publicKeyHash := subca.HashCertificatePublicKey(overrideCert)
	caOverride := &subcav1.CertAuthorityOverride{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: string(types.WindowsCA),
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: clusterName,
		},
		Spec: &subcav1.CertAuthorityOverrideSpec{
			CertificateOverrides: []*subcav1.CertificateOverride{
				{
					PublicKey:   publicKeyHash,
					Certificate: string(overrideCertPEM),
					Disabled:    true, // We'll push the CRL even if it's disabled.
				},
			},
		},
		Status: &subcav1.CertAuthorityOverrideStatus{
			PublicKeyHashToCrl: map[string]*subcav1.CertificateRevocationList{
				publicKeyHash: {
					Pem: string(pem.EncodeToMemory(&pem.Block{
						Type:  "X509 CRL",
						Bytes: []byte(overrideCRL),
					})),
				},
			},
		},
	}

	ldap := newFakeLDAP()
	csc := NewCertificateStoreClient(CertificateStoreConfig{
		AccessPoint: &fakeAccessPoint{
			cas: []types.CertAuthority{
				ca,
			},
			caOverrides: []*subcav1.CertAuthorityOverride{
				caOverride,
			},
		},
		Domain:      domain,
		Logger:      logtest.NewLogger(),
		ClusterName: clusterName,
		LC:          &LDAPConfig{}, // Unused, LDAP is faked.
		DialLDAPForTesting: func(context.Context, *LDAPConfig, *tls.Config) (LDAPClientForCRLUpdate, error) {
			return ldap.newClient(), nil
		},
	})

	tc := &tls.Config{} // Unused, LDAP is faked.
	require.NoError(t,
		csc.Update(t.Context(), tc),
		"Update errored",
	)

	// Prepare wanted state.
	containerDN, err := crlContainerDN(domain, types.WindowsCA)
	require.NoError(t, err)
	caCRLDN, err := CRLDN(caCert.Subject.CommonName, caCert.SubjectKeyId, domain, types.WindowsCA)
	require.NoError(t, err)
	overrideCRLDN, err := CRLDN(overrideCert.Subject.CommonName, overrideCert.SubjectKeyId, domain, types.WindowsCA)
	require.NoError(t, err)
	want := map[string]*ldapEntry{
		containerDN: {
			ObjectClass: "container",
		},
		caCRLDN: {
			ObjectClass: "cRLDistributionPoint",
			Attrs: map[string][]string{
				"certificateRevocationList": {caCRL},
			},
		},
		overrideCRLDN: {
			ObjectClass: "cRLDistributionPoint",
			Attrs: map[string][]string{
				"certificateRevocationList": {overrideCRL},
			},
		},
	}

	// Assert LDAP state.
	if diff := cmp.Diff(want, ldap.entries); diff != "" {
		t.Fatalf("LDAP entries mismatch (-want +got)\n%s", diff)
	}

	// Exercise entry update flow.
	t.Run("update", func(t *testing.T) {
		require.NoError(t,
			csc.Update(t.Context(), tc),
			"Update errored",
		)

		// Assert LDAP state.
		if diff := cmp.Diff(want, ldap.entries); diff != "" {
			t.Fatalf("LDAP entries mismatch (-want +got)\n%s", diff)
		}
	})
}

type fakeAccessPoint struct {
	CRLGenerator

	cas         []types.CertAuthority
	caOverrides []*subcav1.CertAuthorityOverride
}

func (f *fakeAccessPoint) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error) {
	var resp []types.CertAuthority
	for _, ca := range f.cas {
		if ca.GetType() != caType {
			continue
		}
		if !loadKeys {
			ca = ca.WithoutSecrets().(types.CertAuthority)
		}
		resp = append(resp, ca)
	}
	return resp, nil
}

func (f *fakeAccessPoint) GetCertAuthorityOverride(ctx context.Context, id types.CertAuthorityOverrideID) (*subcav1.CertAuthorityOverride, error) {
	for _, caOverride := range f.caOverrides {
		if caOverride.Metadata.Name == id.ClusterName && caOverride.SubKind == id.CAType {
			return caOverride, nil
		}
	}
	return nil, trace.NotFound("not found")
}

type ldapEntry struct {
	ObjectClass string
	Attrs       map[string][]string
}

type fakeLDAP struct {
	entries map[string]*ldapEntry // dn -> entry
}

func newFakeLDAP() *fakeLDAP {
	return &fakeLDAP{
		entries: make(map[string]*ldapEntry),
	}
}

func (l *fakeLDAP) newClient() *fakeLDAPClient {
	return &fakeLDAPClient{ldap: l}
}

func (l *fakeLDAP) CreateContainer(ctx context.Context, dn string) error {
	if _, ok := l.entries[dn]; ok {
		return nil
	}
	l.entries[dn] = &ldapEntry{
		ObjectClass: "container",
	}
	return nil
}

func (l *fakeLDAP) Create(dn string, class string, attrs map[string][]string) error {
	if _, ok := l.entries[dn]; ok {
		// AlreadyExists triggers the update flow on CertificateStoreClient.
		return trace.AlreadyExists("entry %q already exists", dn)
	}
	l.entries[dn] = &ldapEntry{
		ObjectClass: class,
		Attrs:       attrs,
	}
	return nil
}

func (l *fakeLDAP) Update(ctx context.Context, dn string, replaceAttrs map[string][]string) error {
	e, ok := l.entries[dn]
	if !ok {
		return fmt.Errorf("entry %q not found", dn)
	}
	e.Attrs = replaceAttrs
	return nil
}

type fakeLDAPClient struct {
	ldap   *fakeLDAP
	closed bool
}

func (c *fakeLDAPClient) Close() error {
	c.closed = true
	return nil
}

func (c *fakeLDAPClient) CreateContainer(ctx context.Context, dn string) error {
	if c.closed {
		return errors.New("ldap client closed")
	}
	return c.ldap.CreateContainer(ctx, dn)
}

func (c *fakeLDAPClient) Create(dn string, class string, attrs map[string][]string) error {
	if c.closed {
		return errors.New("ldap client closed")
	}
	return c.ldap.Create(dn, class, attrs)
}

func (c *fakeLDAPClient) Update(ctx context.Context, dn string, replaceAttrs map[string][]string) error {
	if c.closed {
		return errors.New("ldap client closed")
	}
	return c.ldap.Update(ctx, dn, replaceAttrs)
}
