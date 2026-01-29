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

package oraclejoin_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/join/joinclient/oracle"
	"github.com/gravitational/teleport/lib/join/jointest"
	"github.com/gravitational/teleport/lib/join/joinutils"
	"github.com/gravitational/teleport/lib/join/oraclejoin"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

// TestJoinOracle tests the Oracle join method, with faked OCI IMDS and API servers.
func TestJoinOracle(t *testing.T) {
	t.Parallel()

	imdsListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	fakeIMDS, err := newFakeIMDS()
	require.NoError(t, err)

	testutils.RunTestBackgroundTask(t.Context(), t, &testutils.TestBackgroundTask{
		Name: "fake IMDS server",
		Task: func(ctx context.Context) error {
			err := fakeIMDS.serve(imdsListener)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return trace.Wrap(err)
		},
		Terminate: imdsListener.Close,
	})

	oracleAPIListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	fakeOracleAPI := newFakeOracleAPI(fakeIMDS.cas.caCertBase64)

	testutils.RunTestBackgroundTask(t.Context(), t, &testutils.TestBackgroundTask{
		Name: "fake Oracle API server",
		Task: func(ctx context.Context) error {
			err := fakeOracleAPI.serve(oracleAPIListener)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return trace.Wrap(err)
		},
		Terminate: oracleAPIListener.Close,
	})

	fakeOracleAPIClient, err := newFakeHTTPClient(oracleAPIListener.Addr())
	require.NoError(t, err)

	server, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
		TLS: &authtest.TLSServerConfig{
			APIConfig: &auth.APIConfig{
				OracleHTTPClient: fakeOracleAPIClient,
			},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, server.Shutdown(t.Context())) })

	nopClient, err := server.NewClient(authtest.TestNop())
	require.NoError(t, err)

	isAccessDenied := func(t assert.TestingT, err error, msgAndArgs ...any) bool {
		return assert.ErrorAs(t, err, new(*trace.AccessDeniedError), msgAndArgs...)
	}

	for _, tc := range []struct {
		desc             string
		claims           oraclejoin.Claims
		allowRules       []*types.ProvisionTokenSpecV2Oracle_Rule
		tokenName        string
		requestTokenName string
		instanceKeyAlg   cryptosuites.Algorithm
		assertion        assert.ErrorAssertionFunc
	}{
		{
			desc: "allow tenant",
			claims: oraclejoin.Claims{
				InstanceID:    makeInstanceID("phx", "myinstance"),
				CompartmentID: makeCompartmentID("mycompartment"),
				TenancyID:     makeTenancyID("mytenant"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{Tenancy: makeTenancyID("mytenant")},
			},
			tokenName:        "mytoken",
			requestTokenName: "mytoken",
			assertion:        assert.NoError,
		},
		{
			desc: "allow tenant,compartment",
			claims: oraclejoin.Claims{
				InstanceID:    makeInstanceID("phx", "myinstance"),
				CompartmentID: makeCompartmentID("mycompartment"),
				TenancyID:     makeTenancyID("mytenant"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy: makeTenancyID("mytenant"),
					ParentCompartments: []string{
						makeCompartmentID("othercompartment"),
						makeCompartmentID("mycompartment"),
					},
				},
			},
			tokenName:        "mytoken",
			requestTokenName: "mytoken",
			assertion:        assert.NoError,
		},
		{
			desc: "allow tenant,compartment,region",
			claims: oraclejoin.Claims{
				InstanceID:    makeInstanceID("phx", "myinstance"),
				CompartmentID: makeCompartmentID("mycompartment"),
				TenancyID:     makeTenancyID("mytenant"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy: makeTenancyID("mytenant"),
					ParentCompartments: []string{
						makeCompartmentID("othercompartment"),
						makeCompartmentID("mycompartment"),
					},
					Regions: []string{"otherregion", "phx"},
				},
			},
			tokenName:        "mytoken",
			requestTokenName: "mytoken",
			assertion:        assert.NoError,
		},
		{
			desc: "allow tenant,compartment,region,instance",
			claims: oraclejoin.Claims{
				InstanceID:    makeInstanceID("phx", "myinstance"),
				CompartmentID: makeCompartmentID("mycompartment"),
				TenancyID:     makeTenancyID("mytenant"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy: makeTenancyID("mytenant"),
					ParentCompartments: []string{
						makeCompartmentID("othercompartment"),
						makeCompartmentID("mycompartment"),
					},
					Regions: []string{"otherregion", "phx"},
					Instances: []string{
						makeInstanceID("phx", "otherinstance"),
						makeInstanceID("phx", "myinstance"),
					},
				},
			},
			tokenName:        "mytoken",
			requestTokenName: "mytoken",
			assertion:        assert.NoError,
		},
		{
			desc: "allow multiple rules",
			claims: oraclejoin.Claims{
				InstanceID:    makeInstanceID("phx", "myinstance"),
				CompartmentID: makeCompartmentID("mycompartment"),
				TenancyID:     makeTenancyID("mytenant"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy: makeTenancyID("othertenant"),
					ParentCompartments: []string{
						makeCompartmentID("othercompartment"),
					},
				},
				{
					Tenancy: makeTenancyID("mytenant"),
					ParentCompartments: []string{
						makeCompartmentID("othercompartment"),
						makeCompartmentID("mycompartment"),
					},
				},
			},
			tokenName:        "mytoken",
			requestTokenName: "mytoken",
			assertion:        assert.NoError,
		},
		{
			desc: "wrong token",
			claims: oraclejoin.Claims{
				InstanceID:    makeInstanceID("phx", "myinstance"),
				CompartmentID: makeCompartmentID("mycompartment"),
				TenancyID:     makeTenancyID("mytenant"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{Tenancy: makeTenancyID("mytenant")},
			},
			tokenName:        "mytoken",
			requestTokenName: "badtoken",
			assertion:        isAccessDenied,
		},
		{
			desc: "wrong tenant",
			claims: oraclejoin.Claims{
				InstanceID:    makeInstanceID("phx", "myinstance"),
				CompartmentID: makeCompartmentID("mycompartment"),
				TenancyID:     makeTenancyID("badtenant"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{Tenancy: makeTenancyID("mytenant")},
			},
			tokenName:        "mytoken",
			requestTokenName: "mytoken",
			assertion:        isAccessDenied,
		},
		{
			desc: "wrong compartment",
			claims: oraclejoin.Claims{
				InstanceID:    makeInstanceID("phx", "myinstance"),
				CompartmentID: makeCompartmentID("badcompartment"),
				TenancyID:     makeTenancyID("mytenant"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy: makeTenancyID("mytenant"),
					ParentCompartments: []string{
						makeCompartmentID("othercompartment"),
						makeCompartmentID("mycompartment"),
					},
				},
			},
			tokenName:        "mytoken",
			requestTokenName: "mytoken",
			assertion:        isAccessDenied,
		},
		{
			desc: "wrong region",
			claims: oraclejoin.Claims{
				InstanceID:    makeInstanceID("badregion", "myinstance"),
				CompartmentID: makeCompartmentID("mycompartment"),
				TenancyID:     makeTenancyID("mytenant"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy: makeTenancyID("mytenant"),
					ParentCompartments: []string{
						makeCompartmentID("othercompartment"),
						makeCompartmentID("mycompartment"),
					},
					Regions: []string{"otherregion", "phx"},
				},
			},
			tokenName:        "mytoken",
			requestTokenName: "mytoken",
			assertion:        isAccessDenied,
		},
		{
			desc: "wrong instance",
			claims: oraclejoin.Claims{
				InstanceID:    makeInstanceID("phx", "badinstance"),
				CompartmentID: makeCompartmentID("mycompartment"),
				TenancyID:     makeTenancyID("mytenant"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy: makeTenancyID("mytenant"),
					ParentCompartments: []string{
						makeCompartmentID("othercompartment"),
						makeCompartmentID("mycompartment"),
					},
					Regions: []string{"otherregion", "phx"},
					Instances: []string{
						makeInstanceID("phx", "otherinstance"),
						makeInstanceID("phx", "myinstance"),
					},
				},
			},
			tokenName:        "mytoken",
			requestTokenName: "mytoken",
			assertion:        isAccessDenied,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fakeIMDS.instanceClaims = tc.claims

			imdsClient, err := newFakeHTTPClient(imdsListener.Addr())
			require.NoError(t, err)

			spec := types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodOracle,
				Roles:      []types.SystemRole{types.RoleNode},
				Oracle: &types.ProvisionTokenSpecV2Oracle{
					Allow: tc.allowRules,
				},
			}
			token, err := types.NewProvisionTokenFromSpec(
				tc.tokenName,
				time.Now().Add(time.Minute),
				spec,
			)
			require.NoError(t, err)
			require.NoError(t, server.Auth().UpsertToken(t.Context(), token))

			scopedToken, err := jointest.ScopedTokenFromProvisionTokenSpec(spec, &joiningv1.ScopedToken{
				Scope: "/test",
				Metadata: &headerv1.Metadata{
					Name: "scoped_" + token.GetName(),
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/test/one",
					UsageMode:     string(joining.TokenUsageModeUnlimited),
				},
			})
			require.NoError(t, err)
			_, err = server.Auth().CreateScopedToken(t.Context(), &joiningv1.CreateScopedTokenRequest{
				Token: scopedToken,
			})
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err := server.Auth().DeleteScopedToken(t.Context(), &joiningv1.DeleteScopedTokenRequest{
					Name: scopedToken.GetMetadata().GetName(),
				})
				require.NoError(t, err)
			})

			t.Run("unscoped", func(t *testing.T) {
				_, err = joinclient.Join(t.Context(), joinclient.JoinParams{
					Token: tc.requestTokenName,
					ID: state.IdentityID{
						Role: types.RoleInstance,
					},
					AuthClient:       nopClient,
					OracleIMDSClient: imdsClient,
				})
				tc.assertion(t, err)
			})

			t.Run("scoped", func(t *testing.T) {
				_, err = joinclient.Join(t.Context(), joinclient.JoinParams{
					Token: "scoped_" + tc.requestTokenName,
					ID: state.IdentityID{
						Role: types.RoleInstance,
					},
					AuthClient:       nopClient,
					OracleIMDSClient: imdsClient,
				})
				tc.assertion(t, err)
			})
		})
	}
}

