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
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/e/lib/subca/subcav1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
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
	validLookingCAOverride := &subcapb.CertAuthorityOverride{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: caType,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: clusterName,
		},
		Spec: &subcapb.CertAuthorityOverrideSpec{
			CertificateOverrides: []*subcapb.CertificateOverride{
				nil, // Passes initial checks, but fails validation.
			},
		},
	}

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
					t.Context(), &subcapb.CreateCSRRequest{
						CaType: caType,
					})
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
					t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{
						CaOverride: validLookingCAOverride,
					})
				return err
			},
			want: []*authorizeAttempt{
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbCreate},
			},
		},
		{
			name: "GetCertAuthorityOverride",
			doRPC: func(t *testing.T) error {
				_, err := subCA.GetCertAuthorityOverride(t.Context(), &subcapb.GetCertAuthorityOverrideRequest{
					CaId: &subcapb.CertAuthorityOverrideID{
						CaType: caType,
					},
				})
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
	ca, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
		Type:       caType1,
		DomainName: env.ClusterName,
	}, loadKeys)
	require.NoError(t, err)
	require.Len(t, ca.GetActiveKeys().TLS, 1, "CA has an unexpected number of active keys")
	ca1Cert, err := tlsutils.ParseCertificatePEM(ca.GetActiveKeys().TLS[0].Cert)
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

	tests := []struct {
		name     string
		req      *subcapb.CreateCSRRequest
		wantCSRs func(t *testing.T) []*x509.CertificateRequest
	}{
		{
			name: "ok",
			req: &subcapb.CreateCSRRequest{
				CaType: string(caType1),
			},
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				return []*x509.CertificateRequest{
					newExpectedCSR(ca1Cert),
				}
			},
		},
		{
			name: "multiple CSRs",
			req: &subcapb.CreateCSRRequest{
				CaType: string(caType2),
			},
			wantCSRs: func(t *testing.T) []*x509.CertificateRequest {
				return []*x509.CertificateRequest{
					newExpectedCSR(ca2Cert1),
					newExpectedCSR(ca2Cert2),
					newExpectedCSR(ca2Cert3),
					newExpectedCSR(ca2Cert4),
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

func newExpectedCSR(caCert *x509.Certificate) *x509.CertificateRequest {
	return &x509.CertificateRequest{
		RawSubjectPublicKeyInfo: caCert.RawSubjectPublicKeyInfo,
		SignatureAlgorithm:      caCert.SignatureAlgorithm,
		PublicKeyAlgorithm:      caCert.PublicKeyAlgorithm,
		PublicKey:               caCert.PublicKey,
		Subject:                 csrSubjectFromCACert(caCert),
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
			name: "ca_type invalid",
			req: &subcapb.CreateCSRRequest{
				CaType: "bad-ca-type",
			},
			wantErr: "authority type is not supported",
		},
		{
			name: "public_key_hash not implemented",
			req: &subcapb.CreateCSRRequest{
				CaType: string(validCAType),
				PublicKeyHash: &subcapb.PublicKeyHash{
					Value: "f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				},
			},
			wantErr: "not implemented",
		},
		{
			name: "custom_subject not implemented",
			req: &subcapb.CreateCSRRequest{
				CaType: string(validCAType),
				CustomSubject: &subcapb.DistinguishedName{
					Names: []*subcapb.AttributeTypeAndValue{
						{
							Oid: []int32{2, 5, 4, 3}, // CN
							Value: func() *string {
								x := "Llama CA"
								return &x
							}(),
						},
					},
				},
			},
			wantErr: "not implemented",
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

	t.Run("nil resource", func(t *testing.T) {
		t.Parallel()

		_, err := subCA.CreateCertAuthorityOverride(t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{})
		assert.ErrorContains(t, err, "name required")
	})

	t.Run("invalid cluster name", func(t *testing.T) {
		t.Parallel()

		caOverride := env.NewOverrideForCAType(t, caType1)
		caOverride.Metadata.Name = "badclustername"

		_, err := subCA.CreateCertAuthorityOverride(t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{
			CaOverride: caOverride,
		})
		assert.ErrorContains(t, err, `only "`+env.ClusterName+`" is allowed`)
	})

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		caOverride := env.NewOverrideForCAType(t, caType2)

		// Create resource.
		createResp, err := subCA.CreateCertAuthorityOverride(t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{
			CaOverride: caOverride,
		})
		require.NoError(t, err, "CreateCertAuthorityOverride errored")

		// Assert resource.
		got := createResp.CaOverride
		want := caOverride
		want.Metadata.Revision = got.GetMetadata().GetRevision()
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Fatalf("Create mismatch (-want +got)\n%s", diff)
		}

		// Assert audit.
		emitter := env.MockEmitter
		assertCAOverrideEvent(t, emitter.Events(), &wantEvent{
			Type:    events.CertAuthOverrideCreateEvent,
			Code:    events.CertAuthOverrideCreateCode,
			Success: true,
		})

		// Assert storage.
		getResp, err := subCA.GetCertAuthorityOverride(t.Context(), &subcapb.GetCertAuthorityOverrideRequest{
			CaId: &subcapb.CertAuthorityOverrideID{
				CaType: got.SubKind,
			},
		})
		require.NoError(t, err, "GetCertAuthorityOverride errored")
		want = got
		got = getResp.CaOverride
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("Get mismatch (-want +got)\n%s", diff)
		}
	})
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
	csrResp, err := subCA.CreateCSR(t.Context(), &subcapb.CreateCSRRequest{
		CaType: string(caType),
	})
	require.NoError(t, err, "CreateCSR errored")
	require.Len(t, csrResp.GetCsrs(), 1, "CreateCSR returned an unexpected number of CSRs")

	// Create certificate, from CSR, using the external root.
	csr, err := tlsca.ParseCertificateRequestPEM([]byte(csrResp.GetCsrs()[0].GetPem()))
	require.NoError(t, err)
	now := env.Clock.Now()
	certDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		Subject:               csr.Subject,
		NotBefore:             now.Add(-1 * time.Minute),
		NotAfter:              now.Add(1 * time.Hour),
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
	caOverride := &subcapb.CertAuthorityOverride{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: string(caType),
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: env.ClusterName,
		},
		Spec: &subcapb.CertAuthorityOverrideSpec{
			CertificateOverrides: []*subcapb.CertificateOverride{
				{
					Certificate: string(certPEM),
				},
			},
		},
	}

	// Create override.
	_, err = subCA.CreateCertAuthorityOverride(
		t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{
			CaOverride: caOverride,
		})
	require.NoError(t, err, "Create errored")
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
	resp, err := subCA.CreateCertAuthorityOverride(t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{
		CaOverride: o1,
	})
	require.NoError(t, err, "Create errored")
	o1 = resp.CaOverride

	o2 := env.NewOverrideForCAType(t, caType2)
	resp, err = subCA.CreateCertAuthorityOverride(t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{
		CaOverride: o2,
	})
	require.NoError(t, err, "Create errored")
	o2 = resp.CaOverride

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		resp, err := subCA.ListCertAuthorityOverride(
			t.Context(), &subcapb.ListCertAuthorityOverrideRequest{})
		require.NoError(t, err, "List")
		assert.Empty(t, resp.NextPageToken, "got non-empty nextPageToken")

		got := resp.CaOverrides
		want := []*subcapb.CertAuthorityOverride{o1, o2}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("List mismatch (-want +got)\n%s", diff)
		}
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
}
