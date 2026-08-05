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
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
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
	computeProvider   ComputeServiceClientProvider
	validRegions      []string
	defaultRegion     string
	usageReporter     usagereporter.UsageReporter
}

type beamServiceTestPackConfig struct {
	aliasGenerator  func() (string, error)
	computeClient   *fakeComputeService
	noComputeClient bool
	computeProvider ComputeServiceClientProvider
	validRegions    []string
	defaultRegion   string
	usageReporter   usagereporter.UsageReporter
}

// recordingUsageReporter captures events submitted via AnonymizeAndSubmit.
type recordingUsageReporter struct {
	mu     sync.Mutex
	events []usagereporter.Anonymizable
}

func (r *recordingUsageReporter) AnonymizeAndSubmit(events ...usagereporter.Anonymizable) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, events...)
}

func (r *recordingUsageReporter) recorded() []usagereporter.Anonymizable {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]usagereporter.Anonymizable(nil), r.events...)
}

func newBeamServiceTestPack(t *testing.T, cfg beamServiceTestPackConfig) *beamServiceTestPack {
	t.Helper()

	if cfg.aliasGenerator == nil {
		cfg.aliasGenerator = sequenceAliasGenerator("warm-orbit", "sparkling-zephyr")
	}

	if cfg.computeClient == nil && !cfg.noComputeClient {
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

	ur := cfg.usageReporter
	if ur == nil {
		ur = usagereporter.DiscardUsageReporter{}
	}

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
		computeProvider:   cfg.computeProvider,
		validRegions:      cfg.validRegions,
		defaultRegion:     cfg.defaultRegion,
		usageReporter:     ur,
	}

	return pack
}

func (p *beamServiceTestPack) admin(t *testing.T) types.User {
	t.Helper()

	role := beamAdminRole(t)
	_, err := p.role.CreateRole(t.Context(), role)
	require.NoError(t, err)

	user, err := types.NewUser("admin")
	require.NoError(t, err)
	user.SetRoles([]string{role.GetName()})

	return user
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

func (p *beamServiceTestPack) nonBeamUser(t *testing.T) types.User {
	t.Helper()

	role := nonBeamUserRole(t)
	_, err := p.role.CreateRole(t.Context(), role)
	require.NoError(t, err)

	user, err := types.NewUser("non-beam-user")
	require.NoError(t, err)
	user.SetRoles([]string{role.GetName()})

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

	var computeClient compute.BeamsOrchestratorServiceClient
	if p.compute != nil {
		computeClient = p.compute
	}
	service, err := NewBeamService(BeamsServiceConfig{
		ClusterName:                  "dunder-mifflin.beams.run",
		AuthPreferenceGetter:         testAuthPreferenceGetter{},
		BeamReader:                   p.beam,
		StorageBackend:               p.backend,
		AppWriter:                    p.app,
		BeamWriter:                   p.beam,
		DelegationSessionWriter:      p.delegationSession,
		ProvisionTokenWriter:         p.token,
		RoleWriter:                   p.role,
		UserWriter:                   p.identity,
		NodeWriter:                   p.presence,
		WorkloadIdentityWriter:       p.workloadIdentity,
		ComputeServiceClient:         computeClient,
		ComputeServiceClientProvider: p.computeProvider,
		Authorizer:                   authorizer,
		UsageReporter:                p.usageReporter,
		AliasGenerator:               p.aliasGenerator,
		ValidRegions:                 p.validRegions,
		DefaultRegion:                p.defaultRegion,
		Logger:                       logtest.NewLogger(),
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
	getInfoRequests   []*compute.GetInfoRequest
	provisionRequests []*compute.ProvisionBeamRequest
	destroyRequests   []*compute.DestroyBeamRequest
	getInfoResponse   *compute.GetInfoResponse
	getInfoError      error
	provisionResponse *compute.ProvisionBeamResponse
	provisionError    error
	destroyError      error
}

type fakeComputeServiceProvider struct {
	mu      sync.Mutex
	regions []string
	client  compute.BeamsOrchestratorServiceClient
	err     error
}

func (f *fakeComputeServiceProvider) ClientForRegion(region string) (compute.BeamsOrchestratorServiceClient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.regions = append(f.regions, region)
	if f.err != nil {
		return nil, f.err
	}
	return f.client, nil
}

func (f *fakeComputeServiceProvider) getRegions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.regions)
}

func (f *fakeComputeService) CreateBeam(ctx context.Context, in *compute.ProvisionBeamRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (f *fakeComputeService) WaitForBeamProvision(ctx context.Context, in *compute.WaitForBeamProvisionRequest, opts ...grpc.CallOption) (compute.BeamsOrchestratorService_WaitForBeamProvisionClient, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (f *fakeComputeService) GetInfo(ctx context.Context, req *compute.GetInfoRequest, _ ...grpc.CallOption) (*compute.GetInfoResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.getInfoRequests = append(f.getInfoRequests, req)
	if f.getInfoError != nil {
		return nil, f.getInfoError
	}
	return f.getInfoResponse, nil
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

func (f *fakeComputeService) getGetInfoRequests() []*compute.GetInfoRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.getInfoRequests)
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

func nonBeamUserRole(t *testing.T) types.Role {
	t.Helper()

	// This role is missing the rules that allow the user to access any beams
	// at all (despite the label wildcard).
	role, err := types.NewRole(
		"non-beam-user",
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				BeamLabels: types.Labels{
					types.BeamOwnerLabel: {types.Wildcard},
				},
			},
		},
	)
	require.NoError(t, err)

	return role
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

func beamAdminRole(t *testing.T) types.Role {
	t.Helper()

	role, err := types.NewRole(
		"beam-admin",
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				BeamLabels: types.Labels{
					types.BeamOwnerLabel: {types.Wildcard},
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
