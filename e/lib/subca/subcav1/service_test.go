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

package subcav1_test

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/e/lib/subca/subcav1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/subca"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/tlscatest"
)

func TestService_authz(t *testing.T) {
	t.Parallel()

	authorizer := &denyAuthorizer{}
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		Authorizer: authorizer,
	})
	subCA := env.SubCAClient

	const caType = string(types.WindowsCA)
	clusterName := env.ClusterName
	validLookingCAOverride := subcapb.CertAuthorityOverride_builder{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: caType,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: clusterName,
		}.Build(),
		Spec: subcapb.CertAuthorityOverrideSpec_builder{
			CertificateOverrides: []*subcapb.CertificateOverride{
				nil, // Passes initial checks, but fails validation.
			},
		}.Build(),
	}.Build()

	tests := []struct {
		name                   string
		doRPC                  func(t *testing.T) error
		want                   []*authorizeAttempt
		adminActionNotRequired bool
	}{
		{
			name: "CreateCSR",
			doRPC: func(t *testing.T) error {
				_, err := subCA.CreateCSR(
					t.Context(), subcapb.CreateCSRRequest_builder{
						CaType: caType,
					}.Build())
				return err
			},
			want: []*authorizeAttempt{
				// Order is deterministic.
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbRead},
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbList},
			},
			adminActionNotRequired: true,
		},
		{
			name: "CreateCertAuthorityOverride",
			doRPC: func(t *testing.T) error {
				_, err := subCA.CreateCertAuthorityOverride(
					t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
						CaOverride: validLookingCAOverride,
					}.Build())
				return err
			},
			want: []*authorizeAttempt{
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbCreate},
			},
		},
		{
			name: "UpdateCertAuthorityOverride",
			doRPC: func(t *testing.T) error {
				_, err := subCA.UpdateCertAuthorityOverride(
					t.Context(), subcapb.UpdateCertAuthorityOverrideRequest_builder{
						CaOverride: validLookingCAOverride,
					}.Build())
				return err
			},
			want: []*authorizeAttempt{
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbUpdate},
			},
		},
		{
			name: "UpsertCertAuthorityOverride",
			doRPC: func(t *testing.T) error {
				_, err := subCA.UpsertCertAuthorityOverride(
					t.Context(), subcapb.UpsertCertAuthorityOverrideRequest_builder{
						CaOverride: validLookingCAOverride,
					}.Build())
				return err
			},
			want: []*authorizeAttempt{
				// Order is deterministic.
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbUpdate},
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbCreate},
			},
		},
		{
			name: "GetCertAuthorityOverride",
			doRPC: func(t *testing.T) error {
				_, err := subCA.GetCertAuthorityOverride(t.Context(), subcapb.GetCertAuthorityOverrideRequest_builder{
					CaId: subcapb.CertAuthorityOverrideID_builder{
						CaType: caType,
					}.Build(),
				}.Build())
				return err
			},
			want: []*authorizeAttempt{
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbRead},
			},
			adminActionNotRequired: true,
		},
		{
			name: "ListCertAuthorityOverride",
			doRPC: func(t *testing.T) error {
				_, err := subCA.ListCertAuthorityOverride(
					t.Context(), &subcapb.ListCertAuthorityOverrideRequest{})
				return err
			},
			want: []*authorizeAttempt{
				// Order is deterministic.
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbRead},
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbList},
			},
			adminActionNotRequired: true,
		},
		{
			name: "DeleteCertAuthorityOverride",
			doRPC: func(t *testing.T) error {
				_, err := subCA.DeleteCertAuthorityOverride(
					t.Context(), subcapb.DeleteCertAuthorityOverrideRequest_builder{
						CaId: subcapb.CertAuthorityOverrideID_builder{
							CaType: caType, // Not found.
						}.Build(),
					}.Build())
				return err
			},
			want: []*authorizeAttempt{
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbDelete},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Don't t.Parallel(), denyAuthorizer is not built for concurrency.

			authorizer.Reset()
			err := test.doRPC(t)
			require.ErrorAs(t, err, new(*trace.AccessDeniedError), "RPC error mismatch")
			assert.ErrorContains(t, err, "deny authorizer")

			got := authorizer.GetAttemptsAndReset()
			want := test.want
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Authz attempts mismatch (-want +got)\n%s", diff)
			}

			t.Run("admin actions", func(t *testing.T) {
				if test.adminActionNotRequired {
					authorizer.SetAllowWithAdminAction(authz.AdminActionAuthUnauthorized)
					// Success or a non-AccessDenied error are both valid.
					if err := test.doRPC(t); err != nil {
						assert.NotErrorAs(t, err, new(*trace.AccessDeniedError),
							"Want admin action not required")
					}
					return
				}

				// Admin action requirement fails.
				authorizer.SetAllowWithAdminAction(authz.AdminActionAuthUnauthorized)
				err := test.doRPC(t)
				require.ErrorAs(t, err, new(*trace.AccessDeniedError),
					"Want admin action required")
				require.ErrorContains(t, err, "admin-level API")

				// Admin action requirement fulfilled.
				authorizer.SetAllowWithAdminAction(authz.AdminActionAuthMFAVerifiedWithReuse)
				err = test.doRPC(t)
				assert.NotErrorAs(t, err, new(*trace.AccessDeniedError),
					"Want admin action fulfilled")
			})
		})
	}
}

type authorizeAttempt struct {
	Rule, Verb string
}

// denyAuthorizer is an Authorizer/Checker implementation that denies all
// access.
// All access attempts are recorded so that rules/verbs used by the system can
// be verified.
type denyAuthorizer struct {
	services.AccessChecker

	attempts []*authorizeAttempt

	allowNext        bool
	adminActionState authz.AdminActionAuthState
}

func (a *denyAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	return &authz.Context{
		Checker:              a,
		AdminActionAuthState: a.adminActionState,
	}, nil
}

func (a *denyAuthorizer) CheckAccessToRule(context services.RuleContext, namespace string, rule string, verb string) error {
	a.attempts = append(a.attempts, &authorizeAttempt{
		Rule: rule,
		Verb: verb,
	})

	if a.allowNext {
		return nil
	}

	return trace.AccessDenied("deny authorizer")
}

// GetAttemptsAndReset returns the current authentication attempts and resets
// the authorizer state.
func (a *denyAuthorizer) GetAttemptsAndReset() []*authorizeAttempt {
	ats := a.attempts
	a.Reset()
	return ats
}

// SetAllowWithAdminAction allows following CheckAccessToRule calls to succeed
// and sets its admin action state.
func (a *denyAuthorizer) SetAllowWithAdminAction(s authz.AdminActionAuthState) {
	a.allowNext = true
	a.adminActionState = s
}

func (a *denyAuthorizer) Reset() {
	a.attempts = nil
	a.allowNext = false
	a.adminActionState = 0
}

