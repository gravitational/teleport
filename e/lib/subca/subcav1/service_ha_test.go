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
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
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

		caOverride := env.NewOverrideForCAType(t, caType)
		// Sanity check.
		require.Len(t, caOverride.GetSpec().GetCertificateOverrides(), numServices,
			"Unexpected number of caOverride.Spec.CertificateOverrides")

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
		env0 := haEnv.Envs[0]
		env1 := haEnv.Envs[1]

		env1Credential := haEnv.CATypeToCredentials[caType][1]
		env1PublicKey := env1Credential.PublicKeyHash

		// TODO(codingllama): Replace createCSR by its namesake RPC once the
		//  CSR-watcher logic is introduced.
		createCSR := func(ctx context.Context, req *subcapb.CreateCSRRequest) (*subcapb.CreateCSRResponse, error) {
			// Simulate CreateCSR by creating the PendingCSRRequest entity directly.

			var pkhs []*subcapb.PublicKeyHash
			if req.HasPublicKeyHash() {
				pkhs = append(pkhs, req.GetPublicKeyHash())
			} else {
				for _, cred := range haEnv.CATypeToCredentials[caType] {
					pkhs = append(pkhs, subcapb.PublicKeyHash_builder{
						Value: cred.PublicKeyHash,
					}.Build())
				}
			}

			created, err := env0.SubCA.CreatePendingCSRRequest(ctx, subcapb.PendingCSRRequest_builder{
				Kind:    types.KindPendingCSRRequest,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.NewString(),
				}.Build(),
				Spec: subcapb.PendingCSRRequestSpec_builder{
					ClusterName:     env0.ClusterName,
					CaType:          req.GetCaType(),
					CustomSubject:   req.GetCustomSubject(),
					PublicKeyHashes: pkhs,
				}.Build(),
			}.Build())
			if err != nil {
				return nil, fmt.Errorf("create csr req: %w", err)
			}

			// Wait for the PendingCSRRequest to be fulfilled.
			var csrReq *subcapb.PendingCSRRequest
			var csrReqDone bool
			const maxAttempts = numServices * 2 // Arbitrary. Make sure it stops at some point.
			for range maxAttempts {
				synctest.Wait()
				time.Sleep(subcav1.ConditionalUpdateMaxStep)

				var err error
				csrReq, err = env0.SubCA.GetPendingCSRRequest(ctx, created.GetMetadata().GetName())
				if err != nil {
					return nil, fmt.Errorf("read csr req: %w", err)
				}
				csrReqDone = len(csrReq.GetSpec().GetPublicKeyHashes()) == len(csrReq.GetStatus().GetPublicKeyHashToPendingCsr())
				if csrReqDone {
					break
				}
			}
			if !csrReqDone {
				return nil, errors.New("pending CSR request not fulfilled after max iterations")
			}

			// Mimic the CreateCSRResponse.
			var respBuilder subcapb.CreateCSRResponse_builder
			csrMap := csrReq.GetStatus().GetPublicKeyHashToPendingCsr()
			for k, v := range csrMap {
				st := v.GetStatus()
				if codes.Code(st.GetCode()) == codes.OK {
					respBuilder.Csrs = append(respBuilder.Csrs, v.GetCsr())
					continue
				}
				respBuilder.Warnings = append(respBuilder.Warnings, subcapb.CreateCSRWarning_builder{
					UserMessage:   st.GetMessage(),
					PublicKeyHash: k,
				}.Build())
			}
			return respBuilder.Build(), nil
		}

		// Wait for watchers to be ready.
		synctest.Wait()
		time.Sleep(subcav1.WatcherFirstDuration)

		{
			t.Log("test/all keys")

			csrResp, err := createCSR(t.Context(), subcapb.CreateCSRRequest_builder{
				CaType: string(caType),
			}.Build())
			require.NoError(t, err)
			require.Len(t, csrResp.GetCsrs(), numServices, "CreateCSR returned an unexpected number of CSRs")

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

			csrResp, err := createCSR(t.Context(), subcapb.CreateCSRRequest_builder{
				CaType: string(caType),
				PublicKeyHash: subcapb.PublicKeyHash_builder{
					Value: env1PublicKey,
				}.Build(),
			}.Build())
			require.NoError(t, err)
			require.Len(t, csrResp.GetCsrs(), 1, "CreateCSR returned an unexpected number of CSRs")

			validateCSR(t, env1Credential, csrResp.GetCsrs()[0])
		}

		{
			t.Log("test/custom subject")

			llamaCA := "Llama CA"
			csrResp, err := createCSR(t.Context(), subcapb.CreateCSRRequest_builder{
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

			csr := validateCSR(t, env1Credential, csrResp.GetCsrs()[0])
			assert.Equal(t, llamaCA, csr.Subject.CommonName, "csr.Subject.CommonName mismatch")
		}

		{
			t.Log("test/returns successful CSRs")

			// Swap env1's KeystoreManager for a failing one.
			subcav1.SetFailingSigner(t, env1, true)
			synctest.Wait()

			csrResp, err := createCSR(t.Context(), subcapb.CreateCSRRequest_builder{
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