// TestInstanceKeyAlgorithms tests that the Oracle join method supports
// multiple possible instance key algorithms and checks their parameters.
func TestInstanceKeyAlgorithms(t *testing.T) {
	t.Parallel()

	cas, err := newCAChain()
	require.NoError(t, err)

	oracleAPIListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	fakeOracleAPI := newFakeOracleAPI(cas.caCertBase64)

	testutils.RunTestBackgroundTask(t.Context(), t, &testutils.TestBackgroundTask{
		Name: "fake Oracle API server",
		Task: func(ctx context.Context) error {
			err := fakeOracleAPI.serve(oracleAPIListener)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return trace.Wrap(err)
		},
		Terminate: oracleAPIListener.Close,
	})

	fakeOracleAPIClient, err := newFakeHTTPClient(oracleAPIListener.Addr())
	require.NoError(t, err)

	claims := oraclejoin.Claims{
		TenancyID:     makeTenancyID("mytenant"),
		CompartmentID: makeCompartmentID("mycompartment"),
		InstanceID:    makeInstanceID("phx", "myinstance"),
	}

	token, err := types.NewProvisionTokenFromSpec(
		"mytoken",
		time.Now().Add(time.Minute),
		types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodOracle,
			Roles:      []types.SystemRole{types.RoleNode},
			Oracle: &types.ProvisionTokenSpecV2Oracle{
				Allow: []*types.ProvisionTokenSpecV2Oracle_Rule{
					{Tenancy: claims.TenancyID},
				},
			},
		},
	)
	require.NoError(t, err)

	challenge, err := joinutils.GenerateChallenge(base64.StdEncoding, 32)
	require.NoError(t, err)

	makeChallengeSolution := func(t *testing.T, instanceKey, signatureKey crypto.Signer) *messages.OracleChallengeSolution {
		instanceCert, err := cas.issueInstanceCert(claims, instanceKey.Public())
		require.NoError(t, err)

		var signature []byte
		switch signatureKey.Public().(type) {
		case *rsa.PublicKey:
			signature, err = oracle.SignChallenge(signatureKey, challenge)
		case *ecdsa.PublicKey:
			signature, err = crypto.SignMessage(signatureKey, rand.Reader, []byte(challenge), crypto.SHA256)
		case ed25519.PublicKey:
			signature, err = crypto.SignMessage(signatureKey, rand.Reader, []byte(challenge), crypto.Hash(0))
		}
		require.NoError(t, err)

		// Make the root CA request but there's no need to actually sign it
		// since this will be sent to the test's fake Oracle API.
		rootCAReq := &http.Request{
			Method: http.MethodGet,
			URL: &url.URL{
				Scheme: "http",
				Host:   "auth.us-phoenix-1.oraclecloud.com",
				Path:   "/v1/instancePrincipalRootCACertificates",
			},
			Header: http.Header{
				"Date": []string{time.Now().UTC().Format(http.TimeFormat)},
			},
		}
		rootCAReqBytes, err := httputil.DumpRequestOut(rootCAReq, false /*body*/)
		require.NoError(t, err)

		return &messages.OracleChallengeSolution{
			Cert:            instanceCert,
			Intermediate:    cas.intermediateCertPEM,
			Signature:       signature,
			SignedRootCAReq: rootCAReqBytes,
		}
	}

	ecdsaP256Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	isAccessDenied := func(t assert.TestingT, err error, msgAndArgs ...any) bool {
		return assert.ErrorAs(t, err, new(*trace.AccessDeniedError), msgAndArgs...)
	}

	rootCACache := oraclejoin.NewRootCACache()

	for _, tc := range []struct {
		desc         string
		instanceKey  crypto.Signer
		signatureKey crypto.Signer
		badSignature bool
		assertion    assert.ErrorAssertionFunc
	}{
		{
			// RSA key is too small.
			desc:        "rsa1024",
			instanceKey: parseTestKey(rsa1024Key),
			assertion:   isAccessDenied,
		},
		{
			// RSA key pass.
			desc:        "rsa2048",
			instanceKey: parseTestKey(rsa2048Key1),
			assertion:   assert.NoError,
		},
		{
			// ECDSA key rejected.
			desc:        "ecdsaP256",
			instanceKey: ecdsaP256Key,
			assertion:   isAccessDenied,
		},
		{
			// Ed25519 key rejected.
			desc:        "ed25519",
			instanceKey: ed25519Key,
			assertion:   isAccessDenied,
		},
		{
			desc:         "RSA key bad signature",
			instanceKey:  parseTestKey(rsa2048Key1),
			badSignature: true,
			assertion:    isAccessDenied,
		},
		{
			desc:         "challenge signed by different key",
			instanceKey:  parseTestKey(rsa2048Key1),
			signatureKey: parseTestKey(rsa2048Key2),
			assertion:    isAccessDenied,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.signatureKey == nil {
				tc.signatureKey = tc.instanceKey
			}
			solution := makeChallengeSolution(t, tc.instanceKey, tc.signatureKey)

			if tc.badSignature {
				solution.Signature[0] = solution.Signature[0] + 1
			}

			params := &oraclejoin.CheckChallengeSolutionParams{
				Challenge:      challenge,
				Solution:       solution,
				ProvisionToken: token,
				HTTPClient:     fakeOracleAPIClient,
				RootCACache:    rootCACache,
			}

			_, err = oraclejoin.CheckChallengeSolution(t.Context(), params)
			tc.assertion(t, err)
		})
	}
}

