// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"errors"
	"io"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
)

type fakeAuthClient struct {
	authclient.ClientI

	isMFARequired     func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error)
	generateUserCerts func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error)
}

func (f fakeAuthClient) Close() error {
	return nil
}

func (f fakeAuthClient) IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
	if f.isMFARequired == nil {
		return nil, trace.NotImplemented("isMFARequired was not set")
	}

	return f.isMFARequired(ctx, req)
}

func (f fakeAuthClient) GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
	if f.generateUserCerts == nil {
		return nil, trace.NotImplemented("generateUserCerts was not set")
	}

	return f.generateUserCerts(ctx, req)
}

func (f fakeAuthClient) CreateAuthenticateChallenge(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	return &proto.MFAAuthenticateChallenge{WebauthnChallenge: &webauthnpb.CredentialAssertion{}}, nil
}

type fakePrompt struct {
	mfa.Prompt

	err error
}

func (f fakePrompt) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if f.err != nil {
		return nil, f.err
	}

	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{Webauthn: &webauthnpb.CredentialAssertionResponse{}},
	}, nil
}

func TestIssueUserCertsWithMFA(t *testing.T) {
	ca := newTestAuthority(t)
	clock := clockwork.NewFakeClock()

	agent, err := NewLocalAgent(LocalAgentConfig{
		ClientStore: NewMemClientStore(),
		ProxyHost:   "test",
		Username:    "alice",
		Insecure:    true,
		Site:        "test",
		LoadAllCAs:  false,
	})
	require.NoError(t, err)

	keyRing := ca.makeSignedKeyRing(t, KeyRingIndex{
		ProxyHost:   "test",
		ClusterName: "test",
		Username:    "alice",
	}, false)

	require.NoError(t, agent.clientStore.AddKeyRing(keyRing))

	leafKeyRing := ca.makeSignedKeyRing(t, KeyRingIndex{
		ProxyHost:   "test",
		ClusterName: "leaf",
		Username:    "alice",
	}, false)

	require.NoError(t, agent.clientStore.AddKeyRing(leafKeyRing))

	pemBytes, ok := fixtures.PEMBytes["rsa"]
	require.True(t, ok, "RSA key not found in fixtures")

	caSigner, err := ssh.ParsePrivateKey(pemBytes)
	require.NoError(t, err)

	failedPrompt := fakePrompt{err: errors.New("prompt failed intentionally")}

	defaultGenerateUserCerts := func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
		var sshCert, tlsCert []byte
		var err error
		if req.SSHPublicKey != nil {
			sshCert, err = ca.keygen.GenerateUserCert(sshca.UserCertificateRequest{
				CASigner:          caSigner,
				PublicUserKey:     req.SSHPublicKey,
				TTL:               req.Expires.Sub(clock.Now()),
				CertificateFormat: req.Format,
				Identity: sshca.Identity{
					Username:       req.Username,
					RouteToCluster: req.RouteToCluster,
				},
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		if req.TLSPublicKey != nil {
			pub, err := keys.ParsePublicKey(req.TLSPublicKey)
			require.NoError(t, err)
			identity := tlsca.Identity{
				Username: req.Username,
				Groups:   []string{"groups"},
			}
			subject, err := identity.Subject()
			require.NoError(t, err)
			tlsCert, err = ca.tlsCA.GenerateCertificate(tlsca.CertificateRequest{
				Clock:     clock,
				PublicKey: pub,
				Subject:   subject,
				NotAfter:  req.Expires,
			})
			require.NoError(t, err)
		}
		return &proto.Certs{SSH: sshCert, TLS: tlsCert}, nil
	}

	tests := []struct {
		name                    string
		mfaRequired             proto.MFARequired
		agent                   *LocalKeyAgent
		params                  ReissueParams
		prompt                  fakePrompt
		signatureAlgorithmSuite types.SignatureAlgorithmSuite
		assertion               func(t *testing.T, result *IssueUserCertsWithMFAResult, err error)
	}{
		{
			name:        "ssh no mfa",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_NO,
			params:      ReissueParams{NodeName: "test"},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_NO, result.MFARequired)
				require.Nil(t, result.ReusableMFAResponse)
				require.NotNil(t, result.KeyRing)
				require.NotEmpty(t, result.KeyRing.Cert)
			},
		},
		{
			name:        "ssh mfa success",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_YES,
			params:      ReissueParams{NodeName: "test"},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, result.MFARequired)
				require.Nil(t, result.ReusableMFAResponse)
				require.NotNil(t, result.KeyRing)
				require.NotEmpty(t, result.KeyRing.Cert)
			},
		},
		{
			name:        "ssh mfa fail",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_YES,
			params:      ReissueParams{NodeName: "test"},
			prompt:      failedPrompt,
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.Error(t, err)
				require.NotNil(t, result)
				require.Nil(t, result.KeyRing)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, result.MFARequired)
			},
		},
		{
			name:        "kube no mfa",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_NO,
			params:      ReissueParams{KubernetesCluster: "test"},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_NO, result.MFARequired)
				require.NotNil(t, result.KeyRing)
				require.NotEmpty(t, result.KeyRing.KubeTLSCredentials["test"])
			},
		},
		{
			name:        "kube mfa success",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_YES,
			params:      ReissueParams{KubernetesCluster: "test"},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, result.MFARequired)
				require.NotNil(t, result.KeyRing)
				cred := result.KeyRing.KubeTLSCredentials["test"]
				require.NotEmpty(t, cred)
				_, err = cred.TLSCertificate()
				require.NoError(t, err)
				require.IsType(t, (*ecdsa.PrivateKey)(nil), cred.PrivateKey.Signer)
			},
		},
		{
			name:                    "kube legacy",
			mfaRequired:             proto.MFARequired_MFA_REQUIRED_YES,
			params:                  ReissueParams{KubernetesCluster: "test"},
			signatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, result.MFARequired)
				require.NotNil(t, result.KeyRing)
				cred := keyRing.KubeTLSCredentials["test"]
				require.NotEmpty(t, cred)
				_, err = cred.TLSCertificate()
				require.NoError(t, err)
				require.IsType(t, (*rsa.PrivateKey)(nil), cred.PrivateKey.Signer)
			},
		},
		{
			name:        "kube mfa fail",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_YES,
			params:      ReissueParams{KubernetesCluster: "test"},
			prompt:      failedPrompt,
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.Error(t, err)
				require.NotNil(t, result)
				require.Nil(t, result.KeyRing)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, result.MFARequired)
			},
		}, {
			name:        "db no mfa",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_NO,
			params: ReissueParams{
				RouteToDatabase: proto.RouteToDatabase{
					ServiceName: "test",
					Username:    "test",
					Database:    "test",
				},
			},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_NO, result.MFARequired)
				require.NotNil(t, result.KeyRing)
				require.NotEmpty(t, result.KeyRing.DBTLSCredentials["test"])
			},
		},
		{
			name:        "db mfa success",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_YES,
			params: ReissueParams{
				RouteToDatabase: proto.RouteToDatabase{
					ServiceName: "test",
					Username:    "test",
					Database:    "test",
				},
			},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, result.MFARequired)
				require.Nil(t, result.ReusableMFAResponse)
				require.NotNil(t, result.KeyRing)
				cred := keyRing.DBTLSCredentials["test"]
				require.NotEmpty(t, cred)
				_, err = cred.TLSCertificate()
				require.NoError(t, err)
				require.IsType(t, (*rsa.PrivateKey)(nil), cred.PrivateKey.Signer)
			},
		},
		{
			name:        "db mfa fail",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_YES,
			params: ReissueParams{
				RouteToDatabase: proto.RouteToDatabase{
					Username: "test",
					Database: "test",
				},
			},
			prompt: failedPrompt,
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.Error(t, err)
				require.Nil(t, result)
			},
		},
		{
			name: "no keys loaded",
			agent: &LocalKeyAgent{
				clientStore: NewMemClientStore(),
			},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.Error(t, err)
				require.Nil(t, result)
			},
		},
		{
			name:   "existing credentials used",
			params: ReissueParams{NodeName: "test", ExistingCreds: keyRing},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.Error(t, err)
				require.Nil(t, result)
			},
		},
		{
			name:        "mfa unknown",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_UNSPECIFIED,
			params:      ReissueParams{NodeName: "test"},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.Error(t, err)
				require.Nil(t, result)
			},
		},
		{
			name:        "ssh leaf cluster no mfa",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_NO,
			params: ReissueParams{
				NodeName:       "test",
				RouteToCluster: "leaf",
				AuthClient: fakeAuthClient{
					isMFARequired: func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
						return &proto.IsMFARequiredResponse{MFARequired: proto.MFARequired_MFA_REQUIRED_NO, Required: false}, nil
					},
				},
			},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_NO, result.MFARequired)
				require.NotNil(t, result.KeyRing)
				require.NotEmpty(t, result.KeyRing.Cert)
			},
		},
		{
			name:        "ssh leaf cluster mfa",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_YES,
			params: ReissueParams{
				NodeName:       "test",
				RouteToCluster: "leaf",
				AuthClient: fakeAuthClient{
					isMFARequired: func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
						return &proto.IsMFARequiredResponse{MFARequired: proto.MFARequired_MFA_REQUIRED_YES, Required: true}, nil
					},
				},
			},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, result.MFARequired)
				require.NotNil(t, result.KeyRing)
				require.NotEmpty(t, result.KeyRing.Cert)
			},
		},
		{
			name:        "tsh db exec no mfa",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_NO,
			params: ReissueParams{
				RouteToDatabase: proto.RouteToDatabase{
					ServiceName: "test",
					Username:    "test",
				},
				RequesterName:       proto.UserCertsRequest_TSH_DB_EXEC,
				ReusableMFAResponse: &proto.MFAAuthenticateResponse{},
				AuthClient: fakeAuthClient{
					isMFARequired: func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
						return &proto.IsMFARequiredResponse{MFARequired: proto.MFARequired_MFA_REQUIRED_NO, Required: false}, nil
					},
					generateUserCerts: func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
						// Ensure no MFA response is passed.
						if req.MFAResponse != nil {
							return nil, trace.BadParameter("mfa response is not nil")
						}
						return defaultGenerateUserCerts(ctx, req)
					},
				},
			},
			prompt: failedPrompt, // should not be called
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_NO, result.MFARequired)
				require.Nil(t, result.ReusableMFAResponse)
				require.NotNil(t, result.KeyRing)
			},
		},
		{
			name:        "tsh db exec mfa required",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_YES,
			params: ReissueParams{
				RouteToDatabase: proto.RouteToDatabase{
					ServiceName: "test",
					Username:    "test",
				},
				RequesterName: proto.UserCertsRequest_TSH_DB_EXEC,
			},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, result.MFARequired)
				require.NotNil(t, result.ReusableMFAResponse) // new MFA response
				require.NotNil(t, result.KeyRing)
			},
		},
		{
			name:        "tsh db exec mfa required with reusable MFA",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_YES,
			params: ReissueParams{
				RouteToDatabase: proto.RouteToDatabase{
					ServiceName: "test",
					Username:    "test",
				},
				RequesterName:       proto.UserCertsRequest_TSH_DB_EXEC,
				ReusableMFAResponse: &proto.MFAAuthenticateResponse{},
			},
			prompt: failedPrompt, // should not be called
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, result.MFARequired)
				require.Nil(t, result.ReusableMFAResponse) // no new MFA response
				require.NotNil(t, result.KeyRing)
			},
		},
		{
			name:        "tsh db exec mfa required with reusable MFA expired",
			mfaRequired: proto.MFARequired_MFA_REQUIRED_NO,
			params: ReissueParams{
				RouteToDatabase: proto.RouteToDatabase{
					ServiceName: "test",
					Username:    "test",
				},
				RequesterName:       proto.UserCertsRequest_TSH_DB_EXEC,
				ReusableMFAResponse: &proto.MFAAuthenticateResponse{},
				AuthClient: fakeAuthClient{
					isMFARequired: func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
						return &proto.IsMFARequiredResponse{MFARequired: proto.MFARequired_MFA_REQUIRED_YES, Required: true}, nil
					},
					generateUserCerts: func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
						// This is the fake reusable MFA response passed in the first call.
						if req.MFAResponse != nil && req.MFAResponse.Response == nil {
							return nil, trace.Wrap(&mfa.ErrExpiredReusableMFAResponse)
						}
						// The second call should continue here.
						return defaultGenerateUserCerts(ctx, req)
					},
				},
			},
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, result.MFARequired)
				require.NotNil(t, result.ReusableMFAResponse) // new MFA response
				require.NotNil(t, result.KeyRing)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			agent := agent
			if test.agent != nil {
				agent = test.agent
			}
			if test.params.AuthClient != nil {
				defer test.params.AuthClient.Close()
			}

			suite := test.signatureAlgorithmSuite
			if suite == types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED {
				suite = types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1
			}

			clt := &ClusterClient{
				tc: &TeleportClient{
					localAgent: agent,
					Config: Config{
						WebProxyAddr: "proxy.example.com",
						SiteName:     "test",
						Tracer:       tracing.NoopTracer("test"),
						MFAPromptConstructor: func(cfg *libmfa.PromptConfig) mfa.Prompt {
							return test.prompt
						},
						Stderr: io.Discard,
					},
					lastPing: &webclient.PingResponse{
						Auth: webclient.AuthenticationSettings{
							SignatureAlgorithmSuite: suite,
						},
					},
				},
				ProxyClient: &proxy.Client{},
				AuthClient: fakeAuthClient{
					isMFARequired: func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
						switch test.mfaRequired {
						case proto.MFARequired_MFA_REQUIRED_YES:
							return &proto.IsMFARequiredResponse{MFARequired: proto.MFARequired_MFA_REQUIRED_YES, Required: true}, nil
						case proto.MFARequired_MFA_REQUIRED_NO:
							return &proto.IsMFARequiredResponse{MFARequired: proto.MFARequired_MFA_REQUIRED_NO, Required: false}, nil
						default:
							return nil, trace.NotImplemented("mfa unknown")
						}
					},
					generateUserCerts: defaultGenerateUserCerts,
				},
				Tracer:  tracing.NoopTracer("test"),
				cluster: "test",
				root:    "test",
			}

			ctx := context.Background()

			result, err := clt.IssueUserCertsWithMFA(ctx, test.params)
			test.assertion(t, result, err)
		})
	}
}