func TestService_CreateCSR(t *testing.T) {
	t.Parallel()

	const caType1 = types.DatabaseClientCA
	const caType2 = types.WindowsCA
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{
				caType1,
				caType2,
			},
		},
	})
	subCA := env.SubCAClient

	// Fetch CA1 certificate for comparison with CSRs.
	const loadKeys = false
	ca1, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
		Type:       caType1,
		DomainName: env.ClusterName,
	}, loadKeys)
	require.NoError(t, err)
	require.Len(t, ca1.GetActiveKeys().TLS, 1, "CA has an unexpected number of active keys")
	ca1Cert, err := tlsutils.ParseCertificatePEM(ca1.GetActiveKeys().TLS[0].Cert)
	require.NoError(t, err)

	// Prepare CA2 with multiple keys, including keys that can't be parsed.
	parsedCA2 := addKeysToCA(t, env, addKeysToCAParams{
		CAType:               caType2,
		NewBadActiveKeys:     1, // cannot be parsed
		NewActiveKeys:        1, // total 2
		NewBadAdditionalKeys: 1, // cannot be parsed
		NewAdditionalKeys:    2, // total 2
	})
	// Sanity check ("good" keys).
	require.Len(t, parsedCA2.ActiveKeys, 2, "Unexpected number of parsed active keys")
	require.Len(t, parsedCA2.AdditionalKeys, 2, "Unexpected number of parsed additional keys")
	// Sanity check (includes "bad" keys).
	require.Len(t, parsedCA2.CA.GetActiveKeys().TLS, 3, "Unexpected number of CA active keys")
	require.Len(t, parsedCA2.CA.GetAdditionalTrustedKeys().TLS, 3, "Unexpected number of CA additional keys")

	// CA2 certificates.
	ca2Cert1 := parsedCA2.ActiveKeys[0]
	ca2Cert2 := parsedCA2.ActiveKeys[1]
	ca2Cert3 := parsedCA2.AdditionalKeys[0]
	ca2Cert4 := parsedCA2.AdditionalKeys[1]

	// Custom subject objects.
	var (
		customSubjectO  = "Llama Corp"
		customSubjectOU = "Llama CA"
		customSubjectCN = "Llama Teleport CA"
	)
	// We compare subjects using Names/OIDs. The types are slightly different
	// between pkix and proto types, so it's duped here.
	wantCustomSubject := pkix.Name{
		Names: []pkix.AttributeTypeAndValue{
			{Type: []int{2, 5, 4, 10}, Value: customSubjectO},
			{Type: []int{2, 5, 4, 11}, Value: customSubjectOU},
			{Type: []int{2, 5, 4, 3}, Value: customSubjectCN},
			{Type: tlsca.CAClusterNameExtensionOID, Value: env.ClusterName},
		},
	}
	customDN := subcapb.DistinguishedName_builder{
		Names: []*subcapb.AttributeTypeAndValue{
			subcapb.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 10}, Value: &customSubjectO}.Build(),
			subcapb.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 11}, Value: &customSubjectOU}.Build(),
			subcapb.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 3}, Value: &customSubjectCN}.Build(),
		},
	}.Build()

	// Similar to wantCustomSubject, but the cluster name is represented in "O=".
	wantCustomSubjectClusterO := pkix.Name{
		Names: []pkix.AttributeTypeAndValue{
			{Type: []int{2, 5, 4, 11}, Value: customSubjectOU},
			{Type: []int{2, 5, 4, 3}, Value: customSubjectCN},
			{Type: []int{2, 5, 4, 10}, Value: env.ClusterName},
		},
	}
	// "O=$clusterName".
	customDNClusterInO := subcapb.DistinguishedName_builder{
		Names: []*subcapb.AttributeTypeAndValue{
			subcapb.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 11}, Value: &customSubjectOU}.Build(),
			subcapb.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 3}, Value: &customSubjectCN}.Build(),
			subcapb.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 10}, Value: &env.ClusterName}.Build(),
		},
	}.Build()
	// No "O=".
	customDNWithoutO := subcapb.DistinguishedName_builder{
		Names: []*subcapb.AttributeTypeAndValue{
			subcapb.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 11}, Value: &customSubjectOU}.Build(),
			subcapb.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 3}, Value: &customSubjectCN}.Build(),
		},
	}.Build()

	tests := []struct {
		name     string
		req      *subcapb.CreateCSRRequest
		wantCSRs func(t *testing.T) []*x509.CertificateRequest
	}{
		{
			name: "ok",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(caType1),
			}.Build(),
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				return []*x509.CertificateRequest{
					newExpectedCSR(ca1Cert, nil),
				}
			},
		},
		{
			name: "multiple CSRs",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(caType2),
			}.Build(),
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				return []*x509.CertificateRequest{
					newExpectedCSR(ca2Cert1, nil),
					newExpectedCSR(ca2Cert2, nil),
					newExpectedCSR(ca2Cert3, nil),
					newExpectedCSR(ca2Cert4, nil),
				}
			},
		},
		{
			name: "public_key_hash active cert",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(caType2),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: subca.HashCertificatePublicKey(ca2Cert2),
				}.Build(),
			}.Build(),
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				return []*x509.CertificateRequest{
					newExpectedCSR(ca2Cert2, nil),
				}
			},
		},
		{
			name: "public_key_hash additional cert",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(caType2),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: subca.HashCertificatePublicKey(ca2Cert3),
				}.Build(),
			}.Build(),
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				return []*x509.CertificateRequest{
					newExpectedCSR(ca2Cert3, nil),
				}
			},
		},
		{
			name: "public_key_hash case insensitive",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(caType2),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					// subca.HashCertificatePublicKey/HashPublicKey returns a lowercase
					// string.
					Value: strings.ToUpper(subca.HashCertificatePublicKey(ca2Cert2)),
				}.Build(),
			}.Build(),
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				return []*x509.CertificateRequest{
					newExpectedCSR(ca2Cert2, nil),
				}
			},
		},
		{
			name: "custom subject targets single cert",
			req: subcapb.CreateCSRRequest_builder{
				CaType:        string(caType1), // only one active cert.
				CustomSubject: customDN,
			}.Build(),
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				return []*x509.CertificateRequest{
					newExpectedCSR(ca1Cert, &wantCustomSubject),
				}
			},
		},
		{
			name: "custom subject and public_key_hash",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(caType2),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: subca.HashCertificatePublicKey(ca2Cert3),
				}.Build(),
				CustomSubject: customDN,
			}.Build(),
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				return []*x509.CertificateRequest{
					newExpectedCSR(ca2Cert3, &wantCustomSubject),
				}
			},
		},
		{
			name: "custom subject respects O=",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(caType2),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: subca.HashCertificatePublicKey(ca2Cert3),
				}.Build(),
				CustomSubject: customDNClusterInO, // O=clusterName
			}.Build(),
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				return []*x509.CertificateRequest{
					newExpectedCSR(ca2Cert3, &wantCustomSubjectClusterO),
				}
			},
		},
		{
			name: "custom subject favors O=",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(caType2),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: subca.HashCertificatePublicKey(ca2Cert3),
				}.Build(),
				CustomSubject: customDNWithoutO, // O= not present, added in the response
			}.Build(),
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				return []*x509.CertificateRequest{
					newExpectedCSR(ca2Cert3, &wantCustomSubjectClusterO),
				}
			},
		},
		{
			name: "custom subject multiple O=",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(caType2),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: subca.HashCertificatePublicKey(ca2Cert3),
				}.Build(),
				CustomSubject: subcapb.DistinguishedName_builder{
					Names: []*subcapb.AttributeTypeAndValue{
						subcapb.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 10}, Value: &customSubjectO}.Build(),
						// Doesn't count. Cluster name must be the first.
						subcapb.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 10}, Value: &env.ClusterName}.Build(),
					},
				}.Build(),
			}.Build(),
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				wantSubj := &pkix.Name{
					Names: []pkix.AttributeTypeAndValue{
						// Echoes request.
						{Type: []int{2, 5, 4, 10}, Value: customSubjectO},
						{Type: []int{2, 5, 4, 10}, Value: env.ClusterName},
						// Added.
						{Type: tlsca.CAClusterNameExtensionOID, Value: env.ClusterName},
					},
				}
				return []*x509.CertificateRequest{
					newExpectedCSR(ca2Cert3, wantSubj),
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resp, err := subCA.CreateCSR(t.Context(), test.req)
			require.NoError(t, err, "CreateCSR errored")

			got := make([]*x509.CertificateRequest, len(resp.GetCsrs()))
			for i, csrPB := range resp.GetCsrs() {
				csr, err := tlsca.ParseCertificateRequestPEM([]byte(csrPB.GetPem()))
				require.NoError(t, err, "csrs[%d]: ParseCertificateRequestPEM errored", i)
				assert.NoError(t, csr.CheckSignature(), "csrs[%d]: signature check errored", i)
				normalizeCSRForCompare(csr)
				got[i] = csr
			}
			want := test.wantCSRs(t)
			for _, w := range want {
				normalizeCSRForCompare(w)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("CSR mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

type parsedCA struct {
	CA             types.CertAuthority
	ActiveKeys     []*x509.Certificate // only "good" keys
	AdditionalKeys []*x509.Certificate // only "good" keys
}

type addKeysToCAParams struct {
	CAType               types.CertAuthType
	NewBadActiveKeys     int
	NewActiveKeys        int
	NewBadAdditionalKeys int
	NewAdditionalKeys    int
}

func addKeysToCA(
	t *testing.T,
	env *subcav1.Env,
	p addKeysToCAParams,
) *parsedCA {
	t.Helper()

	const loadKeys = true
	ca, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
		Type:       p.CAType,
		DomainName: env.ClusterName,
	}, loadKeys)
	require.NoError(t, err)

	activeKeys := ca.GetActiveKeys()
	additionalKeys := ca.GetAdditionalTrustedKeys()
	var parsedActiveKeys, parsedAdditionalKeys []*x509.Certificate

	for _, spec := range []struct {
		caKeySet   *types.CAKeySet
		parsedKeys *[]*x509.Certificate
		newBadKeys int
		newKeys    int
	}{
		{
			caKeySet:   &activeKeys,
			parsedKeys: &parsedActiveKeys,
			newBadKeys: p.NewBadActiveKeys,
			newKeys:    p.NewActiveKeys,
		},
		{
			caKeySet:   &additionalKeys,
			parsedKeys: &parsedAdditionalKeys,
			newBadKeys: p.NewBadAdditionalKeys,
			newKeys:    p.NewAdditionalKeys,
		},
	} {
		out := spec.parsedKeys
		*out = make([]*x509.Certificate, 0, len(spec.caKeySet.TLS)+spec.newKeys)

		// Parse existing keys.
		for _, kp := range spec.caKeySet.TLS {
			cert, err := tlsca.ParseCertificatePEM(kp.Cert)
			require.NoError(t, err)
			*out = append(*out, cert)
		}

		genCfg := tlscatest.GenerateCAConfig{
			ClusterName: env.ClusterName,
		}

		// Add "bad" keys to the keyset (ie, cannot be parsed).
		for range spec.newBadKeys {
			_, certPEM, err := tlscatest.GenerateSelfSignedCA(genCfg)
			require.NoError(t, err)
			spec.caKeySet.TLS = append(spec.caKeySet.TLS, &types.TLSKeyPair{
				Cert:    certPEM,
				Key:     []byte{1, 2, 3, 4, 5},
				KeyType: types.PrivateKeyType_PKCS11,
			})
		}

		// Add new keys to the keyset.
		for range spec.newKeys {
			keyPEM, certPEM, err := tlscatest.GenerateSelfSignedCA(genCfg)
			require.NoError(t, err)

			// Add cert to out.
			cert, err := tlsca.ParseCertificatePEM(certPEM)
			require.NoError(t, err)
			*out = append(*out, cert)

			// Add key-pair to keyset.
			spec.caKeySet.TLS = append(spec.caKeySet.TLS, &types.TLSKeyPair{
				Cert:    certPEM,
				Key:     keyPEM,
				KeyType: types.PrivateKeyType_RAW,
			})
		}
	}

	// Update CA.
	ca.SetActiveKeys(activeKeys)
	ca.SetAdditionalTrustedKeys(additionalKeys)
	_, err = env.Trust.UpdateCertAuthority(t.Context(), ca)
	require.NoError(t, err)

	return &parsedCA{
		CA:             ca,
		ActiveKeys:     parsedActiveKeys,
		AdditionalKeys: parsedAdditionalKeys,
	}
}

func csrSubjectFromCACert(caCert *x509.Certificate) pkix.Name {
	subj := pkix.Name{
		Names: make([]pkix.AttributeTypeAndValue, len(caCert.Subject.Names)),
	}
	copy(subj.Names, caCert.Subject.Names)

	// Remove serial number OID.
	subj.Names = slices.DeleteFunc(subj.Names, func(atv pkix.AttributeTypeAndValue) bool {
		return len(atv.Type) == 4 &&
			atv.Type[0] == 2 &&
			atv.Type[1] == 5 &&
			atv.Type[2] == 4 &&
			atv.Type[3] == 5
	})

	return subj
}

func newExpectedCSR(caCert *x509.Certificate, subj *pkix.Name) *x509.CertificateRequest {
	if subj == nil {
		s := csrSubjectFromCACert(caCert)
		subj = &s
	}
	return &x509.CertificateRequest{
		RawSubjectPublicKeyInfo: caCert.RawSubjectPublicKeyInfo,
		SignatureAlgorithm:      caCert.SignatureAlgorithm,
		PublicKeyAlgorithm:      caCert.PublicKeyAlgorithm,
		PublicKey:               caCert.PublicKey,
		Subject:                 *subj,
	}
}
func normalizeCSRForCompare(csr *x509.CertificateRequest) {
	csr.Raw = nil
	csr.RawTBSCertificateRequest = nil
	csr.RawSubject = nil
	csr.Signature = nil
	csr.Subject = pkix.Name{
		Names: csr.Subject.Names,
	}
}

func TestService_CreateCSR_errors(t *testing.T) {
	t.Parallel()

	const validCAType = types.WindowsCA
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{validCAType},
		},
	})
	subCA := env.SubCAClient

	// Add multiple active keys so a custom subject always needs a target.
	addKeysToCA(t, env, addKeysToCAParams{
		CAType:        validCAType,
		NewActiveKeys: 1, // total 2
	})

	customCN := "Llama CA"
	validATV := subcapb.AttributeTypeAndValue_builder{
		Oid:   []int32{2, 5, 4, 3},
		Value: &customCN,
	}.Build()

	badClusterName := env.ClusterName + "BAD"
	// Convert []int to []int32.
	clusterNameOID := make([]int32, len(tlsca.CAClusterNameExtensionOID))
	for i, x := range tlsca.CAClusterNameExtensionOID {
		clusterNameOID[i] = int32(x)
	}

	tests := []struct {
		name    string
		req     *subcapb.CreateCSRRequest
		wantErr string
	}{
		{
			name:    "empty",
			req:     &subcapb.CreateCSRRequest{},
			wantErr: "ca_type required",
		},
		{
			name: "ca_type not allowed",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(types.UserCA),
			}.Build(),
			wantErr: "ca_type not allowed",
		},
		{
			name: "ca_type invalid",
			req: subcapb.CreateCSRRequest_builder{
				CaType: "banana",
			}.Build(),
			wantErr: "ca_type not allowed",
		},
		{
			name: "public_key_hash empty",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(validCAType),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: "", // invalid
				}.Build(),
			}.Build(),
			wantErr: "public_key_hash",
		},
		{
			name: "public_key_hash not found",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(validCAType),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: "0000000000000000000000000000000000000000000000000000000000000000",
				}.Build(),
			}.Build(),
			wantErr: "matches no CA certificate",
		},
		{
			name: "custom_subject invalid",
			req: subcapb.CreateCSRRequest_builder{
				CaType:        string(validCAType),
				CustomSubject: &subcapb.DistinguishedName{},
			}.Build(),
			wantErr: "empty distinguished name",
		},
		{
			name: "custom_subject targets multiple certificates",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(validCAType),
				CustomSubject: subcapb.DistinguishedName_builder{
					Names: []*subcapb.AttributeTypeAndValue{
						validATV,
					},
				}.Build(),
			}.Build(),
			wantErr: "cannot match more than one certificate",
		},
		{
			name: "custom_subject invalid cluster name OID",
			req: subcapb.CreateCSRRequest_builder{
				CaType: string(validCAType),
				CustomSubject: subcapb.DistinguishedName_builder{
					Names: []*subcapb.AttributeTypeAndValue{
						// "clusterNameOID" doesn't match the cluster name.
						subcapb.AttributeTypeAndValue_builder{Oid: clusterNameOID, Value: &badClusterName}.Build(),
					},
				}.Build(),
			}.Build(),
			wantErr: "cluster name invalid",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := subCA.CreateCSR(t.Context(), test.req)
			assert.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestService_Create(t *testing.T) {
	t.Parallel()

	const caType1 = types.DatabaseClientCA // invalid test
	const caType2 = types.WindowsCA        // success test

	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{
				caType1,
				caType2,
			},
		},
	})
	subCA := env.SubCAClient

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		caOverride := env.NewOverrideForCAType(t, caType2)

		// Create resource.
		createResp, err := subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
			CaOverride: caOverride,
		}.Build())
		require.NoError(t, err, "CreateCertAuthorityOverride errored")

		// Assert resource.
		got := createResp.GetCaOverride()
		want := caOverride
		want.GetMetadata().SetRevision(got.GetMetadata().GetRevision())
		want.SetStatus(got.GetStatus())
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Fatalf("Create mismatch (-want +got)\n%s", diff)
		}

		assertCRLs(t, got, env.Clock.Now())

		// Assert audit.
		emitter := env.MockEmitter
		assertCAOverrideEvent(t, emitter.Events(), &wantEvent{
			Type:    events.CertAuthOverrideCreateEvent,
			Code:    events.CertAuthOverrideCreateCode,
			Success: true,
		})

		// Assert storage.
		getResp, err := subCA.GetCertAuthorityOverride(t.Context(), subcapb.GetCertAuthorityOverrideRequest_builder{
			CaId: subcapb.CertAuthorityOverrideID_builder{
				CaType: got.GetSubKind(),
			}.Build(),
		}.Build())
		require.NoError(t, err, "GetCertAuthorityOverride errored")
		want = got
		got = getResp.GetCaOverride()
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("Get mismatch (-want +got)\n%s", diff)
		}
	})
}

