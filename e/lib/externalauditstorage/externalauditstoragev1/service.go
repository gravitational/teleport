package externalauditstoragev1

import (
	"context"
	"log/slog"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalauditstorage/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	eastypes "github.com/gravitational/teleport/api/types/externalauditstorage"
	conv "github.com/gravitational/teleport/api/types/externalauditstorage/convert/v1"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/e/lib/externalauditstorage"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/credprovider"
	ecaint "github.com/gravitational/teleport/lib/integrations/externalauditstorage"
	"github.com/gravitational/teleport/lib/services/local"
)

// ClusterAuditConfigGetter is an interface for getting the current cluster
// audit config.
type ClusterAuditConfigGetter interface {
	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error)
}

// ServiceConfig holds configuration options for the external audit gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer
	// ExternalAuditStorage is the External Audit Storage service to use.
	ExternalAuditStorage *local.ExternalAuditStorageService
	// ClusterAuditConfig holds cluster audit configuration
	ClusterAuditConfigGetter ClusterAuditConfigGetter
	// IntegrationSvc is required to create a configurator on demand for draft testing
	IntegrationSvc *local.IntegrationsService
	// OIDCTokenFn is method used to retrieve OIDC tokens for use in OIDC AWS Config
	OIDCTokenFn credprovider.GenerateOIDCTokenFn
	Emitter     apievents.Emitter
}

// Service implements the external audit gRPC service.
type Service struct {
	pb.UnimplementedExternalAuditStorageServiceServer

	logger                   *slog.Logger
	authorizer               authz.Authorizer
	externalAuditStorage     *local.ExternalAuditStorageService
	clusterAuditConfigGetter ClusterAuditConfigGetter
	integrationSvc           *local.IntegrationsService
	oidcTokenFn              credprovider.GenerateOIDCTokenFn
	emitter                  apievents.Emitter
}

// NewService returns a new external audit gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.ExternalAuditStorage == nil:
		return nil, trace.BadParameter("ExternalAuditStorage Service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("Authorizer is required")
	case cfg.ClusterAuditConfigGetter == nil:
		return nil, trace.BadParameter("ClusterAuditConfigGetter is required")
	case cfg.IntegrationSvc == nil:
		return nil, trace.BadParameter("IntegrationSvc is required")
	case cfg.OIDCTokenFn == nil:
		return nil, trace.BadParameter("OIDCTokenFn is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("Emitter is required")
	}
	return &Service{
		logger:                   slog.With(teleport.ComponentKey, teleport.Component("EAS", "service")),
		authorizer:               cfg.Authorizer,
		externalAuditStorage:     cfg.ExternalAuditStorage,
		clusterAuditConfigGetter: cfg.ClusterAuditConfigGetter,
		integrationSvc:           cfg.IntegrationSvc,
		oidcTokenFn:              cfg.OIDCTokenFn,
		emitter:                  cfg.Emitter,
	}, nil
}

func (s *Service) TestDraftExternalAuditStorageBuckets(ctx context.Context, req *pb.TestDraftExternalAuditStorageBucketsRequest) (*pb.TestDraftExternalAuditStorageBucketsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindExternalAuditStorage, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	draft, err := s.externalAuditStorage.GetDraftExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting draft External Audit Storage configuration")
	}

	cfg, err := s.getAWSConfig(ctx, authCtx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting aws config")
	}

	if err := externalauditstorage.ConnectionTestBuckets(ctx, s3.NewFromConfig(cfg), &draft.Spec); err != nil {
		return nil, trace.Wrap(err, "connection test for draft buckets failed")
	}

	return &pb.TestDraftExternalAuditStorageBucketsResponse{}, nil
}

func (s *Service) TestDraftExternalAuditStorageGlue(ctx context.Context, req *pb.TestDraftExternalAuditStorageGlueRequest) (*pb.TestDraftExternalAuditStorageGlueResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindExternalAuditStorage, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	draft, err := s.externalAuditStorage.GetDraftExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting draft External Audit Storage configuration")
	}

	cfg, err := s.getAWSConfig(ctx, authCtx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting aws config")
	}

	if err := externalauditstorage.ConnectionTestGlue(ctx, glue.NewFromConfig(cfg), &draft.Spec); err != nil {
		return nil, trace.Wrap(err, "connection test for draft glue table failed")
	}

	return &pb.TestDraftExternalAuditStorageGlueResponse{}, nil
}

