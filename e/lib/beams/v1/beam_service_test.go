package beamsv1

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/types"
	compute "github.com/gravitational/teleport/e/api/beamservice/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	localservices "github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type beamServiceTestPack struct {
	aliasGenerator    func() (string, error)
	backend           *memory.Memory
	compute           *fakeComputeService
	beam              *localservices.BeamService
	app               *localservices.AppService
	token             *localservices.ProvisioningService
	identity          *localservices.IdentityService
	role              *localservices.AccessService
	workloadIdentity  *localservices.WorkloadIdentityService
	delegationSession *localservices.DelegationSessionService
	presence          *localservices.PresenceService
}

type beamServiceTestPackConfig struct {
	aliasGenerator func() (string, error)
	computeClient  *fakeComputeService
}

func newBeamServiceTestPack(t *testing.T, cfg beamServiceTestPackConfig) *beamServiceTestPack {
	t.Helper()

	if cfg.aliasGenerator == nil {
		cfg.aliasGenerator = sequenceAliasGenerator("warm-orbit", "sparkling-zephyr")
	}

	if cfg.computeClient == nil {
		cfg.computeClient = &fakeComputeService{
			provisionResponse: &compute.ProvisionBeamResponse{
				SshAddr:     "127.0.0.1:3022",
				AppAddrHttp: "127.0.0.1:8443",
				AppAddrTcp:  "127.0.0.1:8444",
			},
		}
	}

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, backend.Close()) })

	beamService, err := localservices.NewBeamService(backend)
	require.NoError(t, err)

	userService, err := localservices.NewIdentityService(backend)
	require.NoError(t, err)

	accessService := localservices.NewAccessService(backend)

	workloadIdentityService, err := localservices.NewWorkloadIdentityService(backend)
	require.NoError(t, err)

	delegationSessionService, err := localservices.NewDelegationSessionService(backend)
	require.NoError(t, err)

	pack := &beamServiceTestPack{
		aliasGenerator:    cfg.aliasGenerator,
		backend:           backend,
		compute:           cfg.computeClient,
		beam:              beamService,
		app:               localservices.NewAppService(backend),
		token:             localservices.NewProvisioningService(backend),
		identity:          userService,
		role:              accessService,
		workloadIdentity:  workloadIdentityService,
		delegationSession: delegationSessionService,
		presence:          localservices.NewPresenceService(backend),
	}

	return pack
}

func (p *beamServiceTestPack) user(t *testing.T, name string) types.User {
	t.Helper()

	user, err := types.NewUser(name)
	require.NoError(t, err)

	role := beamUserRole(t, user.GetName())
	user.SetRoles([]string{role.GetName()})

	_, err = p.role.CreateRole(t.Context(), role)
	require.NoError(t, err)

	return user
}

func (p *beamServiceTestPack) service(t *testing.T, user types.User) *BeamsService {
	authorizer := authz.AuthorizerFunc(func(context.Context) (*authz.Context, error) {
		checker, err := services.NewAccessChecker(
			&services.AccessInfo{
				Roles: user.GetRoles(),
			},
			"test.teleport.sh",
			p.role,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &authz.Context{
			User:     user,
			Checker:  checker,
			Identity: authz.LocalUser{Identity: tlsca.Identity{}},
		}, nil
	})

	service, err := NewBeamService(BeamsServiceConfig{
		AuthPreferenceGetter:    testAuthPreferenceGetter{},
		BeamReader:              p.beam,
		StorageBackend:          p.backend,
		AppWriter:               p.app,
		BeamWriter:              p.beam,
		DelegationSessionWriter: p.delegationSession,
		ProvisionTokenWriter:    p.token,
		RoleWriter:              p.role,
		UserWriter:              p.identity,
		NodeWriter:              p.presence,
		WorkloadIdentityWriter:  p.workloadIdentity,
		ComputeServiceClient:    p.compute,
		Authorizer:              authorizer,
		AliasGenerator:          p.aliasGenerator,
		Logger:                  logtest.NewLogger(),
	})
	require.NoError(t, err)

	return service
}

type testAuthPreferenceGetter struct{}

func (testAuthPreferenceGetter) GetReadOnlyAuthPreference(context.Context) (readonly.AuthPreference, error) {
	return types.DefaultAuthPreference(), nil
}

type fakeComputeService struct {
	mu                sync.Mutex
	provisionRequests []*compute.ProvisionBeamRequest
	destroyRequests   []*compute.DestroyBeamRequest
	provisionResponse *compute.ProvisionBeamResponse
	provisionError    error
	destroyError      error
}

func (f *fakeComputeService) ProvisionBeam(ctx context.Context, req *compute.ProvisionBeamRequest, _ ...grpc.CallOption) (*compute.ProvisionBeamResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.provisionRequests = append(f.provisionRequests, req)
	if f.provisionError != nil {
		return nil, f.provisionError
	}
	return f.provisionResponse, nil
}

func (f *fakeComputeService) DestroyBeam(_ context.Context, req *compute.DestroyBeamRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.destroyRequests = append(f.destroyRequests, req)
	if f.destroyError != nil {
		return nil, f.destroyError
	}
	return &emptypb.Empty{}, nil
}

func (f *fakeComputeService) getProvisionRequests() []*compute.ProvisionBeamRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.provisionRequests)
}

func (f *fakeComputeService) getDestroyRequests() []*compute.DestroyBeamRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.destroyRequests)
}

func sequenceAliasGenerator(aliases ...string) func() (string, error) {
	var (
		mu    sync.Mutex
		index int
	)

	return func() (string, error) {
		mu.Lock()
		defer mu.Unlock()

		if index >= len(aliases) {
			return "", errors.New("all available aliases have been used")
		}

		alias := aliases[index]
		index++

		return alias, nil
	}
}

func beamUserRole(t *testing.T, user string) types.Role {
	t.Helper()

	role, err := types.NewRole(
		fmt.Sprintf("beam-user-%s", user),
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				BeamLabels: types.Labels{
					types.BeamOwnerLabel: {user},
				},
				Rules: []types.Rule{
					{
						Resources: []string{types.KindBeam},
						Verbs:     []string{types.Wildcard},
					},
				},
			},
		},
	)
	require.NoError(t, err)

	return role
}
