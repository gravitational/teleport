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
	"cmp"
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

	isMFARequired               func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error)
	generateUserCerts           func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error)
	createAuthenticateChallenge func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error)
	close                       func() error
}

func (f fakeAuthClient) Close() error {
	if f.close == nil {
		return nil
	}

	return f.close()
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
	if f.createAuthenticateChallenge == nil {
		return &proto.MFAAuthenticateChallenge{WebauthnChallenge: &webauthnpb.CredentialAssertion{}}, nil
	}

	return f.createAuthenticateChallenge(ctx, req)
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

// newTestGenerateUserCerts returns a [fakeAuthClient] GenerateUserCerts
// implementation that issues certificates signed by the given test authority.
func newTestGenerateUserCerts(t *testing.T, ca testAuthority, clock clockwork.Clock, caSigner ssh.Signer) func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
	return func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
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

	defaultGenerateUserCerts := newTestGenerateUserCerts(t, ca, clock, caSigner)

	tests := []struct {
		name                    string
		mfaRequired             proto.MFARequired
		agent                   *LocalKeyAgent
		params                  ReissueParams
		prompt                  fakePrompt
		signatureAlgorithmSuite types.SignatureAlgorithmSuite
		// clientCluster overrides the ClusterClient's cluster field. Defaults to "test".
		clientCluster string
		// generateUserCerts overrides GenerateUserCerts on the connected cluster's auth client.
		// Defaults to issuing them from the test authority.
		generateUserCerts func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error)
		// wantPromptReason, when non-empty, asserts the PromptReason passed to the
		// MFA prompt constructor.
		wantPromptReason string
		assertion        func(t *testing.T, result *IssueUserCertsWithMFAResult, err error)
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
			name:             "ssh mfa success",
			mfaRequired:      proto.MFARequired_MFA_REQUIRED_YES,
			params:           ReissueParams{NodeName: "test"},
			wantPromptReason: `MFA is required to access node "test"`,
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
			name: "ssh login falls back to host login",
			params: ReissueParams{
				NodeName: "test",
				AuthClient: fakeAuthClient{
					isMFARequired: func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
						nodeReq, ok := req.Target.(*proto.IsMFARequiredRequest_Node)
						require.True(t, ok)
						require.Equal(t, "default-login", nodeReq.Node.Login)
						return &proto.IsMFARequiredResponse{MFARequired: proto.MFARequired_MFA_REQUIRED_YES, Required: true}, nil
					},
				},
			},
			generateUserCerts: func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
				require.Equal(t, "default-login", req.SSHLogin)
				return defaultGenerateUserCerts(ctx, req)
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
			name: "ssh login override used",
			params: ReissueParams{
				NodeName: "test",
				SSHLogin: "override-login",
				AuthClient: fakeAuthClient{
					isMFARequired: func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
						nodeReq, ok := req.Target.(*proto.IsMFARequiredRequest_Node)
						require.True(t, ok)
						require.Equal(t, "override-login", nodeReq.Node.Login)
						return &proto.IsMFARequiredResponse{MFARequired: proto.MFARequired_MFA_REQUIRED_YES, Required: true}, nil
					},
				},
			},
			generateUserCerts: func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
				require.Equal(t, "override-login", req.SSHLogin)
				return defaultGenerateUserCerts(ctx, req)
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
				},
			},
			generateUserCerts: func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
				// Ensure no MFA response is passed.
				if req.MFAResponse != nil {
					return nil, trace.BadParameter("mfa response is not nil")
				}
				return defaultGenerateUserCerts(ctx, req)
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
				},
			},
			generateUserCerts: func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
				// This is the fake reusable MFA response passed in the first call.
				if req.MFAResponse != nil && req.MFAResponse.Response == nil {
					return nil, trace.Wrap(&mfa.ErrExpiredReusableMFAResponse)
				}
				// The second call should continue here.
				return defaultGenerateUserCerts(ctx, req)
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
			name:          "session MFA from a leaf cluster mentions the leaf in the prompt reason",
			mfaRequired:   proto.MFARequired_MFA_REQUIRED_YES,
			clientCluster: "leaf",
			params: ReissueParams{
				NodeName: "test",
				// In real world RouteToCluster would be "leaf", but this doesn't work
				// with how the test is currently set up.
				RouteToCluster: "test",
				AuthClient: fakeAuthClient{
					isMFARequired: func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
						return &proto.IsMFARequiredResponse{MFARequired: proto.MFARequired_MFA_REQUIRED_YES, Required: true}, nil
					},
					generateUserCerts: defaultGenerateUserCerts,
				},
			},
			wantPromptReason: `MFA is required to access node "test" from leaf cluster "leaf"`,
			assertion: func(t *testing.T, result *IssueUserCertsWithMFAResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
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

			var capturedPromptCfg *libmfa.PromptConfig
			clientCluster := cmp.Or(test.clientCluster, "test")
			generateUserCerts := defaultGenerateUserCerts
			if test.generateUserCerts != nil {
				generateUserCerts = test.generateUserCerts
			}
			clt := &ClusterClient{
				tc: &TeleportClient{
					localAgent: agent,
					Config: Config{
						WebProxyAddr: "proxy.example.com",
						SiteName:     "test",
						HostLogin:    "default-login",
						Tracer:       tracing.NoopTracer("test"),
						MFAPromptConstructor: func(cfg *libmfa.PromptConfig) mfa.Prompt {
							capturedPromptCfg = cfg
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
					generateUserCerts: generateUserCerts,
				},
				Tracer:  tracing.NoopTracer("test"),
				cluster: clientCluster,
				root:    "test",
			}

			// Auth clients for other clusters, so cases whose client is connected to a leaf can reach the root without a proxy.
			dial := func(_ context.Context, clusterName string) (authclient.ClientI, error) {
				// Mirror ConnectToCluster: the connected cluster resolves without opening a connection.
				if clusterName == clientCluster {
					return clt.CurrentCluster(), nil
				}
				return fakeAuthClient{
					isMFARequired: func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
						return &proto.IsMFARequiredResponse{MFARequired: test.mfaRequired, Required: test.mfaRequired == proto.MFARequired_MFA_REQUIRED_YES}, nil
					},
					generateUserCerts: defaultGenerateUserCerts,
				}, nil
			}

			ctx := context.Background()

			result, err := clt.issueUserCertsWithMFA(ctx, dial, test.params)
			test.assertion(t, result, err)

			if test.wantPromptReason != "" {
				require.NotNil(t, capturedPromptCfg, "MFA prompt constructor was not invoked")
				require.Equal(t, test.wantPromptReason, capturedPromptCfg.PromptReason)
			}
		})
	}
}

