package externalcloudauditv1

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
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalcloudaudit/v1"
	"github.com/gravitational/teleport/api/types"
	conv "github.com/gravitational/teleport/api/types/externalcloudaudit/convert/v1"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/e/lib/externalcloudaudit"
	"github.com/gravitational/teleport/lib/authz"
	ecaint "github.com/gravitational/teleport/lib/integrations/externalcloudaudit"
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
	// ExternalCloudAudit is the external cloud audit service to use.
	ExternalCloudAudit services.ExternalCloudAudits
	// ClusterAuditConfig holds cluster audit configuration
	ClusterAuditConfigGetter ClusterAuditConfigGetter
	// IntegrationSvc is required to create a configurator on demand for draft testing
	IntegrationSvc *local.IntegrationsService
	// OIDCTokenFn is method used to retrieve OIDC tokens for use in OIDC AWS Config
	OIDCTokenFn ecaint.GenerateOIDCTokenFn
}

// Service implements the external audit gRPC service.
type Service struct {
	pb.UnimplementedExternalCloudAuditServiceServer

	logger *logrus.Entry

	authorizer               authz.Authorizer
	externalCloudAudit       services.ExternalCloudAudits
	clusterAuditConfigGetter ClusterAuditConfigGetter
	integrationSvc           *local.IntegrationsService
	oidcTokenFn              ecaint.GenerateOIDCTokenFn
}

// NewService returns a new external audit gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.ExternalCloudAudit == nil:
		return nil, trace.BadParameter("ExternalCloudAudit Service is required")
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
		logger:                   logrus.WithField(trace.Component, "externalcloudaudit.service"),
		authorizer:               cfg.Authorizer,
		externalCloudAudit:       cfg.ExternalCloudAudit,
		clusterAuditConfigGetter: cfg.ClusterAuditConfigGetter,
		integrationSvc:           cfg.IntegrationSvc,
	}, nil
}

func (s *Service) TestDraftExternalCloudAuditBuckets(ctx context.Context, req *pb.TestDraftExternalCloudAuditBucketsRequest) (*pb.TestDraftExternalCloudAuditBucketsResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	draft, err := s.externalCloudAudit.GetDraftExternalCloudAudit(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting draft external cloud audit")
	}

	cfg, err := s.getAWSConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting aws config")
	}

	if err := externalcloudaudit.ConnectionTestBuckets(ctx, s3.NewFromConfig(cfg), &draft.Spec); err != nil {
		return nil, trace.Wrap(err, "connection test for draft buckets failed")
	}

	return &pb.TestDraftExternalCloudAuditBucketsResponse{}, nil
}

func (s *Service) TestDraftExternalCloudAuditGlue(ctx context.Context, req *pb.TestDraftExternalCloudAuditGlueRequest) (*pb.TestDraftExternalCloudAuditGlueResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	draft, err := s.externalCloudAudit.GetDraftExternalCloudAudit(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting draft external cloud audit")
	}

	cfg, err := s.getAWSConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting aws config")
	}

	if err := externalcloudaudit.ConnectionTestGlue(ctx, glue.NewFromConfig(cfg), &draft.Spec); err != nil {
		return nil, trace.Wrap(err, "connection test for draft glue table failed")
	}

	return &pb.TestDraftExternalCloudAuditGlueResponse{}, nil
}

func (s *Service) TestDraftExternalCloudAuditAthena(ctx context.Context, req *pb.TestDraftExternalCloudAuditAthenaRequest) (*pb.TestDraftExternalCloudAuditAthenaResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	draft, err := s.externalCloudAudit.GetDraftExternalCloudAudit(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting draft external cloud audit")
	}

	cfg, err := s.getAWSConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "error getting aws config")
	}

	if err := externalcloudaudit.ConnectionTestAthena(ctx, athena.NewFromConfig(cfg), &draft.Spec); err != nil {
		return nil, trace.Wrap(err, "connection test for draft athena query failed")
	}

	return &pb.TestDraftExternalCloudAuditAthenaResponse{}, nil
}

