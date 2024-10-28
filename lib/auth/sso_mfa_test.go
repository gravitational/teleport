/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"context"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/mfatypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

func TestSSOMFAChallenge_Creation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fakeClock := clockwork.NewFakeClock()
	testAuthServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: fakeClock,
	})
	require.NoError(t, err)
	testServer, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)

	a := testServer.Auth()
	a.SetSAMLService(&fakeSSOService{a})
	a.SetOIDCService(&fakeSSOService{a})

	// Enable SSO MFA support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type: constants.Local,
		SecondFactors: []types.SecondFactorType{
			types.SecondFactorType_SECOND_FACTOR_TYPE_SSO,
		},
		AllowLocalAuth: types.NewBoolOption(false),
	})
	require.NoError(t, err)
	_, err = a.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Create a standard user.
	standardUser, _, err := CreateUserAndRole(a, "standard", []string{"role"}, nil)
	require.NoError(t, err)

	// Create a fake saml user with MFA disabled.
	noMFASAMLUser, noMFASAMLRole, err := CreateUserAndRole(a, "saml-user-no-mfa", []string{"role"}, nil)
	require.NoError(t, err)

	noMFASAMLConnector, err := types.NewSAMLConnector("saml-no-mfa", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "http://localhost:65535/acs", // not called
		Issuer:                   "test",
		SSO:                      "https://localhost:65535/sso", // not called
		AttributesToRoles: []types.AttributeMapping{
			// not used. can be any name, value but role must exist
			{Name: "groups", Value: "admin", Roles: []string{noMFASAMLRole.GetName()}},
		},
	})
	require.NoError(t, err)
	_, err = a.UpsertSAMLConnector(ctx, noMFASAMLConnector)
	require.NoError(t, err)

	noMFASAMLUser.SetCreatedBy(types.CreatedBy{
		Time: a.clock.Now(),
		Connector: &types.ConnectorRef{
			ID:   noMFASAMLConnector.GetName(),
			Type: noMFASAMLConnector.GetKind(),
		},
	})
	_, err = a.UpsertUser(ctx, noMFASAMLUser)
	require.NoError(t, err)

	// Create a fake saml user with MFA enabled.
	samlUser, samlRole, err := CreateUserAndRole(a, "saml-user", []string{"role"}, nil)
	require.NoError(t, err)

	samlConnector, err := types.NewSAMLConnector("saml", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "http://localhost:65535/acs", // not called
		Issuer:                   "test",
		SSO:                      "https://localhost:65535/sso", // not called
		AttributesToRoles: []types.AttributeMapping{
			// not used. can be any name, value but role must exist
			{Name: "groups", Value: "admin", Roles: []string{samlRole.GetName()}},
		},
		MFASettings: &types.SAMLConnectorMFASettings{
			Enabled: true,
			Issuer:  "test",
			Sso:     "https://localhost:65535/sso", // not called
		},
	})
	require.NoError(t, err)
	_, err = a.UpsertSAMLConnector(ctx, samlConnector)
	require.NoError(t, err)

	samlUser.SetCreatedBy(types.CreatedBy{
		Time: a.clock.Now(),
		Connector: &types.ConnectorRef{
			ID:   samlConnector.GetName(),
			Type: samlConnector.GetKind(),
		},
	})
	_, err = a.UpsertUser(ctx, samlUser)
	require.NoError(t, err)

	// Create a fake oidc user with MFA enabled.
	oidcUser, oidcRole, err := CreateUserAndRole(a, "oidc-user", []string{"role"}, nil)
	require.NoError(t, err)

	oidcConnector, err := types.NewOIDCConnector("oidc", types.OIDCConnectorSpecV3{
		ClientID:     "12345",
		ClientSecret: "678910",
		RedirectURLs: []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "test",
				Value: "test",
				Roles: []string{oidcRole.GetName()},
			},
		},
		MFASettings: &types.OIDCConnectorMFASettings{
			Enabled:      true,
			ClientId:     "12345",
			ClientSecret: "678910",
		},
	})
	require.NoError(t, err)
	_, err = a.UpsertOIDCConnector(ctx, oidcConnector)
	require.NoError(t, err)

	oidcUser.SetCreatedBy(types.CreatedBy{
		Time: a.clock.Now(),
		Connector: &types.ConnectorRef{
			ID:   oidcConnector.GetName(),
			Type: oidcConnector.GetKind(),
		},
	})
	_, err = a.UpsertUser(ctx, oidcUser)
	require.NoError(t, err)

	for _, tt := range []struct {
		name             string
		username         string
		setup            func(t *testing.T)
		challengeRequest *proto.CreateAuthenticateChallengeRequest
		assertChallenge  func(t *testing.T, chal *proto.MFAAuthenticateChallenge, err error)
	}{
		{
			name:     "NOK non sso user",
			username: standardUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN, // which scope doesn't matter here.
				},
				SSOClientRedirectURL: "/web/home", // value doesn't matter, as long as it isn't empty.
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge, err error) {
				require.NoError(t, err)
				assert.Nil(t, chal.SSOChallenge)
			},
		},
		{
			name:     "NOK sso mfa not enabled by auth connector",
			username: noMFASAMLUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN, // which scope doesn't matter here.
				},
				SSOClientRedirectURL: "/web/home", // value doesn't matter, as long as it isn't empty.
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge, err error) {
				require.NoError(t, err)
				assert.Nil(t, chal.SSOChallenge)
			},
		},
		{
			name:     "NOK sso mfa not enabled by auth preference",
			username: samlUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN, // which scope doesn't matter here.
				},
				SSOClientRedirectURL: "/web/home", // value doesn't matter, as long as it isn't empty.
			},
			setup: func(t *testing.T) {
				// disable SSO MFA.
				authPref.SetSecondFactors()
				require.NoError(t, err)
				_, err = a.UpsertAuthPreference(ctx, authPref)
				require.NoError(t, err)
				t.Cleanup(func() {
					authPref.SetSecondFactors(types.SecondFactorType_SECOND_FACTOR_TYPE_SSO)
					require.NoError(t, err)
					_, err = a.UpsertAuthPreference(ctx, authPref)
					require.NoError(t, err)
				})
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge, err error) {
				require.NoError(t, err)
				assert.Nil(t, chal.SSOChallenge)
			},
		},
		{
			name:     "NOK SSOClientRedirectURL not provided",
			username: samlUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN, // which scope doesn't matter here.
				},
				SSOClientRedirectURL: "",
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge, err error) {
				require.NoError(t, err)
				assert.Nil(t, chal.SSOChallenge)
			},
		},
		{
			name:     "OK saml user",
			username: samlUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN, // which scope doesn't matter here.
				},
				SSOClientRedirectURL: "/web/home", // value doesn't matter, as long as it isn't empty.
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge, err error) {
				require.NoError(t, err)
				require.NotNil(t, chal.SSOChallenge, "expected SSO challenge to be returned")
				assert.NotEmpty(t, chal.SSOChallenge.RedirectUrl)
				assert.NotEmpty(t, chal.SSOChallenge.RequestId)

				// We should find an auth request with the resulting request ID.
				req, err := a.GetSAMLAuthRequest(ctx, chal.SSOChallenge.RequestId)
				assert.NoError(t, err)
				assert.Equal(t, chal.SSOChallenge.RedirectUrl, req.RedirectURL)
				assert.Equal(t, chal.SSOChallenge.RequestId, req.ID)
				assert.Equal(t, "/web/home", req.ClientRedirectURL)
				assert.Equal(t, samlConnector.GetName(), req.ConnectorID)
				assert.Equal(t, samlConnector.GetKind(), req.Type)
				assert.True(t, req.CheckUser)

				// We should find non validated SSO MFA session data tied to the challenge by auth request ID.
				sd, err := a.GetSSOMFASessionData(ctx, chal.SSOChallenge.RequestId)
				require.NoError(t, err)
				assert.Equal(t, &services.SSOMFASessionData{
					RequestID:     chal.SSOChallenge.RequestId,
					Username:      samlUser.GetName(),
					ConnectorID:   samlConnector.GetName(),
					ConnectorType: samlConnector.GetKind(),
					ChallengeExtensions: &mfatypes.ChallengeExtensions{
						Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					},
				}, sd)
			},
		},
		{
			name:     "OK oidc user",
			username: oidcUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN, // which scope doesn't matter here.
				},
				SSOClientRedirectURL: "/web/home", // value doesn't matter, as long as it isn't empty.
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge, err error) {
				require.NoError(t, err)
				require.NotNil(t, chal.SSOChallenge, "expected SSO challenge to be returned")
				assert.NotEmpty(t, chal.SSOChallenge.RedirectUrl)
				assert.NotEmpty(t, chal.SSOChallenge.RequestId)

				// We should find an auth request with the resulting request ID.
				req, err := a.GetOIDCAuthRequest(ctx, chal.SSOChallenge.RequestId)
				assert.NoError(t, err)
				assert.Equal(t, chal.SSOChallenge.RedirectUrl, req.RedirectURL)
				assert.Equal(t, chal.SSOChallenge.RequestId, req.StateToken)
				assert.Equal(t, "/web/home", req.ClientRedirectURL)
				assert.Equal(t, oidcConnector.GetName(), req.ConnectorID)
				assert.Equal(t, oidcConnector.GetKind(), req.Type)
				assert.True(t, req.CheckUser)

				// We should find non validated SSO MFA session data tied to the challenge by auth request ID.
				sd, err := a.GetSSOMFASessionData(ctx, chal.SSOChallenge.RequestId)
				require.NoError(t, err)
				assert.Equal(t, &services.SSOMFASessionData{
					RequestID:     chal.SSOChallenge.RequestId,
					Username:      oidcUser.GetName(),
					ConnectorID:   oidcConnector.GetName(),
					ConnectorType: oidcConnector.GetKind(),
					ChallengeExtensions: &mfatypes.ChallengeExtensions{
						Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					},
				}, sd)
			},
		},
		{
			name:     "OK allow reuse",
			username: samlUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN, // which scope doesn't matter here.
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
				},
				SSOClientRedirectURL: "/web/home", // value doesn't matter, as long as it isn't empty.
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge, err error) {
				require.NoError(t, err)
				require.NotNil(t, chal.SSOChallenge, "expected SSO challenge to be returned")

				// We should find non validated SSO MFA session data tied to the challenge by auth request ID.
				sd, err := a.GetSSOMFASessionData(ctx, chal.SSOChallenge.RequestId)
				require.NoError(t, err)
				assert.Equal(t, mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES, sd.ChallengeExtensions.AllowReuse)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			userClient, err := testServer.NewClient(TestUser(tt.username))
			require.NoError(t, err)

			if tt.setup != nil {
				tt.setup(t)
			}

			chal, err := userClient.CreateAuthenticateChallenge(ctx, tt.challengeRequest)
			tt.assertChallenge(t, chal, err)
		})
	}
}