// fakeHTTPClient overrides the address and scheme of HTTP requests to direct
// them to a faked server for a test.
type fakeHTTPClient struct {
	addr net.Addr
	utils.HTTPDoClient
}

func newFakeHTTPClient(addr net.Addr) (*fakeHTTPClient, error) {
	clt, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &fakeHTTPClient{
		addr:         addr,
		HTTPDoClient: clt,
	}, nil
}

func (c *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	slog.InfoContext(req.Context(), "HTTP request",
		"method", req.Method,
		"url", req.URL)
	req.URL.Host = c.addr.String()
	req.URL.Scheme = "http"
	return c.HTTPDoClient.Do(req)
}

// fakeIMDS implements the subset of the HTTP interface of the OCI IMDS that is
// used for Oracle joining.
type fakeIMDS struct {
	cas            *caChain
	instanceKey    crypto.Signer
	instanceClaims oraclejoin.Claims
}

func newFakeIMDS() (*fakeIMDS, error) {
	cas, err := newCAChain()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	instanceKey := parseTestKey(rsa2048Key1)
	f := &fakeIMDS{
		cas:         cas,
		instanceKey: instanceKey,
	}
	return f, nil
}

func (f *fakeIMDS) serve(lis net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/opc/v2/instance/region", f.handleRegion)
	mux.HandleFunc("/opc/v2/identity/cert.pem", f.handleCert)
	mux.HandleFunc("/opc/v2/identity/key.pem", f.handleKey)
	mux.HandleFunc("/opc/v2/identity/intermediate.pem", f.handleIntermediate)
	mux.HandleFunc("/v1/x509", f.handleX509)
	return trace.Wrap(http.Serve(lis, mux))
}

