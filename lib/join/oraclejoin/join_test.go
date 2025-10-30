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
	"github.com/gravitational/teleport/lib/join/joinutils"
	"github.com/gravitational/teleport/lib/join/oraclejoin"
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
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fakeIMDS.instanceClaims = tc.claims

			imdsClient, err := newFakeHTTPClient(imdsListener.Addr())
			require.NoError(t, err)

			token, err := types.NewProvisionTokenFromSpec(
				tc.tokenName,
				time.Now().Add(time.Minute),
				types.ProvisionTokenSpecV2{
					JoinMethod: types.JoinMethodOracle,
					Roles:      []types.SystemRole{types.RoleNode},
					Oracle: &types.ProvisionTokenSpecV2Oracle{
						Allow: tc.allowRules,
					},
				},
			)
			require.NoError(t, err)
			require.NoError(t, server.Auth().UpsertToken(t.Context(), token))

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

	makeChallengeSolution := func(t *testing.T, instanceKey crypto.Signer) *messages.OracleChallengeSolution {
		instanceCert, err := cas.issueInstanceCert(claims, instanceKey.Public())
		require.NoError(t, err)

		var signature []byte
		switch instanceKey.Public().(type) {
		case *rsa.PublicKey:
			signature, err = oracle.SignChallenge(instanceKey, challenge)
		case *ecdsa.PublicKey:
			signature, err = crypto.SignMessage(instanceKey, rand.Reader, []byte(challenge), crypto.SHA256)
		case ed25519.PublicKey:
			signature, err = crypto.SignMessage(instanceKey, rand.Reader, []byte(challenge), crypto.Hash(0))
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

	rsa1024Key, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)

	ecdsaP256Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	isAccessDenied := func(t assert.TestingT, err error, msgAndArgs ...any) bool {
		return assert.ErrorAs(t, err, new(*trace.AccessDeniedError), msgAndArgs...)
	}

	for _, tc := range []struct {
		desc         string
		instanceKey  crypto.Signer
		badSignature bool
		assertion    assert.ErrorAssertionFunc
	}{
		{
			// RSA key is too small.
			desc:        "rsa1024",
			instanceKey: rsa1024Key,
			assertion:   isAccessDenied,
		},
		{
			// RSA key pass.
			desc:        "rsa2048",
			instanceKey: rsa2048Key(),
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
			instanceKey:  rsa2048Key(),
			badSignature: true,
			assertion:    isAccessDenied,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			solution := makeChallengeSolution(t, tc.instanceKey)

			if tc.badSignature {
				solution.Signature[0] = solution.Signature[0] + 1
			}

			params := &oraclejoin.CheckChallengeSolutionParams{
				Challenge:      challenge,
				Solution:       solution,
				ProvisionToken: token,
				HTTPClient:     fakeOracleAPIClient,
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
	caCert         []byte
	instanceKey    crypto.Signer
	instanceClaims oraclejoin.Claims
}

func newFakeIMDS() (*fakeIMDS, error) {
	cas, err := newCAChain()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	instanceKey := rsa2048Key()
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

// rsa2048Key returns a pre-generated 2048-bit RSA key (generating this key in
// the test would be slow).
func rsa2048Key() crypto.Signer {
	const keyPemFixture = string(`-----BEGIN TEST RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`)
	key, err := keys.ParsePrivateKey([]byte(strings.ReplaceAll(keyPemFixture, "TEST ", "")))
	if err != nil {
		panic(err)
	}
	return key.Signer
}
