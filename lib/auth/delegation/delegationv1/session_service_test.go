/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package delegationv1_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	delegationv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/delegation/delegationv1"
	"github.com/gravitational/teleport/lib/auth/internal"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/trace"
)

func sessionServiceTestPack(t *testing.T) (*delegationv1.SessionService, *sessionTestPack) {
	t.Helper()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	profileUpstream, err := local.NewDelegationProfileService(backend)
	require.NoError(t, err)

	sessionUpstream, err := local.NewDelegationSessionService(backend)
	require.NoError(t, err)

	accessService := local.NewAccessService(backend)
	require.NoError(t, err)

	presenceService := local.NewPresenceService(backend)
	require.NoError(t, err)

	identityService, err := local.NewIdentityService(backend)
	require.NoError(t, err)

	appServer, err := types.NewAppServerV3(
		types.Metadata{Name: "hr-system"},
		types.AppServerSpecV3{
			HostID: uuid.NewString(),
			App: &types.AppV3{
				Metadata: types.Metadata{Name: "hr-system"},
				Spec:     types.AppSpecV3{URI: "https://hr-system"},
			},
		},
	)
	require.NoError(t, err)

	_, err = presenceService.UpsertApplicationServer(t.Context(), appServer)
	require.NoError(t, err)

	pack := &sessionTestPack{
		profiles: profileUpstream,
		sessions: sessionUpstream,
		access:   accessService,
		presence: presenceService,
		identity: identityService,
	}

	service, err := delegationv1.NewSessionService(delegationv1.SessionServiceConfig{
		Authorizer: authz.AuthorizerFunc(func(context.Context) (*authz.Context, error) {
			if pack.botName != "" {
				return &authz.Context{
					Identity: authz.LocalUser{
						Identity: tlsca.Identity{BotName: pack.botName},
					},
					AdminActionAuthState: authz.AdminActionAuthUnauthorized,
				}, nil
			}

			if pack.user != nil {
				checker, err := services.NewAccessChecker(
					&services.AccessInfo{
						Roles: pack.user.GetRoles(),
					},
					"test.teleport.sh",
					accessService,
				)
				require.NoError(t, err)

				return &authz.Context{
					User:                 pack.user,
					AdminActionAuthState: pack.adminActionAuthState,
					Checker:              checker,
				}, nil
			}

			return nil, trace.AccessDenied("remember to call authenticateUser or authenticateBot on the test pack")
		}),
		ProfileReader:  profileUpstream,
		SessionReader:  sessionUpstream,
		SessionWriter:  sessionUpstream,
		ResourceLister: presenceService,
		RoleGetter:     accessService,
		UserGetter:     identityService,
		CertGenerator: delegationv1.CertGeneratorFunc(func(ctx context.Context, req internal.CertRequest) (*proto.Certs, error) {
			if pack.onGenerateCert != nil {
				return pack.onGenerateCert(ctx, req)
			}
			return nil, trace.NotImplemented("Certificate generation not implemented")
		}),
		ClusterNameGetter: testClusterNameGetter{clusterName: "test.teleport.sh"},
		AppSessionCreator: delegationv1.AppSessionCreatorFunc(func(ctx context.Context, req internal.NewAppSessionRequest) (types.WebSession, error) {
			if pack.onCreateAppSession != nil {
				return pack.onCreateAppSession(ctx, req)
			}
			return nil, trace.NotImplemented("App session creation not implemented")
		}),
		Logger: logtest.NewLogger(),
	})

	return service, pack
}

type sessionTestPack struct {
	profiles services.DelegationProfiles
	sessions services.DelegationSessions
	access   services.Access
	presence services.Presence
	identity services.Identity

	user                 types.User
	adminActionAuthState authz.AdminActionAuthState
	botName              string

	onCreateAppSession func(context.Context, internal.NewAppSessionRequest) (types.WebSession, error)
	onGenerateCert     func(context.Context, internal.CertRequest) (*proto.Certs, error)
}

func (p *sessionTestPack) authenticateUser(
	t *testing.T,
	name string,
	mfaState authz.AdminActionAuthState,
	roleSpec types.RoleSpecV6,
) {
	t.Helper()

	p.user = p.createUser(t, name, roleSpec)
	p.adminActionAuthState = mfaState
}

func (p *sessionTestPack) createUser(
	t *testing.T,
	name string,
	roleSpec types.RoleSpecV6,
) types.User {
	t.Helper()

	user, err := types.NewUser(name)
	require.NoError(t, err)

	role, err := types.NewRole(name, roleSpec)
	require.NoError(t, err)
	user.AddRole(role.GetName())

	_, err = p.access.CreateRole(t.Context(), role)
	require.NoError(t, err)

	_, err = p.identity.CreateUser(t.Context(), user)
	require.NoError(t, err)

	return user
}

func (p *sessionTestPack) authenticateBot(botName string) {
	p.botName = botName
}

func (p *sessionTestPack) createSession(t *testing.T, spec *delegationv1pb.DelegationSessionSpec) *delegationv1pb.DelegationSession {
	t.Helper()

	session, err := p.sessions.CreateDelegationSession(t.Context(), &delegationv1pb.DelegationSession{
		Kind:    types.KindDelegationProfile,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    uuid.NewString(),
			Expires: timestamppb.New(time.Now().Add(time.Hour)),
		},
		Spec: spec,
	})
	require.NoError(t, err)

	return session
}

type testClusterNameGetter struct {
	clusterName string
}

func (g testClusterNameGetter) GetClusterName(context.Context) (types.ClusterName, error) {
	return types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: g.clusterName,
		ClusterID:   g.clusterName,
	})
}
