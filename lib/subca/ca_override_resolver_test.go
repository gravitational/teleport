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
	caOverride, err := subCA.CreateCertAuthorityOverride(t.Context(), &subcav1.CertAuthorityOverride{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: string(ca.GetType()),
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: ca.GetClusterName(),
		},
		Spec: &subcav1.CertAuthorityOverrideSpec{},
	})
	require.NoError(t, err)

	caID := types.CertAuthorityOverrideID{
		ClusterName: env.ClusterName,
		CAType:      string(caType),
	}

	t.Run("ok: empty CA override", func(t *testing.T) {
		const isEnterpriseBuild = true
		const featureEnabled = true
		r, err := subca.NewCAOverrideResolver(subCA, isEnterpriseBuild, featureEnabled)
		require.NoError(t, err, "NewCAOverrideResolver errored")
		got, err := r.CalculateOverride(t.Context(), caID, subca.Certificate{PEM: caCert1PEM})
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
	o1.Disabled = false
	o2 := env.NewDisabledCertificateOverride(t, caCert2, nil)
	o4 := env.NewDisabledCertificateOverride(t, caCert4, nil)
	o4.Disabled = false
	o4.Chain = caChain.LeafToRootPEMs()[:caChainLen-1] // skip root
	caOverride.Spec.CertificateOverrides = []*subcav1.CertificateOverride{
		o1,
		o2,
		o4,
	}
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
		featureDisabled   bool // Inverse because most tests want enabled.
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
				PublicKeyHash:  o1.PublicKey,
				CACertificate:  subca.Certificate{PEM: []byte(o1.Certificate)},
				CAChain: []subca.Certificate{
					{PEM: []byte(o1.Certificate)},
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
				PublicKeyHash:  o4.PublicKey,
				CACertificate:  subca.Certificate{PEM: []byte(o4.Certificate)},
				CAChain: []subca.Certificate{
					{PEM: []byte(o4.Certificate)},
					{PEM: []byte(o4.Chain[0])},
					{PEM: []byte(o4.Chain[1])},
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
			name:            "ok: feature disabled",
			featureDisabled: true,
			id:              caID,
			caCert:          subca.Certificate{PEM: caCert1PEM},
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
			name: "empty id.ClusterName",
			id: types.CertAuthorityOverrideID{
				CAType: string(caType),
			},
			caCert: subca.Certificate{PEM: caCert3PEM},
			want: &subca.CalculateOverrideResult{
				CACertificate: subca.Certificate{PEM: caCert3PEM},
			},
			wantErr: "clusterName",
		},
		{
			name: "empty id.CAType",
			id: types.CertAuthorityOverrideID{
				ClusterName: env.ClusterName,
			},
			caCert: subca.Certificate{PEM: caCert3PEM},
			want: &subca.CalculateOverrideResult{
				CACertificate: subca.Certificate{PEM: caCert3PEM},
			},
			wantErr: "caType",
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

			r, err := subca.NewCAOverrideResolver(subCA, !test.notEntepriseBuild, !test.featureDisabled)
			require.NoError(t, err, "NewCAOverrideResolver errored")

			got, err := r.CalculateOverride(t.Context(), test.id, test.caCert)
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
