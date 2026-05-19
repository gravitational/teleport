// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package tpmjoin_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"log/slog"
	"math/big"
	"testing"
	"time"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/tpm"
)

func TestJoinTPM(t *testing.T) {
	server, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)

	adminClient, err := server.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	_, err = adminClient.BotServiceClient().CreateBot(t.Context(), &machineidv1.CreateBotRequest{
		Bot: &machineidv1.Bot{
			Metadata: &headerv1.Metadata{
				Name: "testbot",
			},
			Kind: types.KindBot,
			Spec: &machineidv1.BotSpec{},
		},
	})
	require.NoError(t, err)

	nopClient, err := server.NewClient(authtest.TestNop())
	require.NoError(t, err)

	goodTPMKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	goodTPMPub, err := x509.MarshalPKIXPublicKey(goodTPMKey.Public())
	require.NoError(t, err)
	goodTPMPubHash := tpm.HashEKPub(goodTPMPub)

	badTPMKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	fakeTPMValidator := newFakeTPMValidator()
	server.Auth().SetTPMValidator(fakeTPMValidator.validate)

	goodTPMCA, err := newFakeTPMCA()
	require.NoError(t, err)
	badTPMCA, err := newFakeTPMCA()
	require.NoError(t, err)

	tpmCert1, tpmCertSerial1, err := goodTPMCA.issueTPMCert(goodTPMKey.Public())
	require.NoError(t, err)
	tpmCert2, _, err := goodTPMCA.issueTPMCert(goodTPMKey.Public())
	require.NoError(t, err)

	allowRulesNotMatched := func(t require.TestingT, err error, i ...any) {
		require.ErrorContains(t, err, "validated tpm attributes did not match any allow rules")
		require.True(t, trace.IsAccessDenied(err))
	}

	for _, tc := range []struct {
		desc            string
		tokenSpec       *types.ProvisionTokenSpecV2TPM
		tpmKey          crypto.Signer
		tpmCert         []byte
		badTPMSolution  bool
		oss             bool
		assertError     require.ErrorAssertionFunc
		expectJoinAttrs verifiedAttrs
	}{
		{
			desc: "success, ekpub",
			tokenSpec: &types.ProvisionTokenSpecV2TPM{
				Allow: []*types.ProvisionTokenSpecV2TPM_Rule{
					{
						EKPublicHash: goodTPMPubHash,
					},
				},
			},
			tpmKey:      goodTPMKey,
			assertError: require.NoError,
			expectJoinAttrs: verifiedAttrs{
				ekPubHash: goodTPMPubHash,
			},
		},
		{
			desc: "success, ekcert",
			tokenSpec: &types.ProvisionTokenSpecV2TPM{
				Allow: []*types.ProvisionTokenSpecV2TPM_Rule{
					{
						EKCertificateSerial: tpmCertSerial1,
					},
				},
			},
			tpmKey:      goodTPMKey,
			tpmCert:     tpmCert1,
			assertError: require.NoError,
			expectJoinAttrs: verifiedAttrs{
				ekPubHash:    goodTPMPubHash,
				ekCertSerial: tpmCertSerial1,
			},
		},
		{
			desc: "success, both ek cert serial and ek pub hash match",
			tokenSpec: &types.ProvisionTokenSpecV2TPM{
				Allow: []*types.ProvisionTokenSpecV2TPM_Rule{
					{
						EKPublicHash:        goodTPMPubHash,
						EKCertificateSerial: tpmCertSerial1,
					},
				},
			},
			tpmKey:      goodTPMKey,
			tpmCert:     tpmCert1,
			assertError: require.NoError,
			expectJoinAttrs: verifiedAttrs{
				ekPubHash:    goodTPMPubHash,
				ekCertSerial: tpmCertSerial1,
			},
		},
		{
			desc: "success, ek cert verified",
			tokenSpec: &types.ProvisionTokenSpecV2TPM{
				EKCertAllowedCAs: []string{string(goodTPMCA.caCertPEM)},
				Allow: []*types.ProvisionTokenSpecV2TPM_Rule{
					{
						EKCertificateSerial: tpmCertSerial1,
					},
				},
			},
			tpmKey:      goodTPMKey,
			tpmCert:     tpmCert1,
			assertError: require.NoError,
			expectJoinAttrs: verifiedAttrs{
				ekPubHash:      goodTPMPubHash,
				ekCertSerial:   tpmCertSerial1,
				ekCertVerified: true,
			},
		},
		{
			desc: "success, ek cert verified and ek pub hash match",
			tokenSpec: &types.ProvisionTokenSpecV2TPM{
				EKCertAllowedCAs: []string{string(goodTPMCA.caCertPEM)},
				Allow: []*types.ProvisionTokenSpecV2TPM_Rule{
					{
						EKPublicHash:        goodTPMPubHash,
						EKCertificateSerial: tpmCertSerial1,
					},
				},
			},
			tpmKey:      goodTPMKey,
			tpmCert:     tpmCert1,
			assertError: require.NoError,
			expectJoinAttrs: verifiedAttrs{
				ekPubHash:      goodTPMPubHash,
				ekCertSerial:   tpmCertSerial1,
				ekCertVerified: true,
			},
		},
		{
			desc: "failure, mismatched ekpub",
			tokenSpec: &types.ProvisionTokenSpecV2TPM{
				Allow: []*types.ProvisionTokenSpecV2TPM_Rule{
					{
						EKPublicHash: goodTPMPubHash,
					},
				},
			},
			// TPM key does not match pubkey hash in token.
			tpmKey:      badTPMKey,
			assertError: allowRulesNotMatched,
		},
		{
			desc: "failure, mismatched ekcert serial",
			tokenSpec: &types.ProvisionTokenSpecV2TPM{
				Allow: []*types.ProvisionTokenSpecV2TPM_Rule{
					{
						EKCertificateSerial: tpmCertSerial1,
					},
				},
			},
			tpmKey: goodTPMKey,
			// TPM cert does not match serial in token.
			tpmCert:     tpmCert2,
			assertError: allowRulesNotMatched,
		},
		{
			desc: "failure, ek cert not verified",
			tokenSpec: &types.ProvisionTokenSpecV2TPM{
				// Token configures trust for a CA that did not sign the TPM cert.
				EKCertAllowedCAs: []string{string(badTPMCA.caCertPEM)},
				Allow: []*types.ProvisionTokenSpecV2TPM_Rule{
					{
						EKCertificateSerial: tpmCertSerial1,
					},
				},
			},
			tpmKey:  goodTPMKey,
			tpmCert: tpmCert1,
			assertError: func(t require.TestingT, err error, msgAndArgs ...any) {
				require.ErrorAs(t, err, (new(*trace.AccessDeniedError)))
				require.ErrorContains(t, err, "certificate signed by unknown authority")
			},
		},
		{
			desc: "failure, solution mismatch",
			tokenSpec: &types.ProvisionTokenSpecV2TPM{
				Allow: []*types.ProvisionTokenSpecV2TPM_Rule{
					{
						EKPublicHash: goodTPMPubHash,
					},
				},
			},
			tpmKey:         goodTPMKey,
			badTPMSolution: true,
			assertError: func(t require.TestingT, err error, msgAndArgs ...any) {
				require.ErrorAs(t, err, (new(*trace.AccessDeniedError)))
				require.ErrorContains(t, err, "invalid credential activation solution")
			},
		},
		{
			desc: "failure, oss",
			tokenSpec: &types.ProvisionTokenSpecV2TPM{
				Allow: []*types.ProvisionTokenSpecV2TPM_Rule{
					{
						EKPublicHash: goodTPMPubHash,
					},
				},
			},
			tpmKey: goodTPMKey,
			oss:    true,
			assertError: func(t require.TestingT, err error, msgAndArgs ...any) {
				require.ErrorAs(t, err, (new(*trace.AccessDeniedError)))
				require.ErrorContains(t, err, "this feature requires Teleport Enterprise")
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			if !tc.oss {
				modulestest.SetTestModules(t, modulestest.Modules{TestBuildType: modules.BuildEnterprise})
			}

			token, err := types.NewProvisionTokenFromSpec("mytoken", time.Now().Add(time.Minute), types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTPM,
				Roles:      []types.SystemRole{types.RoleBot},
				BotName:    "testbot",
				TPM:        tc.tokenSpec,
			})
			require.NoError(t, err)
			require.NoError(t, server.Auth().UpsertToken(t.Context(), token))

			fakeTPM, err := newFakeTPM(tc.tpmKey, tc.tpmCert)
			require.NoError(t, err)
			fakeTPM.badSolution = tc.badTPMSolution

			checkResult := func(t *testing.T, result *joinclient.JoinResult) {
				t.Helper()

				botCert, err := tlsca.ParseCertificatePEM(result.Certs.TLS)
				require.NoError(t, err)

				id, err := tlsca.FromSubject(botCert.Subject, botCert.NotAfter)
				require.NoError(t, err)
				tpmAttrs := id.JoinAttributes.Tpm
				require.NotNil(t, tpmAttrs)
				gotAttrs := verifiedAttrs{
					ekPubHash:      tpmAttrs.EkPubHash,
					ekCertSerial:   tpmAttrs.EkCertSerial,
					ekCertVerified: tpmAttrs.EkCertVerified,
				}
				assert.Equal(t, tc.expectJoinAttrs, gotAttrs)
			}

			t.Run("legacy", func(t *testing.T) {
				result, err := joinclient.LegacyJoin(t.Context(), joinclient.JoinParams{
					Token:      token.GetName(),
					JoinMethod: types.JoinMethodTPM,
					ID: state.IdentityID{
						Role: types.RoleBot,
					},
					AuthClient: nopClient,
					AttestTPM:  fakeTPM.attest,
				})
				tc.assertError(t, err)
				if err != nil {
					return
				}
				checkResult(t, result)
			})
			t.Run("new", func(t *testing.T) {
				result, err := joinclient.Join(t.Context(), joinclient.JoinParams{
					Token: token.GetName(),
					ID: state.IdentityID{
						Role: types.RoleBot,
					},
					AuthClient: nopClient,
					AttestTPM:  fakeTPM.attest,
				})
				tc.assertError(t, err)
				if err != nil {
					return
				}
				checkResult(t, result)
			})
		})
	}

}