func (f *fakeIMDS) handleRegion(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("phx"))
}

func (f *fakeIMDS) handleCert(w http.ResponseWriter, r *http.Request) {
	instanceCert, err := f.cas.issueInstanceCert(f.instanceClaims, f.instanceKey.Public())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Write(instanceCert)
}

func (f *fakeIMDS) handleKey(w http.ResponseWriter, r *http.Request) {
	keyPEM, err := keys.MarshalPrivateKey(f.instanceKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Write(keyPEM)
}

func (f *fakeIMDS) handleIntermediate(w http.ResponseWriter, r *http.Request) {
	w.Write(f.cas.intermediateCertPEM)
}

func (f *fakeIMDS) handleX509(w http.ResponseWriter, r *http.Request) {
	type token struct {
		Token string `json:"token"`
	}
	resp := token{
		Token: testJWT,
	}
	json.NewEncoder(w).Encode(resp)
}

// fakeOracleAPI implements the subset of the HTTP interface of the OCI auth
// API that is used for Oracle joining.
type fakeOracleAPI struct {
	rootCABase64 string
}

func newFakeOracleAPI(rootCABase64 string) *fakeOracleAPI {
	return &fakeOracleAPI{
		rootCABase64: rootCABase64,
	}
}

func (f *fakeOracleAPI) serve(lis net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/instancePrincipalRootCACertificates", f.handleRootCACertificates)
	return trace.Wrap(http.Serve(lis, mux))
}

func (f *fakeOracleAPI) handleRootCACertificates(w http.ResponseWriter, r *http.Request) {
	type rootCAResp struct {
		Certificates []string `json:"certificates"`
		RefreshIn    string   `json:"refreshIn"`
	}
	resp := rootCAResp{
		Certificates: []string{f.rootCABase64},
		RefreshIn:    time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	json.NewEncoder(w).Encode(resp)
}

type caChain struct {
	caCertPEM           []byte
	caCertBase64        string
	intermediateKey     crypto.Signer
	intermediateCert    *x509.Certificate
	intermediateCertPEM []byte
}

func newCAChain() (*caChain, error) {
	caKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caCertPEM, err := tlsca.GenerateSelfSignedCAWithSigner(caKey, pkix.Name{CommonName: "root CA"}, nil, time.Hour)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caCert, err := tlsca.ParseCertificatePEM(caCertPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caCertBase64 := strings.TrimSpace(string(caCertPEM))
	caCertBase64 = strings.TrimPrefix(caCertBase64, "-----BEGIN CERTIFICATE-----\n")
	caCertBase64 = strings.TrimSuffix(caCertBase64, "\n-----END CERTIFICATE-----")

	intermediateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	intermediateCert := newX509Cert()
	intermediateCert.Subject = pkix.Name{CommonName: "intermediate CA"}
	intermediateCert.IsCA = true
	intermediateCertDER, err := x509.CreateCertificate(rand.Reader, intermediateCert, caCert, intermediateKey.Public(), caKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	intermediateCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intermediateCertDER})

	return &caChain{
		caCertPEM:           caCertPEM,
		caCertBase64:        caCertBase64,
		intermediateKey:     intermediateKey,
		intermediateCert:    intermediateCert,
		intermediateCertPEM: intermediateCertPEM,
	}, nil
}

func (c *caChain) issueInstanceCert(claims oraclejoin.Claims, pub crypto.PublicKey) ([]byte, error) {
	instanceCert := newX509Cert()
	instanceCert.Subject = pkix.Name{
		CommonName: "instance",
		OrganizationalUnit: []string{
			"opc-instance:" + claims.InstanceID,
			"opc-compartment:" + claims.CompartmentID,
			"opc-tenant:" + claims.TenancyID,
			"opc-certtype:instance",
		},
	}
	instanceCertDER, err := x509.CreateCertificate(rand.Reader, instanceCert, c.intermediateCert, pub, c.intermediateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: instanceCertDER}), nil
}

func newX509Cert() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
}