func assertCRLs(
	t *testing.T,
	caOverride *subcapb.CertAuthorityOverride,
	now time.Time,
) {
	t.Helper()

	// Simpler to work with the parsed variant.
	parsed, err := subca.ParseCAOverride(caOverride)
	require.NoError(t, err, "ParseCAOverride errored")

	// All overrides with a certificate have valid CRLs.
	for i, co := range parsed.CertificateOverrides {
		if co.Certificate == nil {
			continue
		}

		// Fetch and parse CRL.
		crlPB, ok := parsed.CAOverride.GetStatus().GetPublicKeyHashToCrl()[co.PublicKey]
		require.True(t, ok, "CRL not on Status (i=%d)", i)
		block, _ := pem.Decode([]byte(crlPB.GetPem()))
		require.NotNil(t, block, "Failed to decode CRL PEM")
		crl, err := x509.ParseRevocationList(block.Bytes)
		require.NoError(t, err, "Parse CRL")

		// Assert CRL.
		assert.True(t, crl.ThisUpdate.Before(now), "CRL ThisUpdate >= now")
		assert.False(t, crl.NextUpdate.Before(co.Certificate.NotAfter), "CRL NextUpdate < Certificate NotAfter")
		assert.Equal(t,
			co.Certificate.Subject.String(),
			crl.Issuer.String(),
			"CRL issuer mismatch (i=%d)", i,
		)
		// Verify signature.
		assert.NoError(t,
			crl.CheckSignatureFrom(co.Certificate),
			"CRL signature verification failed (i=%d)", i,
		)
	}
}