func (s *Service) TestDraftExternalAuditStorageAthena(ctx context.Context, req *pb.TestDraftExternalAuditStorageAthenaRequest) (*pb.TestDraftExternalAuditStorageAthenaResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindExternalAuditStorage, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	draft, err := s.externalAuditStorage.GetDraftExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting draft External Audit Storage configuration")
	}

	cfg, err := s.getAWSConfig(ctx, authCtx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting aws config")
	}

	if err := externalauditstorage.ConnectionTestAthena(ctx, athena.NewFromConfig(cfg), &draft.Spec); err != nil {
		return nil, trace.Wrap(err, "connection test for draft athena query failed")
	}

	return &pb.TestDraftExternalAuditStorageAthenaResponse{}, nil
}

func (s *Service) GenerateDraftExternalAuditStorage(ctx context.Context, req *pb.GenerateDraftExternalAuditStorageRequest) (*pb.GenerateDraftExternalAuditStorageResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindExternalAuditStorage, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.checkClusterAuditConfig(ctx, req.Region); err != nil {
		return nil, trace.Wrap(err, "unable to configure External Audit Storage")
	}

	resp, err := s.externalAuditStorage.GenerateDraftExternalAuditStorage(ctx, req.IntegrationName, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.GenerateDraftExternalAuditStorageResponse{
		ExternalAuditStorage: conv.ToProto(resp),
	}, nil
}

func (s *Service) CreateDraftExternalAuditStorage(ctx context.Context, req *pb.CreateDraftExternalAuditStorageRequest) (*pb.CreateDraftExternalAuditStorageResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindExternalAuditStorage, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Validation of parameters is done in FromProtoDraft.
	draft, err := conv.FromProtoDraft(req.GetExternalAuditStorage())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.checkClusterAuditConfig(ctx, draft.Spec.Region); err != nil {
		return nil, trace.Wrap(err, "unable to configure External Audit Storage")
	}

	resp, err := s.externalAuditStorage.CreateDraftExternalAuditStorage(ctx, draft)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.CreateDraftExternalAuditStorageResponse{
		ExternalAuditStorage: conv.ToProto(resp),
	}, nil
}

func (s *Service) UpsertDraftExternalAuditStorage(ctx context.Context, req *pb.UpsertDraftExternalAuditStorageRequest) (*pb.UpsertDraftExternalAuditStorageResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindExternalAuditStorage, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Validation of parameters is done in FromProtoDraft.
	draft, err := conv.FromProtoDraft(req.GetExternalAuditStorage())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.checkClusterAuditConfig(ctx, draft.Spec.Region); err != nil {
		return nil, trace.Wrap(err, "unable to configure External Audit Storage")
	}
	resp, err := s.externalAuditStorage.UpsertDraftExternalAuditStorage(ctx, draft)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.UpsertDraftExternalAuditStorageResponse{
		ExternalAuditStorage: conv.ToProto(resp),
	}, nil
}

func (s *Service) GetDraftExternalAuditStorage(ctx context.Context, req *pb.GetDraftExternalAuditStorageRequest) (*pb.GetDraftExternalAuditStorageResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindExternalAuditStorage, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	externalAudit, err := s.externalAuditStorage.GetDraftExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.GetDraftExternalAuditStorageResponse{
		ExternalAuditStorage: conv.ToProto(externalAudit),
	}, nil
}

func (s *Service) DeleteDraftExternalAuditStorage(ctx context.Context, req *pb.DeleteDraftExternalAuditStorageRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindExternalAuditStorage, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.externalAuditStorage.DeleteDraftExternalAuditStorage(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) PromoteToClusterExternalAuditStorage(ctx context.Context, req *pb.PromoteToClusterExternalAuditStorageRequest) (*pb.PromoteToClusterExternalAuditStorageResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindExternalAuditStorage, types.VerbCreate, types.VerbUpdate, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(nklaassen): administrative endpoint with mfa.

	draft, err := s.externalAuditStorage.GetDraftExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to retrieve current draft configuration")
	}

	if err := s.checkClusterAuditConfig(ctx, draft.Spec.Region); err != nil {
		return nil, trace.Wrap(err, "unable to configure External Audit Storage")
	}

	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.ExternalAuditStorageEnable{
		Metadata: apievents.Metadata{
			Type: events.ExternalAuditStorageEnableEvent,
			Code: events.ExternalAuditStorageEnableCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      "cluster",
			UpdatedBy: userMetadata.User,
		},
		Details: eventDetails(draft),
	})

	if err := s.externalAuditStorage.PromoteToClusterExternalAuditStorage(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.PromoteToClusterExternalAuditStorageResponse{}, nil
}

func (s *Service) GetClusterExternalAuditStorage(ctx context.Context, req *pb.GetClusterExternalAuditStorageRequest) (*pb.GetClusterExternalAuditStorageResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindExternalAuditStorage, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	externalAudit, err := s.externalAuditStorage.GetClusterExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.GetClusterExternalAuditStorageResponse{
		ClusterExternalAuditStorage: conv.ToProto(externalAudit),
	}, nil
}

func (s *Service) DisableClusterExternalAuditStorage(ctx context.Context, req *pb.DisableClusterExternalAuditStorageRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindExternalAuditStorage, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(nklaassen): administrative endpoint with mfa.

	cluster, err := s.externalAuditStorage.GetClusterExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to retrieve current cluster configuration")
	}

	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.ExternalAuditStorageDisable{
		Metadata: apievents.Metadata{
			Type: events.ExternalAuditStorageDisableEvent,
			Code: events.ExternalAuditStorageDisableCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      "cluster",
			UpdatedBy: userMetadata.User,
		},
		Details: eventDetails(cluster),
	})

	if err := s.externalAuditStorage.DisableClusterExternalAuditStorage(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

var externalAuditMissingAthenaError = &trace.BadParameterError{Message: "no athena audit_events_uri is configured in cluster audit config"}

func (s *Service) checkClusterAuditConfig(ctx context.Context, region string) error {
	auditConf, err := s.clusterAuditConfigGetter.GetClusterAuditConfig(ctx)
	if err != nil {
		return trace.Wrap(err, "getting cluster audit config")
	}
	if !slices.ContainsFunc(auditConf.AuditEventsURIs(), func(s string) bool {
		uri, err := utils.ParseSessionsURI(s)
		if err != nil {
			return false
		}
		return uri.Scheme == teleport.ComponentAthena
	}) {
		return trace.Wrap(externalAuditMissingAthenaError)
	}
	if auditConf.Region() != region {
		return trace.BadParameter("region %q rejected: External Audit Storage must be configured in %q", region, auditConf.Region())
	}
	return nil
}

// retrieve an AWS Config using the configurator credentials provider
func (s *Service) getAWSConfig(ctx context.Context, authCtx *authz.Context) (aws.Config, error) {
	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbRead); err != nil {
		return aws.Config{}, trace.Wrap(err)
	}
	configurator, err := ecaint.NewDraftConfigurator(ctx, s.externalAuditStorage, s.integrationSvc)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}
	configurator.SetGenerateOIDCTokenFn(s.oidcTokenFn)
	configurator.WaitForFirstCredentials(ctx)

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(configurator.GetSpec().Region), config.WithCredentialsProvider(configurator.CredentialsProvider()))
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	return cfg, nil
}

func (s *Service) emitEvent(ctx context.Context, e apievents.AuditEvent) {
	if err := s.emitter.EmitAuditEvent(context.Background(), e); err != nil {
		s.logger.InfoContext(ctx, "Failed to emit audit event", "type", e.GetType(), "error", err)
	}
}

func eventDetails(r *eastypes.ExternalAuditStorage) *apievents.ExternalAuditStorageDetails {
	return &apievents.ExternalAuditStorageDetails{
		IntegrationName:        r.Spec.IntegrationName,
		SessionRecordingsUri:   r.Spec.SessionRecordingsURI,
		AthenaWorkgroup:        r.Spec.AthenaWorkgroup,
		GlueDatabase:           r.Spec.GlueDatabase,
		GlueTable:              r.Spec.GlueTable,
		AuditEventsLongTermUri: r.Spec.AuditEventsLongTermURI,
		AthenaResultsUri:       r.Spec.AthenaResultsURI,
		PolicyName:             r.Spec.PolicyName,
		Region:                 r.Spec.Region,
	}
}
