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

package gitserverv1

import (
	"context"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func newServer(t *testing.T, org string) *types.ServerV2 {
	server, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Integration:  org,
		Organization: org,
	})
	require.NoError(t, err)
	serverV2, ok := server.(*types.ServerV2)
	require.True(t, ok)
	return serverV2
}

func TestServiceAccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	org1 := newServer(t, "org1")
	org2 := newServer(t, "org2")
	org3 := newServer(t, "org3")

	testCases := []struct {
		name    string
		checker services.AccessChecker
		run     func(*testing.T, *Service)
	}{
		{
			name:    "create verb not allowed",
			checker: &fakeAccessChecker{ /*nothing allowed*/ },
			run: func(t *testing.T, service *Service) {
				_, err := service.CreateGitServer(ctx, &pb.CreateGitServerRequest{Server: org3})
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "create success",
			checker: &fakeAccessChecker{
				allowVerbs: []string{types.VerbCreate},
			},
			run: func(t *testing.T, service *Service) {
				_, err := service.CreateGitServer(ctx, &pb.CreateGitServerRequest{Server: org3})
				require.NoError(t, err)
			},
		},
		{
			name:    "get verb not allowed",
			checker: &fakeAccessChecker{ /*nothing allowed*/ },
			run: func(t *testing.T, service *Service) {
				_, err := service.GetGitServer(ctx, &pb.GetGitServerRequest{Name: org1.GetName()})
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "get resource denied",
			checker: &fakeAccessChecker{
				allowVerbs: []string{types.VerbRead},
			},
			run: func(t *testing.T, service *Service) {
				_, err := service.GetGitServer(ctx, &pb.GetGitServerRequest{Name: org1.GetName()})
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name: "get success",
			checker: &fakeAccessChecker{
				allowVerbs:    []string{types.VerbRead},
				allowResource: true,
			},
			run: func(t *testing.T, service *Service) {
				server, err := service.GetGitServer(ctx, &pb.GetGitServerRequest{Name: org1.GetName()})
				require.NoError(t, err)
				require.Equal(t, "org1", server.GetGitHub().Organization)
			},
		},
		{
			name: "list verb not allowed",
			checker: &fakeAccessChecker{
				allowVerbs:    []string{types.VerbRead},
				allowResource: true,
			},
			run: func(t *testing.T, service *Service) {
				_, err := service.ListGitServers(ctx, &pb.ListGitServersRequest{})
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "list resource denied",
			checker: &fakeAccessChecker{
				allowVerbs: []string{types.VerbRead, types.VerbList},
			},
			run: func(t *testing.T, service *Service) {
				resp, err := service.ListGitServers(ctx, &pb.ListGitServersRequest{})
				require.NoError(t, err)
				require.Empty(t, resp.Servers)
			},
		},
		{
			name: "list success",
			checker: &fakeAccessChecker{
				allowVerbs:    []string{types.VerbRead, types.VerbList},
				allowResource: true,
			},
			run: func(t *testing.T, service *Service) {
				resp, err := service.ListGitServers(ctx, &pb.ListGitServersRequest{})
				require.NoError(t, err)
				require.Len(t, resp.Servers, 2)
			},
		},
		{
			name:    "update verb not allowed",
			checker: &fakeAccessChecker{ /*nothing allowed*/ },
			run: func(t *testing.T, service *Service) {
				_, err := service.UpdateGitServer(ctx, &pb.UpdateGitServerRequest{Server: org1})
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "update success",
			checker: &fakeAccessChecker{
				allowVerbs: []string{types.VerbUpdate},
			},
			run: func(t *testing.T, service *Service) {
				org1WithRevision, err := service.cfg.Backend.GetGitServer(ctx, org1.GetName())
				require.NoError(t, err)
				_, err = service.UpdateGitServer(ctx, &pb.UpdateGitServerRequest{Server: org1WithRevision.(*types.ServerV2)})
				require.NoError(t, err)
			},
		},
		{
			name:    "upsert verb not allowed",
			checker: &fakeAccessChecker{ /*nothing allowed*/ },
			run: func(t *testing.T, service *Service) {
				_, err := service.UpsertGitServer(ctx, &pb.UpsertGitServerRequest{Server: org3})
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "upsert success",
			checker: &fakeAccessChecker{
				allowVerbs: []string{types.VerbCreate, types.VerbUpdate},
			},
			run: func(t *testing.T, service *Service) {
				_, err := service.UpsertGitServer(ctx, &pb.UpsertGitServerRequest{Server: org3})
				require.NoError(t, err)
			},
		},
		{
			name:    "delete verb not allowed",
			checker: &fakeAccessChecker{ /*nothing allowed*/ },
			run: func(t *testing.T, service *Service) {
				_, err := service.DeleteGitServer(ctx, &pb.DeleteGitServerRequest{Name: org1.GetName()})
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "delete success",
			checker: &fakeAccessChecker{
				allowVerbs: []string{types.VerbDelete},
			},
			run: func(t *testing.T, service *Service) {
				_, err := service.DeleteGitServer(ctx, &pb.DeleteGitServerRequest{Name: org1.GetName()})
				require.NoError(t, err)
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			service := newService(t, test.checker, org1, org2)
			test.run(t, service)
		})
	}
}

type fakeAccessChecker struct {
	services.AccessChecker
	allowVerbs    []string
	allowResource bool
}

func (f fakeAccessChecker) CheckAccessToRule(_ services.RuleContext, _ string, _ string, verb string) error {
	if !slices.Contains(f.allowVerbs, verb) {
		return trace.AccessDenied("verb %s not allowed", verb)
	}
	return nil
}
func (f fakeAccessChecker) CheckAccess(services.AccessCheckable, services.AccessState, ...services.RoleMatcher) error {
	if !f.allowResource {
		return trace.AccessDenied("access denied")
	}
	return nil
}

func newService(t *testing.T, checker services.AccessChecker, existing ...*types.ServerV2) *Service {
	t.Helper()

	b, err := memory.New(memory.Config{})
	require.NoError(t, err)

	backendService, err := local.NewGitServerService(b)
	require.NoError(t, err)

	for _, server := range existing {
		_, err := backendService.CreateGitServer(context.Background(), server)
		require.NoError(t, err)
	}

	authorizer := authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
		user, err := types.NewUser("Linus.Torvalds")
		if err != nil {
			return nil, err
		}
		return &authz.Context{
			User:    user,
			Checker: checker,
		}, nil
	})

	service, err := NewService(Config{
		Authorizer: authorizer,
		Backend:    backendService,
	})
	require.NoError(t, err)
	return service
}
