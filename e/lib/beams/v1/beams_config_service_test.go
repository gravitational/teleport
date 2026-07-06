package beamsv1

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	beamsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	localservices "github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func newBeamsConfigTestService(t *testing.T, rules []types.Rule, adminState authz.AdminActionAuthState) *BeamsConfigService {
	t.Helper()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	storage, err := localservices.NewBeamsConfigService(bk)
	require.NoError(t, err)
	user, err := types.NewUser("test-user")
	require.NoError(t, err)
	role, err := types.NewRole("test-role", types.RoleSpecV6{
		Allow: types.RoleConditions{Rules: rules},
	})
	require.NoError(t, err)

	authorizer := authz.AuthorizerFunc(func(context.Context) (*authz.Context, error) {
		return &authz.Context{
			User: user,
			Checker: services.NewAccessCheckerWithRoleSet(
				&services.AccessInfo{},
				"test.teleport.sh",
				services.RoleSet{role},
			),
			Identity:             authz.LocalUser{Identity: tlsca.Identity{}},
			AdminActionAuthState: adminState,
		}, nil
	})

	svc, err := NewBeamsConfigService(BeamsConfigServiceConfig{
		Authorizer: authorizer,
		Cache:      storage,
		Backend:    storage,
		Emitter:    libevents.NewDiscardEmitter(),
		Logger:     logtest.NewLogger(),
	})
	require.NoError(t, err)
	return svc
}

func TestBeamsConfigService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		rules        []types.Rule
		adminState   authz.AdminActionAuthState
		readAllowed  bool
		writeAllowed bool
	}{
		{
			name:         "rw access",
			rules:        []types.Rule{types.NewRule(types.KindBeamsConfig, services.RW())},
			adminState:   authz.AdminActionAuthNotRequired,
			readAllowed:  true,
			writeAllowed: true,
		},
		{
			name:         "rw access without admin mfa",
			rules:        []types.Rule{types.NewRule(types.KindBeamsConfig, services.RW())},
			adminState:   authz.AdminActionAuthUnauthorized,
			readAllowed:  true,
			writeAllowed: false,
		},
		{
			name:         "ro access",
			rules:        []types.Rule{types.NewRule(types.KindBeamsConfig, services.RO())},
			adminState:   authz.AdminActionAuthNotRequired,
			readAllowed:  true,
			writeAllowed: false,
		},
		{
			name:        "no access",
			rules:       nil,
			readAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newBeamsConfigTestService(t, tt.rules, tt.adminState)

			t.Run("read", func(t *testing.T) {
				// Storage should return the virtual default.
				getResp, err := svc.GetBeamsConfig(t.Context(), beamsv1pb.GetBeamsConfigRequest_builder{}.Build())
				if !tt.readAllowed {
					require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
					return
				}
				require.NoError(t, err)
				require.Equal(t, "anthropic", getResp.GetBeamsConfig().GetSpec().GetLlm().GetAnthropic().GetAppName())
			})

			t.Run("create", func(t *testing.T) {
				config := services.DefaultBeamsConfig()
				config.GetSpec().GetLlm().GetAnthropic().SetAppName("my-anthropic")
				createResp, err := svc.CreateBeamsConfig(t.Context(), beamsv1pb.CreateBeamsConfigRequest_builder{
					BeamsConfig: config,
				}.Build())

				if !tt.writeAllowed {
					require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
					return
				}
				require.NoError(t, err)
				require.Equal(t, "my-anthropic", createResp.GetBeamsConfig().GetSpec().GetLlm().GetAnthropic().GetAppName())
			})

			t.Run("update", func(t *testing.T) {
				config, err := svc.backend.GetBeamsConfig(t.Context())
				require.NoError(t, err)

				toUpdate := proto.Clone(config).(*beamsv1pb.BeamsConfig)
				toUpdate.GetSpec().GetLlm().GetAnthropic().SetAppName("updated-anthropic")
				updateResp, err := svc.UpdateBeamsConfig(t.Context(), beamsv1pb.UpdateBeamsConfigRequest_builder{
					BeamsConfig: toUpdate,
				}.Build())

				if !tt.writeAllowed {
					require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
					return
				}
				require.NoError(t, err)
				require.Equal(t, "updated-anthropic", updateResp.GetBeamsConfig().GetSpec().GetLlm().GetAnthropic().GetAppName())
			})

			t.Run("delete", func(t *testing.T) {
				_, err := svc.DeleteBeamsConfig(t.Context(), beamsv1pb.DeleteBeamsConfigRequest_builder{}.Build())
				if !tt.writeAllowed {
					require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
					return
				}
				require.NoError(t, err)
			})
		})
	}
}
