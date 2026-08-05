package beamsv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	compute "github.com/gravitational/teleport/e/api/beamservice/v1"
	"github.com/gravitational/teleport/e/lib/beams/v1/alias"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// NewBeamService creates a new BeamService with the given configuration.
func NewBeamService(cfg BeamsServiceConfig) (*BeamsService, error) {
	switch {
	case cfg.ClusterName == "":
		return nil, trace.BadParameter("ClusterName is required")
	case cfg.AuthPreferenceGetter == nil:
		return nil, trace.BadParameter("AuthPreferenceGetter is required")
	case cfg.BeamReader == nil:
		return nil, trace.BadParameter("BeamReader is required")
	case cfg.StorageBackend == nil:
		return nil, trace.BadParameter("StorageBackend is required")
	case cfg.AppWriter == nil:
		return nil, trace.BadParameter("AppWriter is required")
	case cfg.BeamWriter == nil:
		return nil, trace.BadParameter("BeamWriter is required")
	case cfg.DelegationSessionWriter == nil:
		return nil, trace.BadParameter("DelegationSessionWriter is required")
	case cfg.ProvisionTokenWriter == nil:
		return nil, trace.BadParameter("ProvisionTokenWriter is required")
	case cfg.RoleWriter == nil:
		return nil, trace.BadParameter("RoleWriter is required")
	case cfg.UserWriter == nil:
		return nil, trace.BadParameter("UserWriter is required")
	case cfg.NodeWriter == nil:
		return nil, trace.BadParameter("NodeWriter is required")
	case cfg.WorkloadIdentityWriter == nil:
		return nil, trace.BadParameter("WorkloadIdentityWriter is required")
	case cfg.ComputeServiceClient == nil && cfg.ComputeServiceClientProvider == nil:
		return nil, trace.BadParameter("ComputeServiceClient or ComputeServiceClientProvider is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("Authorizer is required")
	case cfg.UsageReporter == nil:
		return nil, trace.BadParameter("UsageReporter is required")
	case cfg.Logger == nil:
		return nil, trace.BadParameter("Logger is required")
	}

	if cfg.AliasGenerator == nil {
		cfg.AliasGenerator = alias.Generate
	}
	regionValidator, err := newBeamRegionValidator(cfg.ValidRegions)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ComputeServiceClientProvider == nil {
		cfg.ComputeServiceClientProvider = staticComputeServiceClientProvider{
			client: cfg.ComputeServiceClient,
		}
	}

	return &BeamsService{
		clusterName:             cfg.ClusterName,
		authPreferenceGetter:    cfg.AuthPreferenceGetter,
		beamReader:              cfg.BeamReader,
		storageBackend:          cfg.StorageBackend,
		appWriter:               cfg.AppWriter,
		beamWriter:              cfg.BeamWriter,
		delegationSessionWriter: cfg.DelegationSessionWriter,
		provisionTokenWriter:    cfg.ProvisionTokenWriter,
		roleWriter:              cfg.RoleWriter,
		userWriter:              cfg.UserWriter,
		nodeWriter:              cfg.NodeWriter,
		workloadIdentityWriter:  cfg.WorkloadIdentityWriter,
		computeServiceProvider:  cfg.ComputeServiceClientProvider,
		computeServiceClient:    cfg.ComputeServiceClient,
		authorizer:              cfg.Authorizer,
		usageReporter:           cfg.UsageReporter,
		regionValidator:         regionValidator,
		defaultRegion:           cfg.DefaultRegion,
		generateAlias:           cfg.AliasGenerator,
		logger:                  cfg.Logger,
	}, nil
}

// BeamsServiceConfig contains the configuration options for BeamService.
type BeamsServiceConfig struct {
	// ClusterName is the current cluster's name.
	ClusterName string

	// AuthPreferenceGetter is used to read the cluster's auth preference.
	AuthPreferenceGetter AuthPreferenceGetter

	// BeamReader is used to read beams.
	BeamReader services.BeamReader

	// StorageBackend is used to atomically persist beams and their supporting
	// resources. It must be the same backend backing all local services.
	StorageBackend StorageBackend

	// AppWriter is used to persist beam application resources.
	AppWriter AppWriter

	// BeamWriter is used to persist beam resources.
	BeamWriter services.BeamWriter

	// DelegationSessionWriter is used to persist delegation sessions for beams.
	DelegationSessionWriter DelegationSessionWriter

	// ProvisionTokenWriter is used to persist beam join tokens.
	ProvisionTokenWriter ProvisionTokenWriter

	// UserWriter is used to persist beam bot users.
	UserWriter UserWriter

	// RoleWriter is used to persist beam bot roles.
	RoleWriter RoleWriter

	// NodeWriter is used to persist beam node resources.
	NodeWriter NodeWriter

	// WorkloadIdentityWriter is used to persist beam workload identities.
	WorkloadIdentityWriter WorkloadIdentityWriter

	// ComputeServiceClient is the gRPC client used to communicate with a single
	// beam compute service, responsible for provisioning microVMs.
	ComputeServiceClient compute.BeamsOrchestratorServiceClient

	// ComputeServiceClientProvider returns a beam compute service client for
	// the requested region. If unset, ComputeServiceClient is used for every
	// region.
	ComputeServiceClientProvider ComputeServiceClientProvider

	// Authorizer used to authorize requests.
	Authorizer authz.Authorizer

	// AliasGenerator is called to generate a human-friendly alias when creating
	// a beam.
	AliasGenerator func() (string, error)

	// UsageReporter is used to emit usage events.
	UsageReporter usagereporter.UsageReporter

	// ValidRegions is the list of Beam route regions accepted by this service.
	// If empty, region validation only checks syntax.
	ValidRegions []string

	// DefaultRegion is used when creating a Beam and the client does not
	// request a region. It allows old clients to create region-tagged Beams
	// after regional routing is enabled.
	DefaultRegion string

	// Logger to which errors and messages will be written.
	Logger *slog.Logger
}

type WorkloadIdentityWriter interface {
	AppendPutWorkloadIdentityActions(
		actions []backend.ConditionalAction,
		resource *workloadidentityv1pb.WorkloadIdentity,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)

	AppendDeleteWorkloadIdentityActions(
		actions []backend.ConditionalAction,
		name scopes.QualifiedName,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)
}

type RoleWriter interface {
	AppendPutRoleActions(
		actions []backend.ConditionalAction,
		role types.Role,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)

	AppendDeleteRoleActions(
		actions []backend.ConditionalAction,
		name string,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)
}

type AppWriter interface {
	AppendPutAppActions(
		actions []backend.ConditionalAction,
		app types.Application,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)

	AppendDeleteAppActions(
		actions []backend.ConditionalAction,
		name string,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)
}

type DelegationSessionWriter interface {
	AppendPutDelegationSessionActions(
		actions []backend.ConditionalAction,
		session *delegationv1.DelegationSession,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)

	AppendDeleteDelegationSessionActions(
		actions []backend.ConditionalAction,
		id string,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)
}

type NodeWriter interface {
	AppendPutNodeActions(
		actions []backend.ConditionalAction,
		server types.Server,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)

	AppendDeleteNodeActions(
		actions []backend.ConditionalAction,
		namespace string,
		name string,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)
}

type ProvisionTokenWriter interface {
	AppendPutProvisionTokenActions(
		actions []backend.ConditionalAction,
		token types.ProvisionToken,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)

	AppendDeleteProvisionTokenActions(
		actions []backend.ConditionalAction,
		token string,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)
}

type UserWriter interface {
	AppendPutUserParamsActions(
		actions []backend.ConditionalAction,
		user types.User,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)

	AppendDeleteUserParamsActions(
		actions []backend.ConditionalAction,
		user string,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)
}

type StorageBackend interface {
	AtomicWrite(ctx context.Context, condacts []backend.ConditionalAction) (revision string, err error)
}

// ComputeServiceClientProvider returns a Beam compute service client for a
// requested region. Implementations may return a cached client, create one on
// demand, or ignore the region when all requests should use a single compute
// service. Implementations must return a non-nil client when error is nil.
type ComputeServiceClientProvider interface {
	ClientForRegion(region string) (compute.BeamsOrchestratorServiceClient, error)
}

type staticComputeServiceClientProvider struct {
	client compute.BeamsOrchestratorServiceClient
}

// ClientForRegion returns the configured static compute client, ignoring the
// requested region.
func (p staticComputeServiceClientProvider) ClientForRegion(string) (compute.BeamsOrchestratorServiceClient, error) {
	return p.client, nil
}

// BeamsService implements the RPC methods for managing beams and their
// associated resources.
type BeamsService struct {
	beamsv1.UnimplementedBeamServiceServer

	clusterName          string
	authPreferenceGetter AuthPreferenceGetter
	beamReader           services.BeamReader

	appWriter               AppWriter
	beamWriter              services.BeamWriter
	delegationSessionWriter DelegationSessionWriter
	provisionTokenWriter    ProvisionTokenWriter
	roleWriter              RoleWriter
	userWriter              UserWriter
	nodeWriter              NodeWriter
	workloadIdentityWriter  WorkloadIdentityWriter

	storageBackend         StorageBackend
	computeServiceProvider ComputeServiceClientProvider
	computeServiceClient   compute.BeamsOrchestratorServiceClient
	authorizer             authz.Authorizer
	usageReporter          usagereporter.UsageReporter
	regionValidator        beamRegionValidator
	defaultRegion          string
	generateAlias          func() (string, error)
	logger                 *slog.Logger
}

type AuthPreferenceGetter interface {
	GetReadOnlyAuthPreference(ctx context.Context) (readonly.AuthPreference, error)
}

func (s *BeamsService) checkAccessToBeam(ctx context.Context, authCtx *authz.Context, beam *beamsv1.Beam) error {
	pref, err := s.authPreferenceGetter.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return s.checkAccessToBeamWithAuthPreference(ctx, authCtx, beam, pref)
}

func (s *BeamsService) checkAccessToBeamWithAuthPreference(
	ctx context.Context,
	authCtx *authz.Context,
	beam *beamsv1.Beam,
	pref readonly.AuthPreference,
) error {
	return trace.Wrap(
		authCtx.Checker.CheckAccess(
			types.Resource153ToResourceWithLabels(beam),
			authCtx.GetAccessState(pref),
		),
	)
}