func TestService_Create_fromCSR(t *testing.T) {
	t.Parallel()

	const caType = types.WindowsCA
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{caType},
		},
	})
	subCA := env.SubCAClient

	// Request CSR.
	csrResp, err := subCA.CreateCSR(t.Context(), subcapb.CreateCSRRequest_builder{
		CaType: string(caType),
	}.Build())
	require.NoError(t, err, "CreateCSR errored")
	require.Len(t, csrResp.GetCsrs(), 1, "CreateCSR returned an unexpected number of CSRs")

	// Create certificate, from CSR, using the external root.
	csr, err := tlsca.ParseCertificateRequestPEM([]byte(csrResp.GetCsrs()[0].GetPem()))
	require.NoError(t, err)
	now := env.Clock.Now()
	certDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		Subject:               csr.Subject,
		NotBefore:             now.Add(-1 * time.Minute),
		NotAfter:              now.Add(10 * time.Minute), // < self-signed CA NotAfter
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}, env.ExternalRoot.Cert, csr.PublicKey, env.ExternalRoot.Key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Prepare override.
	caOverride := subcapb.CertAuthorityOverride_builder{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: string(caType),
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: env.ClusterName,
		}.Build(),
		Spec: subcapb.CertAuthorityOverrideSpec_builder{
			CertificateOverrides: []*subcapb.CertificateOverride{
				subcapb.CertificateOverride_builder{
					Certificate: string(certPEM),
				}.Build(),
			},
		}.Build(),
	}.Build()

	// Create override.
	_, err = subCA.CreateCertAuthorityOverride(
		t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
			CaOverride: caOverride,
		}.Build())
	require.NoError(t, err, "Create errored")
}

func TestService_Update(t *testing.T) {
	t.Parallel()

	const caType = types.WindowsCA
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{caType},
		},
	})
	subCA := env.SubCAClient
	emitter := env.MockEmitter

	// Prepare CA override to update.
	createResp, err := subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
		CaOverride: env.NewOverrideForCAType(t, caType),
	}.Build())
	require.NoError(t, err, "CreateCertAuthorityOverride errored")
	created := createResp.GetCaOverride()

	t.Run("ok", func(t *testing.T) {
		ctx := t.Context()
		caOverride := created

		// Enable all overrides.
		require.NotEmpty(t,
			created.GetSpec().GetCertificateOverrides(), "Expected at least one certificate override")
		for _, override := range caOverride.GetSpec().GetCertificateOverrides() {
			override.SetDisabled(false)
		}

		emitter.Reset()
		updateResp, err := subCA.UpdateCertAuthorityOverride(ctx, subcapb.UpdateCertAuthorityOverrideRequest_builder{
			CaOverride: caOverride,
		}.Build())
		require.NoError(t, err, "UpdateCertAuthorityOverride errored")
		updated := updateResp.GetCaOverride()

		// Verify response.
		want := caOverride
		want.GetMetadata().SetRevision(updated.GetMetadata().GetRevision())
		if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
			t.Fatalf("Update mismatch (-want +got)\n%s", diff)
		}

		// Verify stored override.
		getResp, err := subCA.GetCertAuthorityOverride(ctx, subcapb.GetCertAuthorityOverrideRequest_builder{
			CaId: subcapb.CertAuthorityOverrideID_builder{
				CaType: string(caType),
			}.Build(),
		}.Build())
		require.NoError(t, err, "GetCertAuthorityOverride errored")
		if diff := cmp.Diff(want, getResp.GetCaOverride(), protocmp.Transform()); diff != "" {
			t.Errorf("Get mismatch (-want +got)\n%s", diff)
		}

		// Verify audit.
		assertCAOverrideEvent(t, emitter.Events(), &wantEvent{
			Code:    events.CertAuthOverrideUpdateCode,
			Type:    events.CertAuthOverrideUpdateEvent,
			Success: true,
		})
	})
}

// TestService_Update_errors tests errors exclusive to Update.
// See TestService_Write_errors.
func TestService_Update_errors(t *testing.T) {
	t.Parallel()

	const caType = types.WindowsCA
	const caTypeOther = types.DatabaseClientCA

	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{caType},
		},
	})
	subCA := env.SubCAClient

	// Prepare CA override to update.
	createResp, err := subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
		CaOverride: env.NewOverrideForCAType(t, caType),
	}.Build())
	require.NoError(t, err, "CreateCertAuthorityOverride errored")

	// Cloned by tests.
	baseCAOverride := createResp.GetCaOverride()

	tests := []struct {
		name      string
		makeReq   func(caOverride *subcapb.CertAuthorityOverride) *subcapb.UpdateCertAuthorityOverrideRequest
		assertErr func(t *testing.T, err error) // takes precedence over wantErr
		wantErr   string
	}{
		{
			name: "not found",
			makeReq: func(caOverride *subcapb.CertAuthorityOverride) *subcapb.UpdateCertAuthorityOverrideRequest {
				caOverride.SetSubKind(string(caTypeOther))
				caOverride.GetSpec().SetCertificateOverrides(nil) // Valid, it can be empty.
				return subcapb.UpdateCertAuthorityOverrideRequest_builder{
					CaOverride: caOverride,
				}.Build()
			},
			assertErr: func(t *testing.T, err error) {
				// This differs from a pure backend Update, which returns
				// ErrIncorrectRevision for both "not found" and "incorrect revision"
				assert.ErrorAs(t, err, new(*trace.NotFoundError), "error mismatch")
			},
		},
		{
			name: "wrong revision",
			makeReq: func(caOverride *subcapb.CertAuthorityOverride) *subcapb.UpdateCertAuthorityOverrideRequest {
				caOverride.GetMetadata().SetRevision("llamabanana")
				return subcapb.UpdateCertAuthorityOverrideRequest_builder{
					CaOverride: caOverride,
				}.Build()
			},
			assertErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, backend.ErrIncorrectRevision, "revision error mismatch")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			caOverride := proto.Clone(baseCAOverride).(*subcapb.CertAuthorityOverride)
			req := test.makeReq(caOverride)

			_, err := subCA.UpdateCertAuthorityOverride(t.Context(), req)
			if test.assertErr != nil {
				test.assertErr(t, err)
			} else {
				assert.ErrorContains(t, err, test.wantErr, "Update error mismatch")
			}
		})
	}
}

