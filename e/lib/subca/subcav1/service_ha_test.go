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
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/subca/subcav1"
	"github.com/gravitational/teleport/lib/subca"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
)

// TestHACAOverrides tests CA override creation against an HA (High
// Availability), multi-HSM scenario.
func TestHACAOverrides(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		const numServices = 4 // Arbitrary.
		const caType = types.DatabaseClientCA
		haEnv := subcav1.NewHAEnv(t, subcav1.HAEnvParams{
			Storage: subcaenv.EnvParams{
				CATypesToCreate: []types.CertAuthType{
					caType,
				},
			},
			NumServices: numServices,
		})

		// RPCs go against only the first env, watchers take care of the rest.
		env := haEnv.Envs[0]
		clock := env.Clock
		subCA := env.SubCAClient

		// Wait for watchers to durably block, then sleep past the First time.
		synctest.Wait()
		time.Sleep(subcav1.WatcherFirstDuration)
		synctest.Wait()

		// Create CSRs for all keys/certificates.
		csrResp, err := env.SubCAClient.CreateCSR(t.Context(), subcapb.CreateCSRRequest_builder{
			CaType: string(caType),
		}.Build())
		require.NoError(t, err)
		require.Len(t, csrResp.GetCsrs(), numServices, "Unexpected number of CSRs")
		assert.Empty(t, csrResp.GetWarnings(), "CreateCSR returned unexpected warnings")

		// Verify PendingCSRRequest cleanup.
		{
			const pageSize = 0
			const pageToken = ""
			pendingReqs, nextPageToken, err := env.SubCA.ListPendingCSRRequests(t.Context(), pageSize, pageToken)
			require.NoError(t, err)
			assert.Empty(t, pendingReqs, "Expected no PendingCSRRequest instances, cleanup didn't happen.")
			assert.Empty(t, nextPageToken, "Unexpected non-empty nextPageToken")
		}

		// Prepare the CAOverride.
		var caOverride *subcapb.CertAuthorityOverride
		{
			now := env.Clock.Now()
			var certificateOverrides []*subcapb.CertificateOverride
			for _, csr := range csrResp.GetCsrs() {
				co := signCSRUsingExternalRoot(t, env.ExternalRoot, now, csr)
				certificateOverrides = append(certificateOverrides, co)
			}

			caOverride = subcapb.CertAuthorityOverride_builder{
				Kind:    types.KindCertAuthorityOverride,
				SubKind: string(caType),
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: env.ClusterName,
				}.Build(),
				Spec: subcapb.CertAuthorityOverrideSpec_builder{
					CertificateOverrides: certificateOverrides,
				}.Build(),
			}.Build()
			// Sanity check.
			require.Len(t, caOverride.GetSpec().GetCertificateOverrides(), numServices)
		}

		// Attempt to enable without all CRLs fails
		{
			setDisabled(caOverride, false)
			_, err := subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
				CaOverride: caOverride,
			}.Build())
			assert.ErrorContains(t, err, "cannot sign CRL for enabled override")

			setDisabled(caOverride, true)
		}

		// Disabled create is allowed, CRLs are created in background.
		createResp, err := subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
			CaOverride: caOverride,
		}.Build())
		require.NoError(t, err)
		caOverride = createResp.GetCaOverride()
		assert.Len(t, caOverride.GetStatus().GetPublicKeyHashToCrl(), 1,
			"Unexpected number of caOverride.Status.PublicKeyHashToCrl")

		// Wait for CRL creation. Services often hit CompareFailed errors, so we'll
		// accelerate time until they coalesce.
		const maxAttempts = numServices * 2 // Arbitrary. Make sure it stops at some point.
		for range maxAttempts {
			synctest.Wait()
			time.Sleep(subcav1.ConditionalUpdateMaxStep)
			synctest.Wait()

			getResp, err := subCA.GetCertAuthorityOverride(t.Context(), subcapb.GetCertAuthorityOverrideRequest_builder{
				CaId: subcapb.CertAuthorityOverrideID_builder{
					CaType: caOverride.GetSubKind(),
				}.Build(),
			}.Build())
			require.NoError(t, err)
			caOverride = getResp.GetCaOverride()
			if len(caOverride.GetStatus().GetPublicKeyHashToCrl()) == numServices {
				break
			}
		}
		assert.Len(t, caOverride.GetStatus().GetPublicKeyHashToCrl(), numServices,
			"Unexpected number of caOverride.Status.PublicKeyHashToCrl, wait loop reached maxAttempts",
		)

		assertCRLs(t, caOverride, clock.Now())
	})
}