// TestIssueUserCertsWithMFAAuthClientMatrix runs IssueUserCertsWithMFA over every permutation of the inputs
// that decide which auth server serves the request, and asserts:
// - which one answered the MFA requirement check,
// - which one issued the certs,
// - which clusters were dialed.
//
// Whatever the caller passes, the certs must come from the root cluster's auth server.
// A client connected to the root issues them with its own auth client; one connected to a leaf dials the root first.
// A caller-supplied client only answers the requirement check, so handing over a leaf's client cannot move the issuance.
func TestIssueUserCertsWithMFAAuthClientMatrix(t *testing.T) {
	t.Parallel()

	// The clusters a cell can involve, named so the assertions can say which one served each step.
	const (
		root  = "root"
		leaf1 = "leaf1"
		leaf2 = "leaf2"
	)

	// The auth clients a cell can involve, named so the assertions can say which one served each step.
	const (
		connected   = "connected"
		callerRoot  = "caller root"
		callerLeaf  = "caller leaf"
		dialedRoot  = "dialed root"
		dialedLeaf1 = "dialed leaf1"
		dialedLeaf2 = "dialed leaf2"
	)

	ca := newTestAuthority(t)
	clock := clockwork.NewFakeClock()

	localAgent, err := NewLocalAgent(LocalAgentConfig{
		ClientStore: NewMemClientStore(),
		ProxyHost:   root,
		Username:    "alice",
		Insecure:    true,
		Site:        root,
		LoadAllCAs:  false,
	})
	require.NoError(t, err)

	for _, cluster := range []string{root, leaf1, leaf2} {
		require.NoError(t, localAgent.clientStore.AddKeyRing(ca.makeSignedKeyRing(t, KeyRingIndex{
			ProxyHost:   root,
			ClusterName: cluster,
			Username:    "alice",
		}, false)))
	}

	pemBytes, ok := fixtures.PEMBytes["rsa"]
	require.True(t, ok, "RSA key not found in fixtures")
	caSigner, err := ssh.ParsePrivateKey(pemBytes)
	require.NoError(t, err)

	generateUserCerts := newTestGenerateUserCerts(t, ca, clock, caSigner)

	type inputs struct {
		callerClient       string // which auth client the caller supplies, if any
		prefetchedMFACheck bool   // whether ReissueParams.MFACheck is set
		routeToCluster     string // the cluster the caller wants to reach
		clientCluster      string // the cluster the connected auth client belongs to
	}
	type counts = map[string]int // counts how often each auth client, or each cluster, was used.
	type outcome struct {
		issuer string // the auth client expected to issue the certs
		checks counts // every auth client asked whether MFA is required
		dials  counts // every cluster a connection is opened to
	}

	connectedIssues := outcome{issuer: connected}
	leafDialsRoot := outcome{issuer: dialedRoot, dials: counts{root: 1}}

	// Every permutation the loops below generate must appear here, so a missing row fails the test.
	want := map[inputs]outcome{
		{callerClient: "", prefetchedMFACheck: true, routeToCluster: root, clientCluster: root}:   connectedIssues,
		{callerClient: "", prefetchedMFACheck: true, routeToCluster: root, clientCluster: leaf1}:  leafDialsRoot,
		{callerClient: "", prefetchedMFACheck: true, routeToCluster: leaf1, clientCluster: root}:  connectedIssues,
		{callerClient: "", prefetchedMFACheck: true, routeToCluster: leaf1, clientCluster: leaf1}: leafDialsRoot,
		{callerClient: "", prefetchedMFACheck: true, routeToCluster: leaf2, clientCluster: leaf1}: leafDialsRoot,

		{callerClient: callerRoot, prefetchedMFACheck: true, routeToCluster: root, clientCluster: root}:   connectedIssues,
		{callerClient: callerRoot, prefetchedMFACheck: true, routeToCluster: root, clientCluster: leaf1}:  leafDialsRoot,
		{callerClient: callerRoot, prefetchedMFACheck: true, routeToCluster: leaf1, clientCluster: root}:  connectedIssues,
		{callerClient: callerRoot, prefetchedMFACheck: true, routeToCluster: leaf1, clientCluster: leaf1}: leafDialsRoot,
		{callerClient: callerRoot, prefetchedMFACheck: true, routeToCluster: leaf2, clientCluster: leaf1}: leafDialsRoot,

		{callerClient: callerLeaf, prefetchedMFACheck: true, routeToCluster: root, clientCluster: root}:   connectedIssues,
		{callerClient: callerLeaf, prefetchedMFACheck: true, routeToCluster: root, clientCluster: leaf1}:  leafDialsRoot,
		{callerClient: callerLeaf, prefetchedMFACheck: true, routeToCluster: leaf1, clientCluster: root}:  connectedIssues,
		{callerClient: callerLeaf, prefetchedMFACheck: true, routeToCluster: leaf1, clientCluster: leaf1}: leafDialsRoot,
		{callerClient: callerLeaf, prefetchedMFACheck: true, routeToCluster: leaf2, clientCluster: leaf1}: leafDialsRoot,

		{callerClient: "", prefetchedMFACheck: false, routeToCluster: root, clientCluster: root}:   {issuer: connected, checks: counts{connected: 1}},
		{callerClient: "", prefetchedMFACheck: false, routeToCluster: root, clientCluster: leaf1}:  {issuer: dialedRoot, checks: counts{dialedRoot: 1}, dials: counts{root: 1}},
		{callerClient: "", prefetchedMFACheck: false, routeToCluster: leaf1, clientCluster: root}:  {issuer: connected, checks: counts{dialedLeaf1: 1}, dials: counts{leaf1: 1}},
		{callerClient: "", prefetchedMFACheck: false, routeToCluster: leaf1, clientCluster: leaf1}: {issuer: dialedRoot, checks: counts{connected: 1}, dials: counts{root: 1}},
		{callerClient: "", prefetchedMFACheck: false, routeToCluster: leaf2, clientCluster: leaf1}: {issuer: dialedRoot, checks: counts{dialedLeaf2: 1}, dials: counts{leaf2: 1, root: 1}},

		{callerClient: callerRoot, prefetchedMFACheck: false, routeToCluster: root, clientCluster: root}:   {issuer: connected, checks: counts{callerRoot: 1}},
		{callerClient: callerRoot, prefetchedMFACheck: false, routeToCluster: root, clientCluster: leaf1}:  {issuer: dialedRoot, checks: counts{callerRoot: 1}, dials: counts{root: 1}},
		{callerClient: callerRoot, prefetchedMFACheck: false, routeToCluster: leaf1, clientCluster: root}:  {issuer: connected, checks: counts{callerRoot: 1}},
		{callerClient: callerRoot, prefetchedMFACheck: false, routeToCluster: leaf1, clientCluster: leaf1}: {issuer: dialedRoot, checks: counts{callerRoot: 1}, dials: counts{root: 1}},
		{callerClient: callerRoot, prefetchedMFACheck: false, routeToCluster: leaf2, clientCluster: leaf1}: {issuer: dialedRoot, checks: counts{callerRoot: 1}, dials: counts{root: 1}},

		{callerClient: callerLeaf, prefetchedMFACheck: false, routeToCluster: root, clientCluster: root}:   {issuer: connected, checks: counts{callerLeaf: 1}},
		{callerClient: callerLeaf, prefetchedMFACheck: false, routeToCluster: root, clientCluster: leaf1}:  {issuer: dialedRoot, checks: counts{callerLeaf: 1}, dials: counts{root: 1}},
		{callerClient: callerLeaf, prefetchedMFACheck: false, routeToCluster: leaf1, clientCluster: root}:  {issuer: connected, checks: counts{callerLeaf: 1}},
		{callerClient: callerLeaf, prefetchedMFACheck: false, routeToCluster: leaf1, clientCluster: leaf1}: {issuer: dialedRoot, checks: counts{callerLeaf: 1}, dials: counts{root: 1}},
		{callerClient: callerLeaf, prefetchedMFACheck: false, routeToCluster: leaf2, clientCluster: leaf1}: {issuer: dialedRoot, checks: counts{callerLeaf: 1}, dials: counts{root: 1}},
	}

	// newClusterClient returns a client connected to the named cluster, backed by authClient.
	newClusterClient := func(cluster string, authClient authclient.ClientI) *ClusterClient {
		return &ClusterClient{
			tc: &TeleportClient{
				localAgent: localAgent,
				Config: Config{
					WebProxyAddr: "proxy.example.com",
					SiteName:     root,
					HostLogin:    "default-login",
					Tracer:       tracing.NoopTracer("test"),
					MFAPromptConstructor: func(cfg *libmfa.PromptConfig) mfa.Prompt {
						return fakePrompt{}
					},
					Stderr: io.Discard,
				},
				lastPing: &webclient.PingResponse{
					Auth: webclient.AuthenticationSettings{
						SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
					},
				},
			},
			ProxyClient: &proxy.Client{},
			AuthClient:  authClient,
			Tracer:      tracing.NoopTracer("test"),
			cluster:     cluster,
			root:        root,
		}
	}

	requireCounts := func(t *testing.T, want, got counts, what string) {
		if want == nil { // A nil expectation means the step must not have happened at all.
			require.Empty(t, got, what)
			return
		}
		require.Equal(t, want, got, what)
	}

	topologies := []struct{ routeToCluster, clientCluster string }{
		{root, root},
		{root, leaf1},
		{leaf1, root},
		{leaf1, leaf1},
		{leaf2, leaf1},
	}

	for _, callerClient := range []string{"", callerRoot, callerLeaf} {
		for _, prefetchedMFACheck := range []bool{true, false} {
			for _, topology := range topologies {
				in := inputs{
					callerClient:       callerClient,
					prefetchedMFACheck: prefetchedMFACheck,
					routeToCluster:     topology.routeToCluster,
					clientCluster:      topology.clientCluster,
				}

				name := "no caller client"
				if in.callerClient != "" {
					name = in.callerClient
				}
				if in.prefetchedMFACheck {
					name += ", prefetched check"
				} else {
					name += ", checked requirement"
				}
				name += ", routed to " + in.routeToCluster + ", " + in.clientCluster + " client"

				t.Run(name, func(t *testing.T) {
					t.Parallel()

					expected, ok := want[in]
					require.True(t, ok, "no expectation declared for %+v", in)

					checks, certRequests, challenges, closes := counts{}, counts{}, counts{}, counts{}
					var certRoutes []string
					authClients := map[string]fakeAuthClient{}
					for _, name := range []string{connected, callerRoot, callerLeaf, dialedRoot, dialedLeaf1, dialedLeaf2} {
						authClients[name] = fakeAuthClient{
							isMFARequired: func(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
								checks[name]++
								return &proto.IsMFARequiredResponse{MFARequired: proto.MFARequired_MFA_REQUIRED_YES, Required: true}, nil
							},
							generateUserCerts: func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
								certRequests[name]++
								certRoutes = append(certRoutes, req.RouteToCluster)
								return generateUserCerts(ctx, req)
							},
							createAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
								challenges[name]++
								return &proto.MFAAuthenticateChallenge{WebauthnChallenge: &webauthnpb.CredentialAssertion{}}, nil
							},
							close: func() error {
								closes[name]++
								return nil
							},
						}
					}

					clt := newClusterClient(in.clientCluster, authClients[connected])

					params := ReissueParams{NodeName: "test", RouteToCluster: in.routeToCluster}
					if in.prefetchedMFACheck {
						params.MFACheck = &proto.IsMFARequiredResponse{
							MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
							Required:    true,
						}
					}
					if in.callerClient != "" {
						params.AuthClient = authClients[in.callerClient]
					}

					dials := counts{}
					dial := func(_ context.Context, clusterName string) (authclient.ClientI, error) {
						if clusterName == in.clientCluster {
							return clt.CurrentCluster(), nil
						}
						dials[clusterName]++
						return authClients["dialed "+clusterName], nil
					}

					result, err := clt.issueUserCertsWithMFA(context.Background(), dial, params)
					require.NoError(t, err)
					require.NotNil(t, result)
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, result.MFARequired)
					require.NotEmpty(t, result.KeyRing.Cert)

					requireCounts(t, counts{expected.issuer: 1}, certRequests, "the root cluster's auth server must issue the certs, and nothing else")
					requireCounts(t, counts{expected.issuer: 1}, challenges, "the MFA challenge must come from the auth server that issues the certs")
					requireCounts(t, expected.checks, checks, "auth servers asked whether MFA is required")
					requireCounts(t, expected.dials, dials, "clusters dialed")

					require.Equal(t, []string{in.routeToCluster}, certRoutes, "cert requests, by routed cluster")

					wantCloses := counts{}
					for cluster := range expected.dials {
						wantCloses["dialed "+cluster] = 1
					}
					requireCounts(t, wantCloses, closes, "auth clients closed")
				})
			}
		}
	}
}