func TestSSOMFAChallenge_Validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fakeClock := clockwork.NewFakeClock()
	testAuthServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: fakeClock,
	})
	require.NoError(t, err)
	testServer, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)

	a := testServer.Auth()

	// Create a standard user.
	standardUser, _, err := CreateUserAndRole(a, "standard", []string{"role"}, nil)
	require.NoError(t, err)

	// Create a fake saml user with MFA enabled.
	samlUser, samlRole, err := CreateUserAndRole(a, "saml-user", []string{"role"}, nil)
	require.NoError(t, err)

	samlConnector, err := types.NewSAMLConnector("saml", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "http://localhost:65535/acs", // not called
		Issuer:                   "test",
		SSO:                      "https://localhost:65535/sso", // not called
		AttributesToRoles: []types.AttributeMapping{
			// not used. can be any name, value but role must exist
			{Name: "groups", Value: "admin", Roles: []string{samlRole.GetName()}},
		},
		MFASettings: &types.SAMLConnectorMFASettings{
			Enabled: true,
			Issuer:  "test",
			Sso:     "https://localhost:65535/sso", // not called
		},
	})
	require.NoError(t, err)
	_, err = a.UpsertSAMLConnector(ctx, samlConnector)
	require.NoError(t, err)

	userCreatedAt := a.clock.Now().UTC()
	samlUser.SetCreatedBy(types.CreatedBy{
		Time: userCreatedAt,
		Connector: &types.ConnectorRef{
			ID:   samlConnector.GetName(),
			Type: samlConnector.GetKind(),
		},
	})
	_, err = a.UpsertUser(ctx, samlUser)
	require.NoError(t, err)

	ssoDevice, err := types.NewMFADevice(samlConnector.GetDisplay(), samlConnector.GetName(), userCreatedAt, &types.MFADevice_Sso{
		Sso: &types.SSOMFADevice{
			ConnectorId:   samlConnector.GetName(),
			ConnectorType: samlConnector.GetKind(),
			DisplayName:   samlConnector.GetDisplay(),
		},
	})
	require.NoError(t, err)

	// Create a fake saml user with MFA disabled.
	noMFASAMLUser, noMFASAMLRole, err := CreateUserAndRole(a, "saml-user-no-mfa", []string{"role"}, nil)
	require.NoError(t, err)

	noMFASAMLConnector, err := types.NewSAMLConnector("saml-no-mfa", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "http://localhost:65535/acs", // not called
		Issuer:                   "test",
		SSO:                      "https://localhost:65535/sso", // not called
		AttributesToRoles: []types.AttributeMapping{
			// not used. can be any name, value but role must exist
			{Name: "groups", Value: "admin", Roles: []string{noMFASAMLRole.GetName()}},
		},
	})
	require.NoError(t, err)
	_, err = a.UpsertSAMLConnector(ctx, noMFASAMLConnector)
	require.NoError(t, err)

	noMFASAMLUser.SetCreatedBy(types.CreatedBy{
		Time: a.clock.Now(),
		Connector: &types.ConnectorRef{
			ID:   noMFASAMLConnector.GetName(),
			Type: noMFASAMLConnector.GetKind(),
		},
	})
	_, err = a.UpsertUser(ctx, noMFASAMLUser)
	require.NoError(t, err)

	for _, tt := range []struct {
		name               string
		username           string
		sd                 *services.SSOMFASessionData
		ssoResponse        *proto.SSOResponse
		requiredExtensions *mfav1.ChallengeExtensions
		assertValidation   func(t *testing.T, mad *authz.MFAAuthData, err error)
	}{
		{
			name:        "NOK no required extensions data",
			username:    samlUser.GetName(),
			sd:          nil,
			ssoResponse: nil,
			assertValidation: func(t *testing.T, mad *authz.MFAAuthData, err error) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter error but got %v", err)
			},
		},
		{
			name:     "NOK no session data",
			username: samlUser.GetName(),
			sd:       nil,
			ssoResponse: &proto.SSOResponse{
				RequestId: "unknown",
				Token:     "token",
			},
			requiredExtensions: &mfav1.ChallengeExtensions{},
			assertValidation: func(t *testing.T, mad *authz.MFAAuthData, err error) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name:     "NOK mismatch user",
			username: samlUser.GetName(),
			sd: &services.SSOMFASessionData{
				RequestID:     "request1",
				Username:      "wrong-user",
				ConnectorID:   samlConnector.GetName(),
				ConnectorType: samlConnector.GetKind(),
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				},
				Token: "token",
			},
			ssoResponse: &proto.SSOResponse{
				RequestId: "request1",
				Token:     "token",
			},
			requiredExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			},
			assertValidation: func(t *testing.T, mad *authz.MFAAuthData, err error) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name:     "NOK mismatch token",
			username: samlUser.GetName(),
			sd: &services.SSOMFASessionData{
				RequestID:     "request2",
				Username:      samlUser.GetName(),
				ConnectorID:   samlConnector.GetName(),
				ConnectorType: samlConnector.GetKind(),
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				},
				Token: "token",
			},
			ssoResponse: &proto.SSOResponse{
				RequestId: "request2",
				Token:     "wrong-token",
			},
			requiredExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			},
			assertValidation: func(t *testing.T, mad *authz.MFAAuthData, err error) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name:     "NOK non validated session data",
			username: samlUser.GetName(),
			sd: &services.SSOMFASessionData{
				RequestID:     "request2",
				Username:      samlUser.GetName(),
				ConnectorID:   samlConnector.GetName(),
				ConnectorType: samlConnector.GetKind(),
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				},
			},
			ssoResponse: &proto.SSOResponse{
				RequestId: "request2",
				Token:     "token",
			},
			requiredExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			},
			assertValidation: func(t *testing.T, mad *authz.MFAAuthData, err error) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name:     "NOK mismatch scope",
			username: samlUser.GetName(),
			sd: &services.SSOMFASessionData{
				RequestID:     "request3",
				Username:      samlUser.GetName(),
				ConnectorID:   samlConnector.GetName(),
				ConnectorType: samlConnector.GetKind(),
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				},
				Token: "token",
			},
			ssoResponse: &proto.SSOResponse{
				RequestId: "request3",
				Token:     "token",
			},
			requiredExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
			},
			assertValidation: func(t *testing.T, mad *authz.MFAAuthData, err error) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name:     "NOK reuse not allowed",
			username: samlUser.GetName(),
			sd: &services.SSOMFASessionData{
				RequestID:     "request4",
				Username:      samlUser.GetName(),
				ConnectorID:   samlConnector.GetName(),
				ConnectorType: samlConnector.GetKind(),
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
				},
				Token: "token",
			},
			ssoResponse: &proto.SSOResponse{
				RequestId: "request4",
				Token:     "token",
			},
			requiredExtensions: &mfav1.ChallengeExtensions{
				Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
			},
			assertValidation: func(t *testing.T, mad *authz.MFAAuthData, err error) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name:     "NOK sso mfa not enabled by auth connector",
			username: noMFASAMLUser.GetName(),
			sd: &services.SSOMFASessionData{
				RequestID:     "request5",
				Username:      noMFASAMLUser.GetName(),
				ConnectorID:   noMFASAMLConnector.GetName(),
				ConnectorType: noMFASAMLConnector.GetKind(),
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				},
				Token: "token",
			},
			ssoResponse: &proto.SSOResponse{
				RequestId: "request5",
				Token:     "token",
			},
			requiredExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			},
			assertValidation: func(t *testing.T, mad *authz.MFAAuthData, err error) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name:     "NOK non sso user",
			username: standardUser.GetName(),
			sd: &services.SSOMFASessionData{
				RequestID:     "request6",
				Username:      standardUser.GetName(),
				ConnectorID:   samlConnector.GetName(),
				ConnectorType: samlConnector.GetKind(),
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				},
				Token: "token",
			},
			ssoResponse: &proto.SSOResponse{
				RequestId: "request6",
				Token:     "token",
			},
			requiredExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			},
			assertValidation: func(t *testing.T, mad *authz.MFAAuthData, err error) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name:     "OK sso user",
			username: samlUser.GetName(),
			sd: &services.SSOMFASessionData{
				RequestID:     "request7",
				Username:      samlUser.GetName(),
				ConnectorID:   samlConnector.GetName(),
				ConnectorType: samlConnector.GetKind(),
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
				},
				Token: "token",
			},
			ssoResponse: &proto.SSOResponse{
				RequestId: "request7",
				Token:     "token",
			},
			requiredExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			},
			assertValidation: func(t *testing.T, mad *authz.MFAAuthData, err error) {
				assert.NoError(t, err)
				assert.Equal(t, &authz.MFAAuthData{
					User:       samlUser.GetName(),
					Device:     ssoDevice,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
				}, mad)
			},
		},
		{
			name:     "OK sso user allow reuse",
			username: samlUser.GetName(),
			sd: &services.SSOMFASessionData{
				RequestID:     "request8",
				Username:      samlUser.GetName(),
				ConnectorID:   samlConnector.GetName(),
				ConnectorType: samlConnector.GetKind(),
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
				},
				Token: "token",
			},
			ssoResponse: &proto.SSOResponse{
				RequestId: "request8",
				Token:     "token",
			},
			requiredExtensions: &mfav1.ChallengeExtensions{
				Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
			},
			assertValidation: func(t *testing.T, mad *authz.MFAAuthData, err error) {
				assert.NoError(t, err)
				assert.Equal(t, &authz.MFAAuthData{
					User:       samlUser.GetName(),
					Device:     ssoDevice,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
				}, mad)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sd != nil {
				err := a.UpsertSSOMFASessionData(ctx, tt.sd)
				require.NoError(t, err)
			}

			data, err := a.ValidateMFAAuthResponse(ctx, &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_SSO{
					SSO: tt.ssoResponse,
				},
			}, tt.username, tt.requiredExtensions)
			tt.assertValidation(t, data, err)
		})
	}
}

type fakeSSOService struct {
	a *Server
}

func (s *fakeSSOService) CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error) {
	return nil, nil // unused in these tests.
}

func (s *fakeSSOService) CreateSAMLAuthRequestForMFA(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error) {
	req.ID = uuid.NewString()
	req.RedirectURL = uuid.NewString()
	return &req, s.a.Services.CreateSAMLAuthRequest(ctx, req, defaults.SAMLAuthRequestTTL)
}

func (s *fakeSSOService) ValidateSAMLResponse(ctx context.Context, samlResponse, connectorID, clientIP string) (*authclient.SAMLAuthResponse, error) {
	return nil, nil // unused in these tests.
}

func (s *fakeSSOService) CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	return nil, nil // unused in these tests.
}

func (s *fakeSSOService) CreateOIDCAuthRequestForMFA(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	req.StateToken = uuid.NewString()
	req.RedirectURL = uuid.NewString()
	return &req, s.a.Services.CreateOIDCAuthRequest(ctx, req, defaults.OIDCAuthRequestTTL)
}

func (s *fakeSSOService) ValidateOIDCAuthCallback(ctx context.Context, q url.Values) (*authclient.OIDCAuthResponse, error) {
	return nil, nil // unused in these tests.
}
