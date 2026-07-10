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

package discoveryservicev1

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryservicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

const testClusterName = "test-cluster"

func sampleHeartbeat(hostID string) *discoveryservicepb.DiscoveryService {
	return discoveryservicepb.DiscoveryService_builder{
		Kind:    types.KindDiscoveryService,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: hostID,
		}.Build(),
		Spec: discoveryservicepb.DiscoveryServiceSpec_builder{
			Hostname:       "disc-1.example.com",
			DiscoveryGroup: "demo",
		}.Build(),
	}.Build()
}

// TestUpsertAuthz validates the RFD 253 write-path claims: only the Discovery
// builtin role may upsert, only for its own host ID, and no human role can
// write regardless of RBAC verbs.
func TestUpsertAuthz(t *testing.T) {
	t.Parallel()
	ctx, svc, lc := initSvc(t)

	hostID := uuid.NewString()

	t.Run("discovery builtin can upsert its own resource", func(t *testing.T) {
		callCtx := builtinCtx(ctx, types.RoleDiscovery, hostID)
		got, err := svc.UpsertDiscoveryService(callCtx, discoveryservicepb.UpsertDiscoveryServiceRequest_builder{
			DiscoveryService: sampleHeartbeat(hostID),
		}.Build())
		require.NoError(t, err)
		require.Equal(t, hostID, got.GetMetadata().GetName())
	})

	t.Run("discovery builtin cannot upsert another host's resource", func(t *testing.T) {
		callCtx := builtinCtx(ctx, types.RoleDiscovery, hostID)
		_, err := svc.UpsertDiscoveryService(callCtx, discoveryservicepb.UpsertDiscoveryServiceRequest_builder{
			DiscoveryService: sampleHeartbeat(uuid.NewString()),
		}.Build())
		require.Error(t, err)
		require.True(t, trace_IsAccessDenied(err), "expected AccessDenied, got %v", err)
	})

	t.Run("other builtin roles cannot upsert", func(t *testing.T) {
		callCtx := builtinCtx(ctx, types.RoleProxy, hostID)
		_, err := svc.UpsertDiscoveryService(callCtx, discoveryservicepb.UpsertDiscoveryServiceRequest_builder{
			DiscoveryService: sampleHeartbeat(hostID),
		}.Build())
		require.Error(t, err)
		require.True(t, trace_IsAccessDenied(err), "expected AccessDenied, got %v", err)
	})

	t.Run("human role cannot upsert even with full RBAC verbs", func(t *testing.T) {
		callCtx := userCtx(t, ctx, lc, types.RoleSpecV6{
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					types.NewRule(types.KindDiscoveryService, services.RW()),
				},
			},
		})
		_, err := svc.UpsertDiscoveryService(callCtx, discoveryservicepb.UpsertDiscoveryServiceRequest_builder{
			DiscoveryService: sampleHeartbeat(hostID),
		}.Build())
		require.Error(t, err)
		require.True(t, trace_IsAccessDenied(err), "expected AccessDenied, got %v", err)
	})

	t.Run("upsert forces kind version and clears status", func(t *testing.T) {
		callCtx := builtinCtx(ctx, types.RoleDiscovery, hostID)
		hb := sampleHeartbeat(hostID)
		hb.SetKind("bogus")
		hb.SetVersion("v99")
		got, err := svc.UpsertDiscoveryService(callCtx, discoveryservicepb.UpsertDiscoveryServiceRequest_builder{
			DiscoveryService: hb,
		}.Build())
		require.NoError(t, err)
		require.Equal(t, types.KindDiscoveryService, got.GetKind())
		require.Equal(t, types.V1, got.GetVersion())
		require.NotNil(t, got.GetStatus())
	})
}

// TestReadAuthz validates the read-path claims: reads follow standard RBAC
// verbs; a role without verbs is denied.
func TestReadAuthz(t *testing.T) {
	t.Parallel()
	ctx, svc, lc := initSvc(t)

	hostID := uuid.NewString()
	_, err := svc.UpsertDiscoveryService(builtinCtx(ctx, types.RoleDiscovery, hostID), discoveryservicepb.UpsertDiscoveryServiceRequest_builder{
		DiscoveryService: sampleHeartbeat(hostID),
	}.Build())
	require.NoError(t, err)

	readerCtx := userCtx(t, ctx, lc, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindDiscoveryService, services.RO()),
			},
		},
	})
	noVerbsCtx := userCtx(t, ctx, lc, types.RoleSpecV6{})

	t.Run("role with read/list can get and list", func(t *testing.T) {
		got, err := svc.GetDiscoveryService(readerCtx, discoveryservicepb.GetDiscoveryServiceRequest_builder{Name: hostID}.Build())
		require.NoError(t, err)
		require.Equal(t, hostID, got.GetMetadata().GetName())

		resp, err := svc.ListDiscoveryServices(readerCtx, discoveryservicepb.ListDiscoveryServicesRequest_builder{PageSize: 10}.Build())
		require.NoError(t, err)
		require.Len(t, resp.GetDiscoveryServices(), 1)
	})

	t.Run("role without verbs is denied", func(t *testing.T) {
		_, err := svc.GetDiscoveryService(noVerbsCtx, discoveryservicepb.GetDiscoveryServiceRequest_builder{Name: hostID}.Build())
		require.Error(t, err)
		_, err = svc.ListDiscoveryServices(noVerbsCtx, discoveryservicepb.ListDiscoveryServicesRequest_builder{PageSize: 10}.Build())
		require.Error(t, err)
	})

	t.Run("role without delete verb cannot delete", func(t *testing.T) {
		_, err := svc.DeleteDiscoveryService(readerCtx, discoveryservicepb.DeleteDiscoveryServiceRequest_builder{Name: hostID}.Build())
		require.Error(t, err)
	})
}