// fakeTPM is a minimal faked TPM that will return attestation parameters for a
// key and cert it is configured with. It returns the constant "good-solution"
// for all ec solutions, unless badSolution is set.
type fakeTPM struct {
	ekKey     crypto.Signer
	ekPub     []byte
	ekPubHash string

	ekCert []byte

	badSolution bool
}

func newFakeTPM(ekKey crypto.Signer, ekCert []byte) (*fakeTPM, error) {
	ekPub, err := x509.MarshalPKIXPublicKey(ekKey.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ekPubHash := tpm.HashEKPub(ekPub)

	return &fakeTPM{
		ekKey:     ekKey,
		ekPub:     ekPub,
		ekPubHash: ekPubHash,
		ekCert:    ekCert,
	}, nil
}

func (f *fakeTPM) attest(ctx context.Context, _ *slog.Logger) (*tpm.Attestation, func() error, error) {
	close := func() error {
		return nil
	}
	solve := func(*attest.EncryptedCredential) ([]byte, error) {
		if f.badSolution {
			return []byte("bad-solution"), nil
		}
		return []byte("good-solution"), nil
	}
	data := tpm.QueryRes{
		EKPub:     f.ekPub,
		EKPubHash: f.ekPubHash,
	}
	if f.ekCert != nil {
		cert, err := x509.ParseCertificate(f.ekCert)
		if err != nil {
			return nil, close, trace.Wrap(err)
		}
		data.EKCert = &tpm.QueryEKCert{
			Raw:          f.ekCert,
			SerialNumber: tpm.SerialString(cert.SerialNumber),
		}
	}
	return &tpm.Attestation{
		Data: data,
		AttestParams: attest.AttestationParameters{
			Public: f.ekPub,
		},
		Solve: solve,
	}, close, nil
}

// fakeTPMValidator is a minimal fakes TPM validator. It always issues empty
// EncryptedCredential challenges and expects the solution to be
// "good-solution", but it will legitimately validate certificates to their
// issuing CA.
type fakeTPMValidator struct{}

func newFakeTPMValidator() *fakeTPMValidator {
	return &fakeTPMValidator{}
}

func (f *fakeTPMValidator) validate(ctx context.Context, params tpm.ValidateParams) (*tpm.ValidatedTPM, error) {
	ec := &attest.EncryptedCredential{}
	clientSolution, err := params.Solve(ec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if string(clientSolution) != "good-solution" {
		return nil, trace.BadParameter("invalid credential activation solution")
	}

	validated := &tpm.ValidatedTPM{}

	var ekCert *x509.Certificate
	if params.EKCert != nil {
		ekCert, err = x509.ParseCertificate(params.EKCert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		validated.EKCertSerial = tpm.SerialString(ekCert.SerialNumber)
		ekPubPKIX, err := x509.MarshalPKIXPublicKey(ekCert.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		validated.EKPubHash = tpm.HashEKPub(ekPubPKIX)
	}
	if params.AllowedCAs != nil {
		if ekCert == nil {
			return nil, trace.BadParameter("tpm did not provide an EKCert to verify")
		}
		if _, err := ekCert.Verify(x509.VerifyOptions{
			Roots: params.AllowedCAs,
		}); err != nil {
			return nil, trace.Wrap(err, "verifying EKCert")
		}
		validated.EKCertVerified = true
	}
	if params.EKKey != nil {
		validated.EKPubHash = tpm.HashEKPub(params.EKKey)
	}

	return validated, nil
}

// fakeTPMCA issues fake TPM certificates.
type fakeTPMCA struct {
	caKey     crypto.Signer
	caCert    *x509.Certificate
	caCertPEM []byte
	serial    int64
}

func newFakeTPMCA() (*fakeTPMCA, error) {
	caKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caCertPEM, err := tlsca.GenerateSelfSignedCAWithSigner(caKey, pkix.Name{CommonName: "Test TPM CA"}, nil, time.Hour)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caCert, err := tlsca.ParseCertificatePEM(caCertPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &fakeTPMCA{
		caKey:     caKey,
		caCert:    caCert,
		caCertPEM: caCertPEM,
	}, nil
}

func (f *fakeTPMCA) issueTPMCert(pub crypto.PublicKey) ([]byte, string, error) {
	f.serial++
	cert := &x509.Certificate{
		SerialNumber:          big.NewInt(f.serial),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		Subject: pkix.Name{
			CommonName: "testtpm",
		},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, cert, f.caCert, pub, f.caKey)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return certDER, tpm.SerialString(cert.SerialNumber), nil
}

type verifiedAttrs struct {
	ekPubHash      string
	ekCertSerial   string
	ekCertVerified bool
}