const (
	// Generating RSA keys slow so these test use pre-generated keys.
	rsa2048Key1 = string(`-----BEGIN TEST RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAxYsUIrplZRkTS30Zh1O0DVVr40KVky+3EZKiH8OWKaVGlqVk
pmggHpw8pCQwAmxFqLoQxOntCch4p0Vfk/BahtAq0NZ+4qSGX8BIv48ESPudIAY7
iDdNUn+2Xd1Z2lzHMJ9tKUtceVmTlDpOFjXDPxFK2cUg4dY4l6PgM4lJq2kS/Sbd
9YIR/+ekCStKzXt5qziuwMx+tSxDmZ2DbQkdhCyStKNhouyDPxDB6LBLqlU9IZb8
1NpoDaaZ+rv5MmD7hb5Yb6qSuT56zxygd+ae4Qzr7y7/aYqhUK210GsnvHoyK4hF
QP8sAWg0ljgu6lgisjphZdUJ4lSo8wE5SkfCDQIDAQABAoIBAAGeXx7duiD28KKI
tuHV/L6zOXwWOpWHKY/aTLvH5X4X3Zk0Z7u5VLILg6+woDgU3QlB5QtIA2o2G077
kYnryUIbiI5Hg6ilwngcYjw3lshmT2ZIxsoZ8edAJqVkP+07H2K1m7Zf6LUR19S6
GZOzAxOMN7nLFLblA3eynw6tDE58PTtQFZ/DGiCGi7Ac9mKc7EQ1juF8J+ZpzoHC
fvx+huRDu7Bu0jlI+mYNcZgbDDynQxyPUOHCf3pk07DThccz1gZjCC0HyfjUjuYM
PaAIHg+8cnrMHqd6+PL7zhjyH8pdiH1BTy/3R+ovYr4HiGugVq/onwVszXHwoGG9
C1XyrH0CgYEA71xgZH+Bfb27AdeGm+N/6Dd54hDlfXD27F6zrp9qeqeVztSHH1/2
mD/MxsvniXB7LJ5sWrbNz55Y7uycUAD1sfDKXoRZ8c2V6tpqde22jrQhpgbJ8uvx
r2EYksBz7iV8rtfx/KMBk+sf6t+DZnztzENmmsh2HamuXo2br9cEEaMCgYEA00aG
JaGDuwzv/O8RnXpIYMNxDQYr8jd6Bt7nkZcHVMdQtX6dnUPRXHFHAnpke9BCGiG2
GjciaUpzSfgXvsbRMpfJR5hhyhml1MqtjguAlbpDXSaPQwcSTfbZvxHEV2PToXYB
z+AII4tpsXD5N22HvreeapXTyBz3Lhhw8WEk+I8CgYBPUSgsBUiOt1GB4b6cZ73Z
4JBGBl1VvRpF53fZVMA/FsuAt1JzZiRb/UBJXAZEt/5JIdI8GTmIJCvKOKPvqvG/
3k/hFDCN/RdBtND0dSo6jZxc3QEMu3ziJeWzs4x3DPsNIUfx9L4wGwj/lsN/McTH
HEqi3eyuFa1PbdN6aGDTywKBgQC1L1bdsLyyze6FwFQf8/1cFl++JpvLdi4M9F4s
6hNcbi3V6AatFrrWB0M5adMAp2H43Q45Py0glLt4JO3gKsq/E5KG9rRuSD6B1Wqv
VUfpn7ojiWz0s3zMJbUo+ciilTap0fTN27e/G9EBXfwrv5/ZO8j8aQ8dH1IPUuCQ
8JlvGwKBgGlRuhLXrP3Cg0rhaupR0gUZPjPs9AQiGFs186oCYs6PKSFJ/LekbNuq
uOuLdl2Bwlo6XFlT7I4FfAgzO+l35ER2Gihr8hIb8cC1dBAIR1ejXUhoC18NN8GJ
As9vlQ+UkBUy4/0pl7S/Eet7CI0WSwalvjd7bI2Xiau5rO3aT5pQ
-----END TEST RSA PRIVATE KEY-----`)
	rsa2048Key2 = string(`-----BEGIN TEST RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAzsP7Kl+O4JUq5lqGTbbnIDFCJttQq4t/aRHF4LXXrby7JBvG
HekpgRqflEw7U356iVf7GZoSq8XpNsUiWJ1M0sl9z49qmrsNzj9dwfd8dLN6XxJ5
qnECL6SuHx/vwkaseppYQLXZijRQGKOj9h1s/tn8UjabmL9APC9TiYkSzkwsdRyR
xXc9lwQQKRC/AjFKi1DWxAPw0DT1P+guvWZBk8Wwnqcs2uP4OUKoZ9GULD9Jf22Q
kVlU4vkEp5VOaudv08y5enLhRRxLTzSDD0mctRHi+DnNApG2CiNdONkYm6rB5HBD
6GYRd/kn+F0EZ93XEdf+gkMfY78PeHQFGi6T0QIDAQABAoIBAAwmPU/WZDPzf+ns
mTMS046AM0XLCEWNzlngEGA6WMCENC9HrhxobKbOtJOUGr5QUo0kc53XAFB1ZVU+
P/aBOhgYSXS7SOvSBvPyCqVqaeoF+Ja1vefTReockDPfyzvl4QRy2LGz6pYk3Pn9
qS8KhP+N00/Vyc6WM+uPA/OACVaNM/1p+N/g5/W4zs3GQ8ssSBn/CMFaFeMeVwMk
94Mj1UMXSfH6ORgnQtVeFSI/Fmx6QcHVoXqF2OWMcYth4XQHZtl7C+gMGSzSpIzn
CURyAu1Ze70PbdWLPGisXV5GdFIBV6uMJRE9oj0vZqxTOChGY8n30elA0HUWUg2o
CKKBqysCgYEA9xohrszEgZSkHuGNdpVt30AtzI+w9Ig6EtUGgDzp+PU0ossDQVQ/
9/z5EinFs2e8I8hwxjy+u8OQdo43BGQ1t6AkQrCVH6aJXGvILK7so2yycwNPChS/
hjweGV1fcr7kwn4jl9BRiKsLYLtn/bLX+Y9RiXumK1bMB7X2HSxUw7MCgYEA1jYD
n2FE8YkUy0nSWpqkNZt8hb3HJzL265liYRTJxS8o0iAGjiVaZAfDG8gIhdkYXIh5
P+FBmlAbhVecJrStO9RPGTpVVd0ntm6phOK4oqodWjV4I/f7GbT1yvpQFFus+hvP
WNTUvlPKPe7K932d9m25Qs9mxeh7InV1QEn4GGsCgYEAk+UPDek/H/OQO29yVOxh
A4MNJmdGWUWDxKu9pVlQDJLuexUZEKvVUZ8WkDlyO8u1vpEEdpH68rS9LUg3Q6ia
whnWOhgoWPY7NpbIC35y4el38QCk+PqsGzK2LSZGr43zqzkGIqIreqotOCtStXSq
cZLHEYtxTHU5zs+oy5Mx9KMCgYEAyAQDeeyPPaEsI2241xUSQ2P977t2m+mAmhjM
va11gYM5cIqq1EuYjVKaIfSz0JcXoj9kR/uDEB3AtM9LZPDL2NOzT/EiAVzRWg0W
iJhSosCJS9QlbCB+/E/2OiNkZr37VEZnY6DHTThb3Vx9dH584r8tf269ngon/9MB
OphW6iUCgYB05lF3Smdw2mrJ25RrjNFhxY2zmgBAQBIEm/M736j4U/evLaJ/1ViV
6cEjOsAJh6CeHsTQmJqf6AOOYqg0I1yK+YHBdu4H6Rwk0uWNEWPIc7h2m3TjKVUv
8JBWbnqcO00Vy0o9FwEp7zAtW7RWLz730sTJ0WVnsVt5D2CXuqj2gw==
-----END TEST RSA PRIVATE KEY-----`)
	rsa1024Key = string(`-----BEGIN TEST RSA PRIVATE KEY-----
MIICXQIBAAKBgQDQ5Ez3fC5iVdHJyvzTX9p7GUyJOArMM/VJjMwbsln2hcBCq65s
SRDfBPmgR9xz2rnvInJ0yGuPKT7g/VAP+J05EnU4uywXlua0ciXQk5GRWKFIbJfJ
0RktwlOjchy2xwIyuZHPN40jHilNyZkJX90FCxXHnkm3/b6yc+kzkDAv4wIDAQAB
AoGAMYiZeawgQaAxDYVNW4AkykDvBbDc2pxNg2HYOo8ZxxvjQcv9Id9XmVLQMMIp
k+1fXsXP10J5QurYZriaphbhjOvSEhtsbw/HujHbxnOTaZtZNnfdmWWswO6JMtG2
7wjs/McCM0XfjlO94xn4vTw2ajhxfaWsvDpMXm74SMq2Y6kCQQDih+wYUxG6zEAB
y2wgSnQNHGFKTHAWgVskdy+cX99YXUlIqFE9HV/MS+5rzgejwLG1VSOrcsoZb6/p
AKBESr/LAkEA7BDyxG1t3gi/JrG2C0CjM06DAl5Og7Gq0BoWw26QyA0u5EPmZmM1
CW3V+ZTjp974/ZwaBEc8c5CzZnugVcCdSQJBAJ3cJnTU/pfz2e7mOVVPTQwN6OaD
2eB1CHSi8fTBAr1rVLRjRymVnLqbd2x8yOoeUDiTOiYx+hA7upResVCl3n0CQEXd
bjv8Nvvzkr8c8Ue7RZG1tshIqOwI9QjJ79q/KlJKtIoSHmpHCjdULnPDQO057G8C
eCC0BIwfUzkNdZJrgyECQQDH1ZQu7J5Ol67adPPY//RLLFagKWH34Gb2XNT3HSMR
JnKk+4N1PAV4hzYeOX0J+Tl6ugUyMCXQ9u62gw+a80Xn
-----END TEST RSA PRIVATE KEY-----
`)
)

func parseTestKey(keyPEM string) crypto.Signer {
	key, err := keys.ParsePrivateKey([]byte(strings.ReplaceAll(keyPEM, "TEST ", "")))
	if err != nil {
		panic(err)
	}
	return key.Signer
}

const testJWT = `eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJ0ZXN0IiwiaWF0IjoxNzYxODY4ODAwLCJleHAiOjMzMzE4Nzc3NjAwLCJhdWQiOiJ0ZWxlcG9ydC5leGFtcGxlLmNvbSIsInN1YiI6InRlc3QifQ.JRMX4mUsTbBQWNCz5HI543ZIaqMUPfVhQ7qWgg8l3cY`