// TestExpiryIsLiveness validates the "expiry is the liveness signal" claim
// against a real backend with a fake clock: a heartbeat that stops being
// renewed becomes unreadable after its TTL.
func TestExpiryIsLiveness(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{Context: ctx, Clock: clock})
	require.NoError(t, err)
	backendSvc, err := local.NewDiscoveryServiceService(mem)
	require.NoError(t, err)

	hb := sampleHeartbeat(uuid.NewString())
	hb.GetMetadata().SetExpires(timestamppb.New(clock.Now().UTC().Add(90 * time.Second)))
	_, err = backendSvc.UpsertDiscoveryService(ctx, hb)
	require.NoError(t, err)

	// Alive within TTL.
	_, err = backendSvc.GetDiscoveryService(ctx, hb.GetMetadata().GetName())
	require.NoError(t, err)

	// Dead after TTL: absence is the signal.
	clock.Advance(91 * time.Second)
	_, err = backendSvc.GetDiscoveryService(ctx, hb.GetMetadata().GetName())
	require.Error(t, err, "expired heartbeat must not be readable")
}

func trace_IsAccessDenied(err error) bool {
	return trace.IsAccessDenied(err)
}

func builtinCtx(ctx context.Context, role types.SystemRole, hostID string) context.Context {
	return authz.ContextWithUser(ctx, authz.BuiltinRole{
		Username:    hostID + "." + testClusterName,
		ClusterName: testClusterName,
		Role:        role,
		Identity: tlsca.Identity{
			Username:    hostID + "." + testClusterName,
			SystemRoles: []string{string(role)},
			Groups:      []string{string(role)},
		},
	})
}

func userCtx(t *testing.T, ctx context.Context, lc localClient, roleSpec types.RoleSpecV6) context.Context {
	roleName := "role-" + uuid.NewString()
	role, err := types.NewRole(roleName, roleSpec)
	require.NoError(t, err)
	role, err = lc.CreateRole(ctx, role)
	require.NoError(t, err)

	user, err := types.NewUser("user-" + uuid.NewString())
	require.NoError(t, err)
	user.AddRole(roleName)
	user, err = lc.CreateUser(ctx, user)
	require.NoError(t, err)

	return authz.ContextWithUser(ctx, authz.LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username: user.GetName(),
			Groups:   []string{role.GetName()},
		},
	})
}

type localClient interface {
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	CreateRole(ctx context.Context, role types.Role) (types.Role, error)
}

type testAccessPoint struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
}

func initSvc(t *testing.T) (context.Context, *Service, localClient) {
	ctx := context.Background()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	trustSvc := local.NewCAService(bk)
	roleSvc := local.NewAccessService(bk)
	userSvc, err := local.NewTestIdentityService(bk)
	require.NoError(t, err)

	clusterConfigSvc, err := local.NewClusterConfigurationService(bk)
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	require.NoError(t, clusterConfigSvc.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	_, err = clusterConfigSvc.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	accessPoint := &testAccessPoint{
		ClusterConfiguration: clusterConfigSvc,
		Trust:                trustSvc,
		RoleGetter:           roleSvc,
		UserGetter:           userSvc,
	}

	eventService := local.NewEventsService(bk)
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Client:    eventService,
			Component: "test",
		},
		LockGetter: roleSvc,
	})
	require.NoError(t, err)

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: testClusterName,
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	backendSvc, err := local.NewDiscoveryServiceService(bk)
	require.NoError(t, err)

	svc, err := NewService(ServiceConfig{
		Backend:    backendSvc,
		Authorizer: authorizer,
	})
	require.NoError(t, err)

	return ctx, svc, struct {
		*local.AccessService
		*local.IdentityService
	}{
		AccessService:   roleSvc,
		IdentityService: userSvc,
	}
}