func TestService_Update_enableInvalidOverride(t *testing.T) {
	t.Parallel()

	const caType = types.WindowsCA
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{caType},
		},
	})
	subCA := env.SubCAClient

	// Fetch our target CA, plus keys.
	const loadKeys = true
	ca, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
		Type:       caType,
		DomainName: env.ClusterName,
	}, loadKeys)
	require.NoError(t, err, "GetCertAuthority errored")

	// Prepare a CA override.
	caOverride := env.NewOverrideForCA(t, ca, nil /* externalRoot */)
	createResp, err := subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
		CaOverride: caOverride,
	}.Build())
	require.NoError(t, err, "CreateCertAuthorityOverride errored")
	caOverride = createResp.GetCaOverride()

	// Simulate a rotation, so now the override has no target.
	keyPEM, certPEM, err := tlscatest.GenerateSelfSignedCA(tlscatest.GenerateCAConfig{
		ClusterName: env.ClusterName,
	})
	require.NoError(t, err, "GenerateSelfSignedCA errored")
	activeKeys := ca.GetActiveKeys()
	activeKeys.TLS = []*types.TLSKeyPair{
		{
			Cert:    certPEM,
			Key:     keyPEM,
			KeyType: types.PrivateKeyType_RAW,
		},
	}
	ca.SetActiveKeys(activeKeys)
	_, err = env.Trust.UpdateCertAuthority(t.Context(), ca)
	require.NoError(t, err, "UpdateCertAuthority errored")

	// Attempt to enable the poorly-targeted override. This should fail.
	caOverride.GetSpec().GetCertificateOverrides()[0].SetDisabled(false)
	const wantErr = "targets unknown CA certificate"
	t.Run("Update", func(t *testing.T) {
		_, err := subCA.UpdateCertAuthorityOverride(t.Context(), subcapb.UpdateCertAuthorityOverrideRequest_builder{
			CaOverride: caOverride,
		}.Build())
		assert.ErrorContains(t, err, wantErr)
	})
	t.Run("Upsert", func(t *testing.T) {
		_, err := subCA.UpsertCertAuthorityOverride(t.Context(), subcapb.UpsertCertAuthorityOverrideRequest_builder{
			CaOverride: caOverride,
		}.Build())
		assert.ErrorContains(t, err, wantErr)
	})

	t.Run("remove override", func(t *testing.T) {
		// This should work, the override isn't valid anymore.
		caOverride.GetSpec().SetCertificateOverrides(nil)
		_, err = subCA.UpdateCertAuthorityOverride(t.Context(), subcapb.UpdateCertAuthorityOverrideRequest_builder{
			CaOverride: caOverride,
		}.Build())
		assert.NoError(t, err, "Update failed to remove invalid override")
	})
}

func TestService_Upsert(t *testing.T) {
	t.Parallel()

	const caType = types.WindowsCA
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{caType},
		},
	})
	subCA := env.SubCAClient
	emitter := env.MockEmitter

	assertStored := func(t *testing.T, want *subcapb.CertAuthorityOverride) {
		getResp, err := subCA.GetCertAuthorityOverride(t.Context(), subcapb.GetCertAuthorityOverrideRequest_builder{
			CaId: subcapb.CertAuthorityOverrideID_builder{
				CaType: want.GetSubKind(),
			}.Build(),
		}.Build())
		require.NoError(t, err, "GetCertAuthorityOverride errored")
		if diff := cmp.Diff(want, getResp.GetCaOverride(), protocmp.Transform()); diff != "" {
			t.Errorf("Get mismatch (-want +got)\n%s", diff)
		}
	}

	assertUpsertAudit := func(t *testing.T, evs []apievents.AuditEvent) {
		assertCAOverrideEvent(t, evs, &wantEvent{
			Type:    events.CertAuthOverrideUpsertEvent,
			Code:    events.CertAuthOverrideUpsertCode,
			Success: true,
		})
	}

	t.Run("ok", func(t *testing.T) {
		caOverride := env.NewOverrideForCAType(t, caType)

		// Create via Upsert.
		upsertResp, err := subCA.UpsertCertAuthorityOverride(t.Context(), subcapb.UpsertCertAuthorityOverrideRequest_builder{
			CaOverride: caOverride,
		}.Build())
		require.NoError(t, err, "UpsertCertAuthorityOverride errored")
		created := upsertResp.GetCaOverride()

		// Verify response.
		want := caOverride
		want.GetMetadata().SetRevision(created.GetMetadata().GetRevision())
		want.SetStatus(created.GetStatus())
		if diff := cmp.Diff(want, created, protocmp.Transform()); diff != "" {
			t.Fatalf("Upsert mismatch (-want +got)\n%s", diff)
		}

		assertCRLs(t, created, env.Clock.Now())

		assertStored(t, want)

		assertUpsertAudit(t, emitter.Events())

		t.Run("update", func(t *testing.T) {
			caOverride := created

			// Enable all overrides.
			require.NotEmpty(t,
				caOverride.GetSpec().GetCertificateOverrides(), "Expected at least one certificate override")
			for _, override := range caOverride.GetSpec().GetCertificateOverrides() {
				override.SetDisabled(false)
			}

			// Update via Upsert.
			emitter.Reset()
			upsertResp, err := subCA.UpsertCertAuthorityOverride(t.Context(), subcapb.UpsertCertAuthorityOverrideRequest_builder{
				CaOverride: caOverride,
			}.Build())
			require.NoError(t, err, "UpsertCertAuthorityOverride errored")
			updated := upsertResp.GetCaOverride()

			// Verify response.
			want := caOverride
			want.GetMetadata().SetRevision(updated.GetMetadata().GetRevision())
			if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
				t.Fatalf("Upsert mismatch (-want +got)\n%s", diff)
			}

			assertStored(t, want)

			assertUpsertAudit(t, emitter.Events())
		})
	})
}

func TestService_Upsert_reusesStatusCRLs(t *testing.T) {
	t.Parallel()

	const caType = types.WindowsCA
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{caType},
		},
	})
	subCA := env.SubCAClient

	mustUpsert := func(t *testing.T, caOverride *subcapb.CertAuthorityOverride) *subcapb.CertAuthorityOverride {
		t.Helper()

		resp, err := subCA.UpsertCertAuthorityOverride(t.Context(), subcapb.UpsertCertAuthorityOverrideRequest_builder{
			CaOverride: caOverride,
		}.Build())
		require.NoError(t, err, "UpsertCertAuthorityOverride errored")
		return resp.GetCaOverride()
	}

	mustDelete := func(t *testing.T) {
		t.Helper()

		_, err := subCA.DeleteCertAuthorityOverride(t.Context(), subcapb.DeleteCertAuthorityOverrideRequest_builder{
			CaId: subcapb.CertAuthorityOverrideID_builder{
				CaType: string(caType),
			}.Build(),
		}.Build())
		require.NoError(t, err, "DeleteCertAuthorityOverride errored")
	}

	// Create the initial CA override. We'll save it and re-create later.
	template1 := env.NewOverrideForCAType(t, caType)
	ca1 := mustUpsert(t, template1)

	// Update using a different certificate. CRLs should differ.
	template2 := env.NewOverrideForCAType(t, caType)
	template2.SetStatus(ca1.GetStatus()) // ineffective, certificate changed.
	ca2 := mustUpsert(t, template2)
	require.NotEqual(t,
		makeCRLMap(ca1.GetStatus()),
		makeCRLMap(ca2.GetStatus()),
		"CRL map not expected to match",
	)

	// Re-create initial version, without Status. CRLs should differ.
	mustDelete(t)
	ca3 := mustUpsert(t, template1)
	require.NotEqual(t,
		makeCRLMap(ca1.GetStatus()),
		makeCRLMap(ca3.GetStatus()),
		"CRL map not expected to match",
	)

	// Update to the initial version, including Status. CRLs should match.
	// PublicKeyHashToCrl keys are normalized to lowercase by the backend.
	ca1Upper := proto.Clone(ca1).(*subcapb.CertAuthorityOverride)
	ca1Upper.GetStatus().SetPublicKeyHashToCrl(make(map[string]*subcapb.CertificateRevocationList, len(ca1.GetStatus().GetPublicKeyHashToCrl())))
	for k, v := range ca1.GetStatus().GetPublicKeyHashToCrl() {
		ca1Upper.GetStatus().GetPublicKeyHashToCrl()[strings.ToUpper(k)] = v
	}
	ca4 := mustUpsert(t, ca1Upper)
	want := makeCRLMap(ca1.GetStatus())
	got := makeCRLMap(ca4.GetStatus())
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("CRL map mismatch (-want +got)\n%s", diff)
	}
}