func setDisabled(caOverride *subcapb.CertAuthorityOverride, disabled bool) {
	for _, co := range caOverride.GetSpec().GetCertificateOverrides() {
		co.SetDisabled(disabled)
	}
}

func TestService_crlsCreatedOnStartup(t *testing.T) {
	t.Parallel()

	// synctest is used so watcher initialization delays use synthetic time.
	synctest.Test(t, func(t *testing.T) {
		const caType = types.DatabaseClientCA
		storageEnv := subcaenv.New(t, subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{caType},
		})

		// Prepare an override for caType. It doesn't have CRLs, as it's being
		// directly created at the storage layer, bypassing service/gRPC.
		created, err := storageEnv.SubCA.CreateCertAuthorityOverride(t.Context(), storageEnv.NewOverrideForCAType(t, caType))
		require.NoError(t, err)
		// Sanity check.
		require.Empty(t, created.GetStatus().GetPublicKeyHashToCrl())

		// Start a new service. CRLs are created during watcher initialization.
		env := subcav1.NewEnv(t, subcav1.EnvParams{
			StorageEnv: storageEnv,
		})

		// Wait for watchers to initialize and become quiescent.
		time.Sleep(subcav1.WatcherFirstDuration)
		synctest.Wait()

		// Verify.
		got, err := env.SubCAClient.GetCertAuthorityOverride(t.Context(), subcapb.GetCertAuthorityOverrideRequest_builder{
			CaId: subcapb.CertAuthorityOverrideID_builder{
				CaType: string(caType),
			}.Build(),
		}.Build())
		require.NoError(t, err)
		require.NotEmpty(t, got.GetCaOverride().GetStatus().GetPublicKeyHashToCrl(), "CAOverride lacks CRLs")
		assertCRLs(t, got.GetCaOverride(), env.Clock.Now())
	})
}