func (s *Service) GenerateDraftExternalCloudAudit(ctx context.Context, req *pb.GenerateDraftExternalCloudAuditRequest) (*pb.GenerateDraftExternalCloudAuditResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.checkClusterAuditConfig(ctx); err != nil {
		return nil, trace.Wrap(err, "unable to configure External Audit Storage")
	}

	resp, err := s.externalCloudAudit.GenerateDraftExternalCloudAudit(ctx, req.IntegrationName, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.GenerateDraftExternalCloudAuditResponse{
		ExternalCloudAudit: conv.ToProto(resp),
	}, nil
}

func (s *Service) UpsertDraftExternalCloudAudit(ctx context.Context, req *pb.UpsertDraftExternalCloudAuditRequest) (*pb.UpsertDraftExternalCloudAuditResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.checkClusterAuditConfig(ctx); err != nil {
		return nil, trace.Wrap(err, "unable to configure External Audit Storage")
	}

	// Validation of parameters is done in FromProtoDraft.
	draft, err := conv.FromProtoDraft(req.GetExternalCloudAudit())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := s.externalCloudAudit.UpsertDraftExternalCloudAudit(ctx, draft)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.UpsertDraftExternalCloudAuditResponse{
		ExternalCloudAudit: conv.ToProto(resp),
	}, nil
}

func (s *Service) GetDraftExternalCloudAudit(ctx context.Context, req *pb.GetDraftExternalCloudAuditRequest) (*pb.GetDraftExternalCloudAuditResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	externalAudit, err := s.externalCloudAudit.GetDraftExternalCloudAudit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.GetDraftExternalCloudAuditResponse{
		ExternalCloudAudit: conv.ToProto(externalAudit),
	}, nil
}

func (s *Service) DeleteDraftExternalCloudAudit(ctx context.Context, req *pb.DeleteDraftExternalCloudAuditRequest) (*emptypb.Empty, error) {
	if err := s.authorizeVerbs(ctx, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.externalCloudAudit.DeleteDraftExternalCloudAudit(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) PromoteToClusterExternalCloudAudit(ctx context.Context, req *pb.PromoteToClusterExternalCloudAuditRequest) (*pb.PromoteToClusterExternalCloudAuditResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(nklaassen): administrative endpoint with mfa.

	if err := s.checkClusterAuditConfig(ctx); err != nil {
		return nil, trace.Wrap(err, "unable to configure External Audit Storage")
	}

	// TODO(nklaassen): emit audit event.

	if err := s.externalCloudAudit.PromoteToClusterExternalCloudAudit(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.PromoteToClusterExternalCloudAuditResponse{}, nil
}

func (s *Service) GetClusterExternalCloudAudit(ctx context.Context, req *pb.GetClusterExternalCloudAuditRequest) (*pb.GetClusterExternalCloudAuditResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	externalAudit, err := s.externalCloudAudit.GetClusterExternalCloudAudit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.GetClusterExternalCloudAuditResponse{
		ClusterExternalCloudAudit: conv.ToProto(externalAudit),
	}, nil
}

func (s *Service) DisableClusterExternalCloudAudit(ctx context.Context, req *pb.DisableClusterExternalCloudAuditRequest) (*emptypb.Empty, error) {
	if err := s.authorizeVerbs(ctx, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(nklaassen): administrative endpoint with mfa.
	// TODO(nklaassen): emit audit event.

	if err := s.externalCloudAudit.DisableClusterExternalCloudAudit(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Service) authorizeVerbs(ctx context.Context, verbs ...string) error {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, false /*quiet*/, types.KindExternalCloudAudit, verbs...)
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
	configurator, err := ecaint.NewDraftConfigurator(ctx, s.externalCloudAudit, s.integrationSvc)
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
