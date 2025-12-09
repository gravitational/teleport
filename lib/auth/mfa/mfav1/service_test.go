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

package mfav1_test

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	mfav1impl "github.com/gravitational/teleport/lib/auth/mfa/mfav1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCreateSessionChallenge_WebAuthn(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			AuditLog:    &eventstest.MockAuditLog{Emitter: emitter},
			ClusterName: "test-cluster",
			Dir:         t.TempDir(),
			AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
				SecondFactors: []types.SecondFactorType{types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN},
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
		},
	})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user")
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authz.LocalUser{
		Identity: tlsca.Identity{
			Username: user.GetName(),
		}},
	)

	_, err = authtest.RegisterTestDevice(ctx, authServer.Auth(), "foo", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil)
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		AuthServer: authServer.Auth(),
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
	})
	require.NoError(t, err)

	resp, err := service.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: []byte("test-session-id"),
				},
			},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.MfaChallenge)
	require.NotNil(t, resp.MfaChallenge.WebauthnChallenge)
	require.NotEmpty(t, resp.MfaChallenge.WebauthnChallenge.PublicKey.Challenge)

	event := emitter.LastEvent()
	require.Equal(t, events.CreateMFAAuthChallengeCode, event.GetCode())
}

func TestCreateSessionChallenge_SSO(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			AuditLog:    &eventstest.MockAuditLog{Emitter: emitter},
			ClusterName: "test-cluster",
			Dir:         t.TempDir(),
			AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
				SecondFactors: []types.SecondFactorType{
					types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
					types.SecondFactorType_SECOND_FACTOR_TYPE_SSO,
				},
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
		},
	})
	require.NoError(t, err)

	// FIXME: Configure SSO MFA device! This test doesn't pass.
	// _, err = authServer.Auth().CreateSAMLConnector(t.Context(), &types.SAMLConnectorV2{
	// 	Kind:    "saml",
	// 	Version: "v2",
	// 	Metadata: types.Metadata{
	// 		Name: "test-sso",
	// 	},
	// 	Spec: types.SAMLConnectorSpecV2{
	// 		AssertionConsumerService: "https://example.com",
	// 		EntityDescriptorURL:      "https://example.com",
	// 		ServiceProviderIssuer:    "https://example.com",
	// 		MFASettings: &types.SAMLConnectorMFASettings{
	// 			Enabled:             true,
	// 			EntityDescriptorUrl: "https://example.com",
	// 		},
	// 	},
	// })
	// require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user")
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authz.LocalUser{
		Identity: tlsca.Identity{
			Username: user.GetName(),
		}},
	)

	err = authServer.Auth().UpsertMFADevice(ctx, user.GetName(), &types.MFADevice{
		Id:       "1234567",
		AddedAt:  time.Now(),
		LastUsed: time.Now(),
		Metadata: types.Metadata{
			Name: "Foo",
		},
		Device: &types.MFADevice_Sso{
			Sso: &types.SSOMFADevice{
				ConnectorId:   "sso-device-id",
				ConnectorType: "sso-device",
				DisplayName:   "sso-device-name",
			},
		},
	})
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		AuthServer: authServer.Auth(),
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
	})
	require.NoError(t, err)

	resp, err := service.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: []byte("test-session-id"),
				},
			},
			SsoClientRedirectUrl: "https://client/redirect",
			ProxyAddressForSso:   "proxy.example.com",
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.MfaChallenge)
	require.NotNil(t, resp.MfaChallenge.SsoChallenge)
	require.Equal(t, "sso-device-id", resp.MfaChallenge.SsoChallenge.Device.ConnectorId)
	require.Equal(t, "sso-device", resp.MfaChallenge.SsoChallenge.Device.ConnectorType)
	// require.Equal(t, "req-123", resp.MfaChallenge.SsoChallenge.RequestId)
	// require.Equal(t, "https://sso/redirect", resp.MfaChallenge.SsoChallenge.RedirectUrl)
}

func TestCreateSessionChallenge_BadRequest(t *testing.T) {
	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		AuthServer: authServer.Auth(),
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
	})
	require.NoError(t, err)

	tests := []struct {
		name string
		req  *mfav1.CreateSessionChallengeRequest
	}{
		{
			name: "nil request",
			req:  nil,
		},
		{
			name: "missing payload",
			req:  &mfav1.CreateSessionChallengeRequest{},
		},
		{
			name: "missing SshSessionId in payload",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload: &mfav1.SessionIdentifyingPayload{},
			},
		},
		{
			name: "empty SshSessionId in payload",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload: &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
						SshSessionId: []byte{},
					},
				},
			},
		},
		{
			name: "SSO challenge missing SsoClientRedirectUrl",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload: &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
						SshSessionId: []byte("test-session-id"),
					},
				},
				SsoClientRedirectUrl: "",
				ProxyAddressForSso:   "proxy.example.com",
			},
		},
		{
			name: "SSO challenge missing ProxyAddressForSso",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload: &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
						SshSessionId: []byte("test-session-id"),
					},
				},
				SsoClientRedirectUrl: "https://client/redirect",
				ProxyAddressForSso:   "",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := service.CreateSessionChallenge(t.Context(), tc.req)
			require.Nil(t, resp)
			require.True(t, trace.IsBadParameter(err))
		})
	}
}
