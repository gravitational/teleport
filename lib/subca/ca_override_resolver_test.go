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

package subca_test

import (
	"crypto/x509"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/subca"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
	"github.com/gravitational/teleport/lib/tlscatest"
)

func TestCalculateOverrideResult_ToClientOverrideDetailsProto(t *testing.T) {
	t.Parallel()

	const aPublicKeyHash = "6fbd7ba3f34c526f5d6d8ea2659f9fb5ca031712ee588ce35941d568742d44ed"

	tests := []struct {
		name string
		res  *subca.CalculateOverrideResult
		want *proto.CAOverrideCertificateDetails
	}{
		{
			name: "nil returns nil",
		},
		{
			name: "not active returns nil",
			res: &subca.CalculateOverrideResult{
				OverrideActive: false,
				PublicKeyHash:  aPublicKeyHash,
				CACertificate:  subca.Certificate{PEM: []byte("llama456")},
			},
		},
		{
			name: "active returns proto",
			res: &subca.CalculateOverrideResult{
				OverrideActive: true,
				PublicKeyHash:  aPublicKeyHash,
				CACertificate:  subca.Certificate{PEM: []byte("llama456")},
			},
			want: &proto.CAOverrideCertificateDetails{
				PublicKeyHash: aPublicKeyHash,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := test.res.ToClientOverrideDetailsProto()
			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("ToOverrideDetailsProto mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestLoadCAOverrideResolver_errors(t *testing.T) {
	t.Parallel()

	const caType = types.WindowsCA
	env := subcaenv.New(t, subcaenv.EnvParams{
		CATypesToCreate: []types.CertAuthType{caType},
	})
	clusterName := env.ClusterName
	subCA := env.SubCA

	tests := []struct {
		name    string
		id      types.CertAuthorityOverrideID
		wantErr string
	}{
		{
			name: "empty id.ClusterName",
			id: types.CertAuthorityOverrideID{
				CAType: string(caType),
			},
			wantErr: "clusterName",
		},
		{
			name: "empty id.CAType",
			id: types.CertAuthorityOverrideID{
				ClusterName: clusterName,
			},
			wantErr: "caType",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			const isEnterpriseBuild = true
			_, err := subca.LoadCAOverrideResolver(t.Context(), subCA, isEnterpriseBuild, test.id)
			assert.ErrorContains(t, err, test.wantErr)
		})
	}

}

func TestCAOverrideResolver_ApplyOverrides(t *testing.T) {
	t.Parallel()

	const ca1Type = types.WindowsCA
	const ca2Type = types.DatabaseClientCA
	env := subcaenv.New(t, subcaenv.EnvParams{
		CATypesToCreate: []types.CertAuthType{
			ca1Type,
			ca2Type,
		},
	})
	subCA := env.SubCA

	ca1ID := types.CertAuthorityOverrideID{
		ClusterName: env.ClusterName,
		CAType:      string(ca1Type),
	}
	ca2ID := types.CertAuthorityOverrideID{
		ClusterName: env.ClusterName,
		CAType:      string(ca2Type),
	}

	// Add multiple certificates to CA1:
	// - cert2 and cert3 to active keys. (cert1 already active)
	// - cert4 and cert5 to additional keys
	var ca1Cert1, ca1Cert2, ca1Cert3, ca1Cert4, ca1Cert5 []byte
	{
		const loadKeys = true
		ca, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
			Type:       ca1Type,
			DomainName: env.ClusterName,
		}, loadKeys)
		require.NoError(t, err)

		cert1 := ca.GetActiveKeys().TLS[0].Cert

		cfg := tlscatest.GenerateCAConfig{ClusterName: env.ClusterName}
		key2, cert2, err := tlscatest.GenerateSelfSignedCA(cfg)
		require.NoError(t, err)
		key3, cert3, err := tlscatest.GenerateSelfSignedCA(cfg)
		require.NoError(t, err)
		key4, cert4, err := tlscatest.GenerateSelfSignedCA(cfg)
		require.NoError(t, err)
		key5, cert5, err := tlscatest.GenerateSelfSignedCA(cfg)
		require.NoError(t, err)

		aks := ca.GetActiveKeys()
		aks.TLS = append(aks.TLS,
			&types.TLSKeyPair{Cert: cert2, Key: key2, KeyType: types.PrivateKeyType_RAW},
			&types.TLSKeyPair{Cert: cert3, Key: key3, KeyType: types.PrivateKeyType_RAW},
		)
		ca.SetActiveKeys(aks)

		tks := ca.GetAdditionalTrustedKeys()
		tks.TLS = append(tks.TLS,
			&types.TLSKeyPair{Cert: cert4, Key: key4, KeyType: types.PrivateKeyType_RAW},
			&types.TLSKeyPair{Cert: cert5, Key: key5, KeyType: types.PrivateKeyType_RAW},
		)
		ca.SetAdditionalTrustedKeys(tks)

		// Technically we don't need to persist the modified CA, as it's not queried.
		// This makes the scenario more realistic, though.
		_, err = env.Trust.UpdateCertAuthority(t.Context(), ca)
		require.NoError(t, err)

		ca1Cert1 = cert1
		ca1Cert2 = cert2
		ca1Cert3 = cert3
		ca1Cert4 = cert4
		ca1Cert5 = cert5
	}

	// Prepare overrides for CA1:
	// - cert2, cert4 and cert5 have disabled overrides.
	var ca1Override *subcav1.CertAuthorityOverride
	{
		cert2, err := tlsutils.ParseCertificatePEM(ca1Cert2)
		require.NoError(t, err)
		cert4, err := tlsutils.ParseCertificatePEM(ca1Cert4)
		require.NoError(t, err)
		cert5, err := tlsutils.ParseCertificatePEM(ca1Cert5)
		require.NoError(t, err)

		ca1Override, err = subCA.CreateCertAuthorityOverride(t.Context(), subcav1.CertAuthorityOverride_builder{
			Kind:    types.KindCertAuthorityOverride,
			SubKind: string(ca1Type),
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: env.ClusterName,
			},
			Spec: subcav1.CertAuthorityOverrideSpec_builder{
				CertificateOverrides: []*subcav1.CertificateOverride{
					env.NewDisabledCertificateOverride(t, cert2, nil),
					env.NewDisabledCertificateOverride(t, cert4, nil),
					env.NewDisabledCertificateOverride(t, cert5, nil),
				},
			}.Build(),
		}.Build())
		require.NoError(t, err)
	}

	t.Run("all overrides inactive", func(t *testing.T) {
		const isEnterpriseBuild = true
		r, err := subca.LoadCAOverrideResolver(t.Context(), subCA, isEnterpriseBuild, ca1ID)
		require.NoError(t, err, "LoadCAOverrideResolver errored")

		want := [][]byte{
			ca1Cert1,
			ca1Cert2,
			ca1Cert3,
			ca1Cert4,
			ca1Cert5,
		}
		got, err := r.ApplyOverrides(want)
		require.NoError(t, err, "ApplyOverrides errored")
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ApplyOverrides mismatch (-want +got)\n%s", diff)
		}
	})

	// Active overrides for:
	// - cert2 and cert5
	// - (cert4 remains disabled)
	var ca1Override2, ca1Override5 []byte
	{
		ca1Override.GetSpec().GetCertificateOverrides()[0].SetDisabled(false)
		ca1Override.GetSpec().GetCertificateOverrides()[2].SetDisabled(false)
		var err error
		_, err = subCA.UpdateCertAuthorityOverride(t.Context(), ca1Override)
		require.NoError(t, err)

		ca1Override2 = []byte(ca1Override.GetSpec().GetCertificateOverrides()[0].GetCertificate())
		ca1Override5 = []byte(ca1Override.GetSpec().GetCertificateOverrides()[2].GetCertificate())
	}

	// Fetch CA2 certificates.
	var ca2Cert1 []byte
	{
		const loadKeys = false
		ca, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
			Type:       ca2Type,
			DomainName: env.ClusterName,
		}, loadKeys)
		require.NoError(t, err)
		ca2Cert1 = ca.GetActiveKeys().TLS[0].Cert
	}

	tests := []struct {
		name               string
		notEntepriseBuild  bool // Inverse because most tests want enterprise.
		id                 types.CertAuthorityOverrideID
		certPEMs, wantPEMs [][]byte
	}{
		{
			name: "active and disabled overrides",
			id:   ca1ID,
			certPEMs: [][]byte{
				ca1Cert1,
				ca1Cert2,
				ca1Cert3,
				ca1Cert4,
				ca1Cert5,
			},
			wantPEMs: [][]byte{
				ca1Cert1,
				ca1Override2, // active override
				ca1Cert3,
				ca1Cert4,     // disabled override
				ca1Override5, // active override
			},
		},
		{
			name: "input targets no active overrides",
			id:   ca1ID,
			certPEMs: [][]byte{
				ca1Cert1,
				ca1Cert3,
				ca1Cert4,
			},
			wantPEMs: [][]byte{
				ca1Cert1,
				ca1Cert3,
				ca1Cert4, // disabled override
			},
		},
		{
			name: "input targets only active overrides",
			id:   ca1ID,
			certPEMs: [][]byte{
				ca1Cert2,
				ca1Cert5,
			},
			wantPEMs: [][]byte{
				ca1Override2,
				ca1Override5,
			},
		},
		{
			name: "input targets no overrides",
			id:   ca1ID,
			certPEMs: [][]byte{
				ca1Cert1,
				ca1Cert3,
			},
			wantPEMs: [][]byte{
				ca1Cert1,
				ca1Cert3,
			},
		},
		{
			name:              "non enterprise build",
			notEntepriseBuild: true,
			id:                ca1ID,
			certPEMs: [][]byte{
				ca1Cert1,
				ca1Cert2,
				ca1Cert3,
				ca1Cert4,
				ca1Cert5,
			},
			wantPEMs: [][]byte{
				ca1Cert1,
				ca1Cert2, // overrides not applied
				ca1Cert3,
				ca1Cert4,
				ca1Cert5,
			},
		},
		{
			name: "CA without a CA override resource",
			id:   ca2ID,
			certPEMs: [][]byte{
				ca2Cert1,
			},
			wantPEMs: [][]byte{
				ca2Cert1,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			r, err := subca.LoadCAOverrideResolver(t.Context(), subCA, !test.notEntepriseBuild, test.id)
			require.NoError(t, err, "LoadCAOverrideResolver errored")

			gotPEMs, err := r.ApplyOverrides(test.certPEMs)
			require.NoError(t, err, "ApplyOverrides errored")
			if diff := cmp.Diff(test.wantPEMs, gotPEMs); diff != "" {
				t.Errorf("ApplyOverrides mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestCAOverrideResolver_CalculateOverride(t *testing.T) {
	t.Parallel()

	const caChainLen = 3
	caChain, err := subcaenv.MakeCAChain(caChainLen, nil)
	require.NoError(t, err)

	const caType = types.WindowsCA
	const caTypeOther = types.DatabaseClientCA
	env := subcaenv.New(t, subcaenv.EnvParams{
		CATypesToCreate:  []types.CertAuthType{caType},
		SkipExternalRoot: true,
	})
	env.ExternalRoot = caChain[caChainLen-1]
	subCA := env.SubCA

	// Prepare a CA with multiple certificates for testing.
	const loadKeys = true
	ca, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
		Type:       caType,
		DomainName: env.ClusterName,
	}, loadKeys)
	require.NoError(t, err, "GetCertAuthority errored")

	var caCert1PEM, caCert2PEM, caCert3PEM, caCert4PEM []byte
	var caCert1, caCert2, caCert4 *x509.Certificate
	{
		var kps []*types.TLSKeyPair
		activeKeys := ca.GetActiveKeys()
		kps = append(kps, activeKeys.TLS[0])
		const numKeys = 3
		for range numKeys {
			keyPEM, certPEM, err := tlscatest.GenerateSelfSignedCA(tlscatest.GenerateCAConfig{
				ClusterName: env.ClusterName,
			})
			require.NoError(t, err)
			kps = append(kps, &types.TLSKeyPair{
				Cert:    certPEM,
				Key:     keyPEM,
				KeyType: types.PrivateKeyType_RAW,
			})
		}
		caCert1PEM = kps[0].Cert
		caCert2PEM = kps[1].Cert
		caCert3PEM = kps[2].Cert
		caCert4PEM = kps[3].Cert

		activeKeys.TLS = append(activeKeys.TLS, kps...)
		ca.SetActiveKeys(activeKeys)
		_, err = env.Trust.UpdateCertAuthority(t.Context(), ca)
		require.NoError(t, err)

		var certs []*x509.Certificate
		for _, kp := range kps {
			cert, err := tlsutils.ParseCertificatePEM(kp.Cert)
			require.NoError(t, err)
			certs = append(certs, cert)
		}
		caCert1 = certs[0]
		caCert2 = certs[1]
		// caCert3 not needed
		caCert4 = certs[3]
	}

	// Prepare and test an "empty" CA override.
	caOverride, err := subCA.CreateCertAuthorityOverride(t.Context(), subcav1.CertAuthorityOverride_builder{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: string(ca.GetType()),
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: ca.GetClusterName(),
		},
		Spec: &subcav1.CertAuthorityOverrideSpec{},
	}.Build())
	require.NoError(t, err)

	caID := types.CertAuthorityOverrideID{
		ClusterName: env.ClusterName,
		CAType:      string(caType),
	}

	t.Run("ok: empty CA override", func(t *testing.T) {
		const isEnterpriseBuild = true
		r, err := subca.LoadCAOverrideResolver(t.Context(), subCA, isEnterpriseBuild, caID)
		require.NoError(t, err, "LoadCAOverrideResolver errored")
		got, err := r.CalculateOverride(subca.Certificate{PEM: caCert1PEM})
		require.NoError(t, err, "CalculateCAOverride errored")

		want := &subca.CalculateOverrideResult{
			CACertificate: subca.Certificate{PEM: caCert1PEM},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("CalculateCAOverride mismatch (-want +got)\n%s", diff)
		}
	})

	// Prepare a set of overrides:
	//   - o1: target caCert1, enabled
	//   - o2: target caCert2, disabled
	//   - (o3 doesn't exist)
	//   - o4: target caCert4, enabled, has chain
	o1 := env.NewDisabledCertificateOverride(t, caCert1, nil)
	o1.SetDisabled(false)
	o2 := env.NewDisabledCertificateOverride(t, caCert2, nil)
	o4 := env.NewDisabledCertificateOverride(t, caCert4, nil)
	o4.SetDisabled(false)
	o4.SetChain(caChain.LeafToRootPEMs()[:caChainLen-1]) // skip root
	caOverride.GetSpec().SetCertificateOverrides([]*subcav1.CertificateOverride{
		o1,
		o2,
		o4,
	})
	_, err = subCA.UpdateCertAuthorityOverride(t.Context(), caOverride)
	require.NoError(t, err)

	caOtherID := types.CertAuthorityOverrideID{
		ClusterName: env.ClusterName,
		CAType:      string(caTypeOther),
	}
	caOtherCertPEM := []byte("Pretend this is a PEM. Contents not parsed.")

	tests := []struct {
		name              string
		notEntepriseBuild bool // Inverse because most tests want enterprise.
		id                types.CertAuthorityOverrideID
		caCert            subca.Certificate
		wantErr           string
		want              *subca.CalculateOverrideResult
	}{
		{
			name:   "ok: no CA override resource exists",
			id:     caOtherID,
			caCert: subca.Certificate{PEM: caOtherCertPEM},
			want: &subca.CalculateOverrideResult{
				// caOther has no matching CA override resource.
				CACertificate: subca.Certificate{PEM: caOtherCertPEM},
			},
		},
		{
			name:   "ok: active override applied",
			id:     caID,
			caCert: subca.Certificate{PEM: caCert1PEM},
			want: &subca.CalculateOverrideResult{
				// caCert1 is targeted by o1.
				OverrideActive: true,
				PublicKeyHash:  o1.GetPublicKey(),
				CACertificate:  subca.Certificate{PEM: []byte(o1.GetCertificate())},
				CAChain: []subca.Certificate{
					{PEM: []byte(o1.GetCertificate())},
				},
			},
		},
		{
			name:   "ok: active override with chain",
			id:     caID,
			caCert: subca.Certificate{PEM: caCert4PEM},
			want: &subca.CalculateOverrideResult{
				// caCert4 is targeted by o4.
				OverrideActive: true,
				PublicKeyHash:  o4.GetPublicKey(),
				CACertificate:  subca.Certificate{PEM: []byte(o4.GetCertificate())},
				CAChain: []subca.Certificate{
					{PEM: []byte(o4.GetCertificate())},
					{PEM: []byte(o4.GetChain()[0])},
					{PEM: []byte(o4.GetChain()[1])},
				},
			},
		},
		{
			name:              "ok: build is not Enterprise",
			notEntepriseBuild: true,
			id:                caID,
			caCert:            subca.Certificate{PEM: caCert1PEM},
			want: &subca.CalculateOverrideResult{
				// If enabled then o1 would apply, per test above.
				CACertificate: subca.Certificate{PEM: caCert1PEM},
			},
		},
		{
			name:   "ok: inactive override not applied",
			id:     caID,
			caCert: subca.Certificate{PEM: caCert2PEM},
			want: &subca.CalculateOverrideResult{
				// caCert2 is targeted by o2, which is inactive.
				CACertificate: subca.Certificate{PEM: caCert2PEM},
			},
		},
		{
			name:   "ok: certificate not targeted by overrides",
			id:     caID,
			caCert: subca.Certificate{PEM: caCert3PEM},
			want: &subca.CalculateOverrideResult{
				// caCert3 is not targeted by an override.
				CACertificate: subca.Certificate{PEM: caCert3PEM},
			},
		},
		{
			name:   "empty caCert.PEM",
			id:     caID,
			caCert: subca.Certificate{},
			want: &subca.CalculateOverrideResult{
				CACertificate: subca.Certificate{PEM: caCert3PEM},
			},
			wantErr: "caCert",
		},
		{
			name:   "invalid caCert.PEM",
			id:     caID,
			caCert: subca.Certificate{PEM: []byte("not a PEM")},
			want: &subca.CalculateOverrideResult{
				CACertificate: subca.Certificate{PEM: caCert3PEM},
			},
			wantErr: "parse CA certificate",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			r, err := subca.LoadCAOverrideResolver(t.Context(), subCA, !test.notEntepriseBuild, test.id)
			require.NoError(t, err, "LoadCAOverrideResolver errored")

			got, err := r.CalculateOverride(test.caCert)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr)
				return
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("CalculateCAOverride mismatch (-want +got)\n%s", diff)
			}
		})
	}
}