func TestService_Upsert_inputCRLInvalid(t *testing.T) {
	t.Parallel()

	const caType = types.WindowsCA
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{caType},
		},
	})
	clock := env.Clock
	subCA := env.SubCAClient

	const unusedKey = "unused"
	template := env.NewOverrideForCAType(t, caType)
	template.SetStatus(subcapb.CertAuthorityOverrideStatus_builder{
		PublicKeyHashToCrl: map[string]*subcapb.CertificateRevocationList{
			unusedKey: nil,
		},
	}.Build())
	resp, err := subCA.UpsertCertAuthorityOverride(t.Context(), subcapb.UpsertCertAuthorityOverrideRequest_builder{
		CaOverride: template,
	}.Build())
	require.NoError(t, err, "UpsertCertAuthorityOverride errored")

	// Assert that unusedKey is deleted.
	caOverride := resp.GetCaOverride()
	_, ok := caOverride.GetStatus().GetPublicKeyHashToCrl()[unusedKey]
	assert.False(t, ok, "ca1.Status has unexpected key %q", unusedKey)

	// Find the key for the server-created CRL.
	var crlKey string
	for k := range caOverride.GetStatus().GetPublicKeyHashToCrl() {
		crlKey = k
		break
	}
	require.NotEmpty(t, crlKey, "CRL map is unexpectedly empty")

	createExternalCRL := func(t *testing.T, params *subcaenv.CAParams) (rootCA *subcaenv.CA, crlDER []byte) {
		t.Helper()

		rootCA, err := subcaenv.NewSelfSignedCA(params)
		require.NoError(t, err, "NewSelfSignedCA errored")

		now := env.Clock.Now()
		crlDER, err = x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
			Number:     big.NewInt(1),
			ThisUpdate: now.Add(-1 * time.Minute),
			NextUpdate: now.Add(1 * time.Hour),
		}, rootCA.Cert, rootCA.Key)
		require.NoError(t, err, "CreateRevocationList errored")

		return rootCA, pem.EncodeToMemory(&pem.Block{
			Type:  "X509 CRL",
			Bytes: crlDER,
		})
	}

	// Prepare an unrelated CRL.
	_, unrelatedCRLPEM := createExternalCRL(t, &subcaenv.CAParams{
		Clock: env.Clock,
	})

	// Prepare a similar, but still unrelated, CRL.
	parsed, err := subca.ParseCAOverride(caOverride)
	require.NoError(t, err, "ParseCAOverride errored")
	// Sanity check.
	require.Equal(t,
		crlKey,
		parsed.CertificateOverrides[0].PublicKey,
		"crlKey and parsed override publicKey mismatch",
	)
	_, similarCRLPEM := createExternalCRL(t, &subcaenv.CAParams{
		Clock: env.Clock,
		Template: &x509.Certificate{
			// CRL Issuer matches the override certificate.
			Subject: parsed.CertificateOverrides[0].Certificate.Subject,
		},
	})

	createCRLUsingCA := func(t *testing.T, modifyCRL func(crl *x509.RevocationList)) (crlPEM []byte) {
		t.Helper()

		// Fetch the private key from the Teleport CA.
		const loadKeys = true
		ca, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
			Type:       caType,
			DomainName: env.ClusterName,
		}, loadKeys)
		require.NoError(t, err)
		caKey, err := keys.ParsePrivateKey(ca.GetActiveKeys().TLS[0].Key)
		require.NoError(t, err)

		// Fetch the certificate from the override.
		overrideCert, err := tlsutils.ParseCertificatePEM([]byte(caOverride.GetSpec().GetCertificateOverrides()[0].GetCertificate()))
		require.NoError(t, err)

		// Start from a valid CRL, then let the caller modify it.
		now := env.Clock.Now()
		crl := &x509.RevocationList{
			Number:     big.NewInt(1),
			ThisUpdate: now.Add(-1 * time.Minute),
			NextUpdate: now.Add(1 * time.Hour),
		}
		modifyCRL(crl)

		crlDER, err := x509.CreateRevocationList(rand.Reader, crl, overrideCert, caKey)
		require.NoError(t, err, "CreateRevocationList errored")

		return pem.EncodeToMemory(&pem.Block{
			Type:  "X509 CRL",
			Bytes: crlDER,
		})
	}

	// Prepare a few valid CRLs with invalid timestamps.
	invalidThisUpdatePEM := createCRLUsingCA(t, func(crl *x509.RevocationList) {
		crl.ThisUpdate = clock.Now().Add(10 * time.Minute) // should be < now-2m
	})
	invalidNextUpdatePEM := createCRLUsingCA(t, func(crl *x509.RevocationList) {
		crl.NextUpdate = clock.Now() // should be >= overrideCert.NotAfter
	})

	// Upsert a variety of invalid CRLs using a known CRL key, so it gets parsed.
	// None should cause failures.
	tests := []struct {
		name         string
		modifyStatus func(status *subcapb.CertAuthorityOverrideStatus)
	}{
		{
			name: "CRLPB nil",
			modifyStatus: func(status *subcapb.CertAuthorityOverrideStatus) {
				status.GetPublicKeyHashToCrl()[crlKey] = nil
			},
		},
		{
			name: "CRLPB empty",
			modifyStatus: func(status *subcapb.CertAuthorityOverrideStatus) {
				status.GetPublicKeyHashToCrl()[crlKey] = &subcapb.CertificateRevocationList{}
			},
		},
		{
			name: "CRLPB PEM invalid",
			modifyStatus: func(status *subcapb.CertAuthorityOverrideStatus) {
				status.GetPublicKeyHashToCrl()[crlKey] = subcapb.CertificateRevocationList_builder{
					Pem: "not a PEM",
				}.Build()
			},
		},
		{
			name: "CRLPB CRL invalid",
			modifyStatus: func(status *subcapb.CertAuthorityOverrideStatus) {
				val := pem.EncodeToMemory(&pem.Block{
					Type:  "X509 CRL",
					Bytes: []byte("not a CRL"),
				})
				status.GetPublicKeyHashToCrl()[crlKey] = subcapb.CertificateRevocationList_builder{
					Pem: string(val),
				}.Build()
			},
		},
		{
			name: "CRLPB CRL Issuer-Subject mismatch",
			modifyStatus: func(status *subcapb.CertAuthorityOverrideStatus) {
				status.GetPublicKeyHashToCrl()[crlKey] = subcapb.CertificateRevocationList_builder{
					Pem: string(unrelatedCRLPEM),
				}.Build()
			},
		},
		{
			name: "CRLPB CRL signature mismatch",
			modifyStatus: func(status *subcapb.CertAuthorityOverrideStatus) {
				status.GetPublicKeyHashToCrl()[crlKey] = subcapb.CertificateRevocationList_builder{
					Pem: string(similarCRLPEM),
				}.Build()
			},
		},
		{
			name: "CRLPB CRL ThisUpdate invalid",
			modifyStatus: func(status *subcapb.CertAuthorityOverrideStatus) {
				status.GetPublicKeyHashToCrl()[crlKey] = subcapb.CertificateRevocationList_builder{
					Pem: string(invalidThisUpdatePEM),
				}.Build()
			},
		},
		{
			name: "CRLPB CRL NextUpdate invalid",
			modifyStatus: func(status *subcapb.CertAuthorityOverrideStatus) {
				status.GetPublicKeyHashToCrl()[crlKey] = subcapb.CertificateRevocationList_builder{
					Pem: string(invalidNextUpdatePEM),
				}.Build()
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caOverride := proto.Clone(caOverride).(*subcapb.CertAuthorityOverride)
			test.modifyStatus(caOverride.GetStatus())

			// Upsert. It should not fail.
			resp, err := subCA.UpsertCertAuthorityOverride(t.Context(), subcapb.UpsertCertAuthorityOverrideRequest_builder{
				CaOverride: caOverride,
			}.Build())
			require.NoError(t, err, "UpsertCertAuthorityOverride errored")

			// Assert that the CRLs changed.
			assert.NotEqual(t,
				makeCRLMap(caOverride.GetStatus()),
				makeCRLMap(resp.GetCaOverride().GetStatus()),
				"CA override CRLs did not change",
			)

			// Assert that the new CRLs are valid.
			assertCRLs(t, resp.GetCaOverride(), env.Clock.Now())
		})
	}
}

func makeCRLMap(s *subcapb.CertAuthorityOverrideStatus) map[string]string {
	m := make(map[string]string)
	for k, v := range s.GetPublicKeyHashToCrl() {
		m[k] = v.GetPem()
	}
	return m
}