func TestService_CreateCSR_HA(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		const caType = types.DatabaseClientCA
		const numServices = 3
		haEnv := subcav1.NewHAEnv(t, subcav1.HAEnvParams{
			Storage: subcaenv.EnvParams{
				CATypesToCreate: []types.CertAuthType{
					caType,
				},
			},
			NumServices: numServices,
		})
		subCA := haEnv.Envs[0].SubCAClient

		env1 := haEnv.Envs[1]
		env1Credential := haEnv.CATypeToCredentials[caType][1]
		env1PublicKey := env1Credential.PublicKeyHash

		// Wait for watchers to be ready.
		synctest.Wait()
		time.Sleep(subcav1.WatcherFirstDuration)

		const noKeyAccessMessage = "lacks access to private keys"
		{
			t.Log("test/fails if local can't access keys")
			_, err := haEnv.Envs[0].SubCAClient.CreateCSR(t.Context(), subcapb.CreateCSRRequest_builder{
				CaType: string(caType),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: env1PublicKey,
				}.Build(),
				LocalOnly: true,
			}.Build())
			assert.ErrorContains(t, err, noKeyAccessMessage)
		}

		{
			t.Log("test/fails if remote returns no keys")

			// Swap env1's KeystoreManager for a failing one.
			subcav1.SetFailingSigner(t, env1, true)
			synctest.Wait()

			_, err := haEnv.Envs[0].SubCAClient.CreateCSR(t.Context(), subcapb.CreateCSRRequest_builder{
				CaType: string(caType),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: env1PublicKey,
				}.Build(),
			}.Build())
			subcav1.SetFailingSigner(t, env1, false)
			assert.ErrorContains(t, err, noKeyAccessMessage)
		}

		{
			t.Log("test/all keys")

			csrResp, err := subCA.CreateCSR(t.Context(), subcapb.CreateCSRRequest_builder{
				CaType: string(caType),
			}.Build())
			require.NoError(t, err)
			require.Len(t, csrResp.GetCsrs(), numServices, "CreateCSR returned an unexpected number of CSRs")
			assert.Empty(t, csrResp.GetWarnings(), "CreateCSR returned unexpected warnings")

			// Verify that CSRs are valid.
			creds := haEnv.CATypeToCredentials[caType]
			seenPubKeys := make(map[string]struct{})
			for _, csrPB := range csrResp.GetCsrs() {
				// Verify signature.
				csr := parseCSR(t, csrPB)
				require.NoError(t, csr.CheckSignature(), "csr: check signature")

				// Verify issuer.
				gotPubKey := subca.HashPublicKey(csr.RawSubjectPublicKeyInfo)
				seenPubKeys[gotPubKey] = struct{}{}
				var issuer *subcav1.HAEnvCredential
				for _, cred := range creds {
					if cred.PublicKeyHash == gotPubKey {
						issuer = cred
						break
					}
				}
				require.NotNil(t, issuer, "csr has unexpected public key hash: %q", gotPubKey)
			}
			require.Len(t, seenPubKeys, numServices, "CreateCSR did not sign CSRs with all public keys")
		}

		{
			t.Log("test/targeted key")

			csrResp, err := subCA.CreateCSR(t.Context(), subcapb.CreateCSRRequest_builder{
				CaType: string(caType),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: env1PublicKey,
				}.Build(),
			}.Build())
			require.NoError(t, err)
			require.Len(t, csrResp.GetCsrs(), 1, "CreateCSR returned an unexpected number of CSRs")
			assert.Empty(t, csrResp.GetWarnings(), "CreateCSR returned unexpected warnings")

			validateCSR(t, env1Credential, csrResp.GetCsrs()[0])
		}

		{
			t.Log("test/custom subject")

			llamaCA := "Llama CA"
			csrResp, err := subCA.CreateCSR(t.Context(), subcapb.CreateCSRRequest_builder{
				CaType: string(caType),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: env1PublicKey,
				}.Build(),
				CustomSubject: subcapb.DistinguishedName_builder{
					Names: []*subcapb.AttributeTypeAndValue{
						subcapb.AttributeTypeAndValue_builder{
							Oid:   []int32{2, 5, 4, 3}, // CN
							Value: &llamaCA,
						}.Build(),
					},
				}.Build(),
			}.Build())
			require.NoError(t, err)
			require.Len(t, csrResp.GetCsrs(), 1, "CreateCSR returned an unexpected number of CSRs")
			assert.Empty(t, csrResp.GetWarnings(), "CreateCSR returned unexpected warnings")

			csr := validateCSR(t, env1Credential, csrResp.GetCsrs()[0])
			want := pkix.RDNSequence{
				{{Type: asn1.ObjectIdentifier{2, 5, 4, 10}, Value: env1.ClusterName}}, // O
				{{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: llamaCA}},           // CN
			}
			if diff := cmp.Diff(want, csr.Subject.ToRDNSequence()); diff != "" {
				t.Errorf("CSR Subject mismatch (-want +got)\n%s", diff)
			}
		}

		{
			t.Log("test/returns successful CSRs")

			// Swap env1's KeystoreManager for a failing one.
			subcav1.SetFailingSigner(t, env1, true)
			synctest.Wait()

			csrResp, err := subCA.CreateCSR(t.Context(), subcapb.CreateCSRRequest_builder{
				CaType: string(caType),
			}.Build())
			subcav1.SetFailingSigner(t, env1, false) // Restore signer
			require.NoError(t, err)

			// CreateCSR should return all successful CSRs.
			// * env0: signs locally
			// * env1: fails
			// * env2+: signs remotely
			assert.Len(t, csrResp.GetCsrs(), numServices-1, "CreateCSR returned an unexpected number of CSRs")

			// Assert warning.
			require.Len(t, csrResp.GetWarnings(), 1, "CreateCSR returned an unexpected number of warnings")
			warn := csrResp.GetWarnings()[0]
			assert.Contains(t, warn.GetUserMessage(), subcav1.ErrFailingSigner.Error(),
				"CreateCSRResponse.warnings[0]: user message mismatch")
			want := subcapb.CreateCSRWarning_builder{
				UserMessage:   warn.GetUserMessage(),
				PublicKeyHash: env1PublicKey,
			}.Build()
			if diff := cmp.Diff(want, warn, protocmp.Transform()); diff != "" {
				t.Errorf("CreateCSRResponse.warnings[0] mismatch (-want +got)\n%s", diff)
			}
		}

		{
			t.Log("test/timeout and startup requests")

			env1.Stop()
			synctest.Wait()

			req := subcapb.CreateCSRRequest_builder{
				CaType: string(caType),
			}.Build()

			// 1. A missing Auth times out the request and returns a warning.
			csrResp, err := subCA.CreateCSR(t.Context(), req)
			require.NoError(t, err)
			assert.Len(t, csrResp.GetCsrs(), numServices-1, "CreateCSR returned an unexpected number of CSRs")
			require.Len(t, csrResp.GetWarnings(), 1, "CreateCSR returned an unexpected number of warnings")

			// Verify warning.
			warn := csrResp.GetWarnings()[0]
			assert.Contains(t, warn.GetUserMessage(), "not fulfilled before timeout")
			want := subcapb.CreateCSRWarning_builder{
				UserMessage:   warn.GetUserMessage(),
				PublicKeyHash: env1PublicKey,
			}.Build()
			if diff := cmp.Diff(warn, want, protocmp.Transform()); diff != "" {
				t.Errorf("CreateCSRResponse.warnings[0] mismatch (-want +got)\n%s", diff)
			}

			// 2. A pending request is fulfilled by a starting server.
			csrRespC := make(chan *subcapb.CreateCSRResponse, 1)
			go func() {
				csrResp, err := subCA.CreateCSR(t.Context(), req)
				csrRespC <- csrResp
				assert.NoError(t, err)
			}()

			// Wait for the request above to block.
			synctest.Wait()

			// "Restart" the missing Auth.
			env1 = subcav1.NewEnv(t, subcav1.EnvParams{
				StorageEnv:      env1.Env,
				KeystoreManager: env1.KeystoreManager,
			})
			haEnv.Envs[1] = env1

			// The request is fulfilled by the started Auth.
			csrResp = <-csrRespC
			require.NotNil(t, csrResp, "async CreateCSR RPC failed")
			require.Len(t, csrResp.GetCsrs(), numServices, "CreateCSR returned an unexpected number of CSRs")
			assert.Empty(t, csrResp.GetWarnings(), "CreateCSR returned unexpected warnings")
		}
	})
}

func validateCSR(
	t *testing.T,
	issuer *subcav1.HAEnvCredential,
	csrPB *subcapb.CertificateSigningRequest,
) *x509.CertificateRequest {
	t.Helper()

	csr := parseCSR(t, csrPB)
	require.NoError(t, csr.CheckSignature(), "csr: check signature")
	gotPub := subca.HashPublicKey(csr.RawSubjectPublicKeyInfo)
	require.Equal(t, issuer.PublicKeyHash, gotPub, "csr: public key hash")
	return csr
}

func parseCSR(t *testing.T, csrPB *subcapb.CertificateSigningRequest) *x509.CertificateRequest {
	t.Helper()

	block, _ := pem.Decode([]byte(csrPB.GetPem()))
	require.NotNil(t, block)
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	require.NoError(t, err)
	return csr
}
