package externalauditstoragev1

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalauditstorage/v1"
	"github.com/gravitational/teleport/api/types"
	conv "github.com/gravitational/teleport/api/types/externalauditstorage/convert/v1"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/e/lib/externalauditstorage"
	"github.com/gravitational/teleport/lib/authz"
	ecaint "github.com/gravitational/teleport/lib/integrations/externalauditstorage"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

// ClusterAuditConfigGetter is an interface for getting the current cluster
// audit config.
type ClusterAuditConfigGetter interface {
	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)
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
	OIDCTokenFn ecaint.GenerateOIDCTokenFn
}

// Service implements the external audit gRPC service.
type Service struct {
	pb.UnimplementedExternalAuditStorageServiceServer

	logger *logrus.Entry

	authorizer               authz.Authorizer
	externalAuditStorage     *local.ExternalAuditStorageService
	clusterAuditConfigGetter ClusterAuditConfigGetter
	integrationSvc           *local.IntegrationsService
	oidcTokenFn              ecaint.GenerateOIDCTokenFn
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
	}
	return &Service{
		logger:                   logrus.WithField(trace.Component, "ExternalAuditStorage.service"),
		authorizer:               cfg.Authorizer,
		externalAuditStorage:     cfg.ExternalAuditStorage,
		clusterAuditConfigGetter: cfg.ClusterAuditConfigGetter,
		integrationSvc:           cfg.IntegrationSvc,
		oidcTokenFn:              cfg.OIDCTokenFn,
	}, nil
}

func (s *Service) TestDraftExternalAuditStorageBuckets(ctx context.Context, req *pb.TestDraftExternalAuditStorageBucketsRequest) (*pb.TestDraftExternalAuditStorageBucketsResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	draft, err := s.externalAuditStorage.GetDraftExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting draft External Audit Storage configuration")
	}

	cfg, err := s.getAWSConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting aws config")
	}

	if err := externalauditstorage.ConnectionTestBuckets(ctx, s3.NewFromConfig(cfg), &draft.Spec); err != nil {
		return nil, trace.Wrap(err, "connection test for draft buckets failed")
	}

	return &pb.TestDraftExternalAuditStorageBucketsResponse{}, nil
}

func (s *Service) TestDraftExternalAuditStorageGlue(ctx context.Context, req *pb.TestDraftExternalAuditStorageGlueRequest) (*pb.TestDraftExternalAuditStorageGlueResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	draft, err := s.externalAuditStorage.GetDraftExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting draft External Audit Storage configuration")
	}

	cfg, err := s.getAWSConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting aws config")
	}

	if err := externalauditstorage.ConnectionTestGlue(ctx, glue.NewFromConfig(cfg), &draft.Spec); err != nil {
		return nil, trace.Wrap(err, "connection test for draft glue table failed")
	}

	return &pb.TestDraftExternalAuditStorageGlueResponse{}, nil
}

func (s *Service) TestDraftExternalAuditStorageAthena(ctx context.Context, req *pb.TestDraftExternalAuditStorageAthenaRequest) (*pb.TestDraftExternalAuditStorageAthenaResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	draft, err := s.externalAuditStorage.GetDraftExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting draft External Audit Storage configuration")
	}

	cfg, err := s.getAWSConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting aws config")
	}

	if err := externalauditstorage.ConnectionTestAthena(ctx, athena.NewFromConfig(cfg), &draft.Spec); err != nil {
		return nil, trace.Wrap(err, "connection test for draft athena query failed")
	}

	return &pb.TestDraftExternalAuditStorageAthenaResponse{}, nil
}

func (s *Service) GenerateDraftExternalAuditStorage(ctx context.Context, req *pb.GenerateDraftExternalAuditStorageRequest) (*pb.GenerateDraftExternalAuditStorageResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.checkClusterAuditConfig(ctx); err != nil {
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

func (s *Service) UpsertDraftExternalAuditStorage(ctx context.Context, req *pb.UpsertDraftExternalAuditStorageRequest) (*pb.UpsertDraftExternalAuditStorageResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.checkClusterAuditConfig(ctx); err != nil {
		return nil, trace.Wrap(err, "unable to configure External Audit Storage")
	}

	// Validation of parameters is done in FromProtoDraft.
	draft, err := conv.FromProtoDraft(req.GetExternalAuditStorage())
	if err != nil {
		return nil, trace.Wrap(err)
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
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
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
	if err := s.authorizeVerbs(ctx, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.externalAuditStorage.DeleteDraftExternalAuditStorage(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) PromoteToClusterExternalAuditStorage(ctx context.Context, req *pb.PromoteToClusterExternalAuditStorageRequest) (*pb.PromoteToClusterExternalAuditStorageResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(nklaassen): administrative endpoint with mfa.

	if err := s.checkClusterAuditConfig(ctx); err != nil {
		return nil, trace.Wrap(err, "unable to configure External Audit Storage")
	}

	// TODO(nklaassen): emit audit event.

	if err := s.externalAuditStorage.PromoteToClusterExternalAuditStorage(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.PromoteToClusterExternalAuditStorageResponse{}, nil
}

func (s *Service) GetClusterExternalAuditStorage(ctx context.Context, req *pb.GetClusterExternalAuditStorageRequest) (*pb.GetClusterExternalAuditStorageResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
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
	if err := s.authorizeVerbs(ctx, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(nklaassen): administrative endpoint with mfa.
	// TODO(nklaassen): emit audit event.

	if err := s.externalAuditStorage.DisableClusterExternalAuditStorage(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Service) authorizeVerbs(ctx context.Context, verbs ...string) error {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, false /*quiet*/, types.KindExternalAuditStorage, verbs...)
	return trace.Wrap(err)
}

var externalAuditMissingAthenaError = &trace.BadParameterError{Message: "no athena audit_events_uri is configured in cluster audit config"}

func (s *Service) checkClusterAuditConfig(ctx context.Context) error {
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
	return nil
}

// retrieve an AWS Config using the configurator credentials provider
func (s *Service) getAWSConfig(ctx context.Context) (aws.Config, error) {
	if _, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, false /*quiet*/, types.KindIntegration, types.VerbRead); err != nil {
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