// TestService_Write_errors tests error conditions common to multiple CA
// override write methods.
// Namely: Create, Update and Upsert.
func TestService_Write_errors(t *testing.T) {
	t.Parallel()

	const caType = types.WindowsCA
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{caType},
		},
	})
	subCA := env.SubCAClient

	// Cloned by tests. Note that it doesn't exist on storage.
	baseCAOverride := env.NewOverrideForCAType(t, caType)

	type testCase struct {
		name           string
		makeCAOverride func(caOverride *subcapb.CertAuthorityOverride) *subcapb.CertAuthorityOverride
		skipUpdate     bool
		assertErr      func(t *testing.T, err error) // takes precedence over wantErr
		wantErr        string
	}

	assertTestCae := func(t *testing.T, tc *testCase, err error) {
		t.Helper()
		if tc.assertErr != nil {
			tc.assertErr(t, err)
		} else {
			assert.ErrorContains(t, err, tc.wantErr, "error mismatch")
		}
	}

	runTestCase := func(t *testing.T, tc *testCase, baseCAOverride *subcapb.CertAuthorityOverride) {
		caOverride := proto.Clone(baseCAOverride).(*subcapb.CertAuthorityOverride)
		if tc.makeCAOverride != nil {
			caOverride = tc.makeCAOverride(caOverride)
		}

		t.Run("create", func(t *testing.T) {
			_, err := subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
				CaOverride: caOverride,
			}.Build())
			assertTestCae(t, tc, err)
		})
		if !tc.skipUpdate {
			t.Run("update", func(t *testing.T) {
				_, err := subCA.UpdateCertAuthorityOverride(t.Context(), subcapb.UpdateCertAuthorityOverrideRequest_builder{
					CaOverride: caOverride,
				}.Build())
				assertTestCae(t, tc, err)
			})
		}
		t.Run("upsert", func(t *testing.T) {
			_, err := subCA.UpsertCertAuthorityOverride(t.Context(), subcapb.UpsertCertAuthorityOverrideRequest_builder{
				CaOverride: caOverride,
			}.Build())
			assertTestCae(t, tc, err)
		})
	}

	t.Run("invalid cluster name", func(t *testing.T) {
		t.Parallel()

		// Fetch the certificate we want to override.
		const loadKeys = false
		ca, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
			Type:       caType,
			DomainName: env.ClusterName,
		}, loadKeys)
		require.NoError(t, err)
		require.Len(t, ca.GetActiveKeys().TLS, 1, "Unexpected number of CA active keys")
		kp := ca.GetActiveKeys().TLS[0]
		caCert, err := tlsutils.ParseCertificatePEM(kp.Cert)
		require.NoError(t, err)

		// Replace the cluster name in the cert with a bad name.
		const badClusterName = "badclustername"
		caCert.Subject.Organization = []string{badClusterName}

		// Create the badly-named override.
		ca.SetActiveKeys(types.CAKeySet{}) // No keyset = no overrides.
		caOverride := env.NewOverrideForCA(t, ca, nil /* externalRoot */)
		caOverride.GetMetadata().SetName(badClusterName)
		caOverride.GetSpec().SetCertificateOverrides([]*subcapb.CertificateOverride{
			env.NewDisabledCertificateOverride(t, caCert, nil /* externalRoot */),
		})

		runTestCase(t, &testCase{
			wantErr: `only "` + env.ClusterName + `" is allowed`,
		}, caOverride)
	})

	t.Run("override certificate lasts too long", func(t *testing.T) {
		t.Parallel()

		// Fetch the CA we want to override.
		const loadKeys = false
		ca, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
			Type:       caType,
			DomainName: env.ClusterName,
		}, loadKeys)
		require.NoError(t, err)
		require.Len(t, ca.GetActiveKeys().TLS, 1, "Unexpected number of CA active keys")
		kp := ca.GetActiveKeys().TLS[0]
		caCert, err := tlsutils.ParseCertificatePEM(kp.Cert)
		require.NoError(t, err)

		// Create an override that lasts more than the self-signed cert.
		overrideCA, err := env.ExternalRoot.NewIntermediateCA(&subcaenv.CAParams{
			Clock: env.Clock,
			Pub:   caCert.PublicKey,
			Template: &x509.Certificate{
				Subject: pkix.Name{
					Organization: []string{env.ClusterName},
				},
				NotAfter: caCert.NotAfter.Add(1 * time.Minute),
			},
		})
		require.NoError(t, err, "NewIntermediateCA errored")

		// Create an "empty" CA override...
		ca.SetActiveKeys(types.CAKeySet{})
		ca.SetAdditionalTrustedKeys(types.CAKeySet{})
		caOverride := env.NewOverrideForCA(t, ca, nil /* externalRoot */)
		// ...then add our override certificate to it.
		caOverride.GetSpec().SetCertificateOverrides([]*subcapb.CertificateOverride{
			subcapb.CertificateOverride_builder{
				Certificate: string(overrideCA.CertPEM),
			}.Build(),
		})

		runTestCase(t, &testCase{
			skipUpdate: true, // Update wants an existing resource
			wantErr:    "expires after self-signed CA certificate",
		}, caOverride)
	})

	tests := []testCase{
		{
			name: "nil ca_override",
			makeCAOverride: func(_ *subcapb.CertAuthorityOverride) *subcapb.CertAuthorityOverride {
				return nil
			},
			wantErr: "name required",
		},
		{
			name: "ca_override.metadata.name empty",
			makeCAOverride: func(caOverride *subcapb.CertAuthorityOverride) *subcapb.CertAuthorityOverride {
				caOverride.GetMetadata().SetName("")
				return caOverride
			},
			wantErr: "name required",
		},
		{
			name: "ca_override.sub_kind empty",
			makeCAOverride: func(caOverride *subcapb.CertAuthorityOverride) *subcapb.CertAuthorityOverride {
				caOverride.SetSubKind("")
				return caOverride
			},
			wantErr: "sub_kind required",
		},
		{
			name: "ca_override.sub_kind not allowed",
			makeCAOverride: func(caOverride *subcapb.CertAuthorityOverride) *subcapb.CertAuthorityOverride {
				caOverride.SetSubKind(string(types.UserCA))
				return caOverride
			},
			wantErr: "unsupported sub_kind/caType",
		},
		{
			name: "ca_override.sub_kind unknown",
			makeCAOverride: func(caOverride *subcapb.CertAuthorityOverride) *subcapb.CertAuthorityOverride {
				caOverride.SetSubKind("banana")
				return caOverride
			},
			wantErr: "unsupported sub_kind/caType",
		},
		{
			name: "new override targets unknown certificate",
			makeCAOverride: func(caOverride *subcapb.CertAuthorityOverride) *subcapb.CertAuthorityOverride {
				caOverride.GetSpec().SetCertificateOverrides(append(caOverride.GetSpec().GetCertificateOverrides(),
					subcapb.CertificateOverride_builder{
						PublicKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
						Disabled:  true,
					}.Build(),
				))
				return caOverride
			},
			skipUpdate: true, // Update wants an existing resource
			wantErr:    "targets unknown CA certificate",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runTestCase(t, &test, baseCAOverride)
		})
	}
}

// TestService_ForcedWrites tests writes that require
// force_immediate_disable/--force to succeed.
func TestService_ForcedWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		makeRPC  func(ctx context.Context, subCA subcapb.SubCAServiceClient, cao *subcapb.CertAuthorityOverride, force bool) error
		isDelete bool
	}{
		{
			name: "Update",
			makeRPC: func(ctx context.Context, subCA subcapb.SubCAServiceClient, cao *subcapb.CertAuthorityOverride, force bool) error {
				_, err := subCA.UpdateCertAuthorityOverride(ctx, subcapb.UpdateCertAuthorityOverrideRequest_builder{
					CaOverride:            cao,
					ForceImmediateDisable: force,
				}.Build())
				return err
			},
		},
		{
			name: "Upsert",
			makeRPC: func(ctx context.Context, subCA subcapb.SubCAServiceClient, cao *subcapb.CertAuthorityOverride, force bool) error {
				_, err := subCA.UpsertCertAuthorityOverride(ctx, subcapb.UpsertCertAuthorityOverrideRequest_builder{
					CaOverride:            cao,
					ForceImmediateDisable: force,
				}.Build())
				return err
			},
		},
		{
			name: "Delete",
			makeRPC: func(ctx context.Context, subCA subcapb.SubCAServiceClient, cao *subcapb.CertAuthorityOverride, force bool) error {
				_, err := subCA.DeleteCertAuthorityOverride(ctx, subcapb.DeleteCertAuthorityOverrideRequest_builder{
					CaId: subcapb.CertAuthorityOverrideID_builder{
						CaType: cao.GetSubKind(),
					}.Build(),
					ForceImmediateDelete: force,
				}.Build())
				return err
			},
			isDelete: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			const caType = types.WindowsCA
			env := subcav1.NewEnv(t, subcav1.EnvParams{
				StorageParams: subcaenv.EnvParams{
					CATypesToCreate: []types.CertAuthType{
						caType,
					},
				},
			})
			subCA := env.SubCAClient

			// Add additional CA keys for testing.
			key1PEM, cert1PEM, err := tlscatest.GenerateSelfSignedCA(tlscatest.GenerateCAConfig{
				ClusterName: env.ClusterName,
			})
			require.NoError(t, err, "GenerateSelfSignedCA errored")
			key2PEM, cert2PEM, err := tlscatest.GenerateSelfSignedCA(tlscatest.GenerateCAConfig{
				ClusterName: env.ClusterName,
			})
			require.NoError(t, err, "GenerateSelfSignedCA errored")
			const loadKeys = true
			ca, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
				Type:       caType,
				DomainName: env.ClusterName,
			}, loadKeys)
			require.NoError(t, err, "GetCertAuthority errored")
			additionalKeys := ca.GetAdditionalTrustedKeys()
			additionalKeys.TLS = append(additionalKeys.TLS,
				&types.TLSKeyPair{
					Cert:    cert1PEM,
					Key:     key1PEM,
					KeyType: types.PrivateKeyType_RAW,
				},
				&types.TLSKeyPair{
					Cert:    cert2PEM,
					Key:     key2PEM,
					KeyType: types.PrivateKeyType_RAW,
				},
			)
			ca.SetAdditionalTrustedKeys(additionalKeys)
			_, err = env.Trust.UpdateCertAuthority(t.Context(), ca)
			require.NoError(t, err, "UpdateCertAuthority errored")

			// Parse new CA certificates.
			cert1, err := tlsutils.ParseCertificatePEM(cert1PEM)
			require.NoError(t, err, "ParseCertificatePEM errored")
			cert2, err := tlsutils.ParseCertificatePEM(cert2PEM)
			require.NoError(t, err, "ParseCertificatePEM errored")

			// Prepare an all-enabled CA override.
			caOverride := env.NewOverrideForCAType(t, caType)
			caOverride.GetSpec().SetCertificateOverrides(append(caOverride.GetSpec().GetCertificateOverrides(),
				env.NewDisabledCertificateOverride(t, cert1, nil /* externalRoot */),
				env.NewDisabledCertificateOverride(t, cert2, nil /* externalRoot */),
			))
			for _, co := range caOverride.GetSpec().GetCertificateOverrides() {
				co.SetDisabled(false)
			}
			createResp, err := subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
				CaOverride: caOverride,
			}.Build())
			require.NoError(t, err, "Create errored")

			// Change override so it disables an enabled, active override.
			caOverride = createResp.GetCaOverride()
			caOverride.GetSpec().GetCertificateOverrides()[0].SetDisabled(true)

			// Attempt to write. It should fail because we are changing an enabled,
			// active override.
			const force = false
			require.ErrorContains(t,
				test.makeRPC(t.Context(), subCA, caOverride, force),
				"enabled override",
				"error mismatch",
			)

			t.Run("force", func(t *testing.T) {
				// Attempt forced write.
				const force = true
				require.NoError(t,
					test.makeRPC(t.Context(), subCA, caOverride, force),
					"unexpected error",
				)

				// Assert modification (Update/Upsert) or deletion.
				id := subcapb.CertAuthorityOverrideID_builder{
					CaType: string(caType),
				}.Build()
				getResp, err := subCA.GetCertAuthorityOverride(t.Context(), subcapb.GetCertAuthorityOverrideRequest_builder{
					CaId: id,
				}.Build())
				if test.isDelete {
					assert.ErrorAs(t, err, new(*trace.NotFoundError), "Get error mismatch")
					return
				}
				require.NoError(t, err, "Get errored")

				got := getResp.GetCaOverride()
				want := caOverride
				want.GetMetadata().SetRevision(got.GetMetadata().GetRevision())
				if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
					t.Errorf("CA override mismatch (-want +got)\n%s", diff)
				}

				// Show that additional keys can be disabled without forcing.
				t.Run("additional keys disable", func(t *testing.T) {
					caOverride := got
					for _, co := range caOverride.GetSpec().GetCertificateOverrides() {
						co.SetDisabled(true)
					}

					// Update.
					const force = false
					assert.NoError(t,
						test.makeRPC(t.Context(), subCA, caOverride, force),
					)

					// Verify update.
					getResp, err := subCA.GetCertAuthorityOverride(t.Context(), subcapb.GetCertAuthorityOverrideRequest_builder{
						CaId: id,
					}.Build())
					require.NoError(t, err)
					got := getResp.GetCaOverride()
					want := caOverride
					want.GetMetadata().SetRevision(got.GetMetadata().GetRevision())
					if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
						t.Errorf("CA override mismatch (-want +got)\n%s", diff)
					}
				})
			})
		})
	}
}

func TestService_List(t *testing.T) {
	t.Parallel()

	const caType1 = types.DatabaseClientCA
	const caType2 = types.WindowsCA
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{
				caType1,
				caType2,
			},
		},
	})
	subCA := env.SubCAClient

	t.Run("empty", func(t *testing.T) {
		// Don't t.Parallel(), can't race against override creation.

		got, err := subCA.ListCertAuthorityOverride(
			t.Context(), &subcapb.ListCertAuthorityOverrideRequest{})
		require.NoError(t, err, "List errored")

		// Verify empty response.
		want := &subcapb.ListCertAuthorityOverrideResponse{}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("List mismatch (-want +got)\n%s", diff)
		}
	})

	// Prepare overrides for testing.
	o1 := env.NewOverrideForCAType(t, caType1)
	resp, err := subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
		CaOverride: o1,
	}.Build())
	require.NoError(t, err, "Create errored")
	o1 = resp.GetCaOverride()

	o2 := env.NewOverrideForCAType(t, caType2)
	resp, err = subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
		CaOverride: o2,
	}.Build())
	require.NoError(t, err, "Create errored")
	o2 = resp.GetCaOverride()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		resp, err := subCA.ListCertAuthorityOverride(
			t.Context(), &subcapb.ListCertAuthorityOverrideRequest{})
		require.NoError(t, err, "List")
		assert.Empty(t, resp.GetNextPageToken(), "got non-empty nextPageToken")

		got := resp.GetCaOverrides()
		want := []*subcapb.CertAuthorityOverride{o1, o2}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("List mismatch (-want +got)\n%s", diff)
		}
	})
}

func TestService_Delete(t *testing.T) {
	t.Parallel()

	const caType = types.DatabaseClientCA
	const caTypeOther = types.WindowsCA

	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{
				caType,
			},
		},
	})
	subCA := env.SubCAClient

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		_, err := subCA.DeleteCertAuthorityOverride(t.Context(), subcapb.DeleteCertAuthorityOverrideRequest_builder{
			CaId: subcapb.CertAuthorityOverrideID_builder{
				CaType: string(caTypeOther),
			}.Build(),
		}.Build())
		assert.ErrorAs(t, err, new(*trace.NotFoundError), "Delete error mismatch")
	})

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		// Prepare override to delete.
		o := env.NewOverrideForCAType(t, caType)
		created, err := subCA.CreateCertAuthorityOverride(ctx, subcapb.CreateCertAuthorityOverrideRequest_builder{
			CaOverride: o,
		}.Build())
		require.NoError(t, err, "Create errored")

		emitter := env.MockEmitter
		emitter.Reset()

		// Delete.
		id := subcapb.CertAuthorityOverrideID_builder{
			CaType: created.GetCaOverride().GetSubKind(),
		}.Build()
		_, err = subCA.DeleteCertAuthorityOverride(ctx, subcapb.DeleteCertAuthorityOverrideRequest_builder{
			CaId: id,
		}.Build())
		require.NoError(t, err, "Delete errored")

		// Assert audit.
		assertCAOverrideEvent(t, emitter.Events(), &wantEvent{
			Type:    events.CertAuthOverrideDeleteEvent,
			Code:    events.CertAuthOverrideDeleteCode,
			Success: true,
		})

		t.Run("Get returns not found", func(t *testing.T) {
			t.Parallel()

			_, err := subCA.GetCertAuthorityOverride(ctx, subcapb.GetCertAuthorityOverrideRequest_builder{
				CaId: id,
			}.Build())
			assert.ErrorAs(t, err, new(*trace.NotFoundError), "Get error mismatch")
		})

		t.Run("double-Delete returns not found", func(t *testing.T) {
			t.Parallel()

			_, err := subCA.DeleteCertAuthorityOverride(ctx, subcapb.DeleteCertAuthorityOverrideRequest_builder{
				CaId: id,
			}.Build())
			assert.ErrorAs(t, err, new(*trace.NotFoundError), "Delete error mismatch")
		})
	})
}

type wantEvent struct {
	Code    string
	Type    string
	Success bool
}

func assertCAOverrideEvent(
	t *testing.T,
	events []apievents.AuditEvent,
	want *wantEvent,
) {
	t.Helper()
	require.Len(t, events, 1, "Number of audit events")

	e := events[0]
	require.IsType(t, &apievents.CertAuthorityOverrideEvent{}, e, "Event type mismatch")
	caoEvent := e.(*apievents.CertAuthorityOverrideEvent)

	assert.Equal(t, want.Type, caoEvent.Type, "Event.Type mismatch")
	assert.Equal(t, want.Code, caoEvent.Code, "Event.Code mismatch")
	assert.Equal(t, want.Success, caoEvent.Success, "Event.Success mismatch")

	wantName := caoEvent.CaOverride.CaType + "/" + caoEvent.CaOverride.ClusterName
	assert.Equal(t, wantName, caoEvent.Name, "Event.Name mismatch")
}
