package workloadclusterv1

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	cloudv1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
)

var (
	// errCloudClientNotReady is returned when the cloud client is not initialized yet
	errCloudClientNotReady = trace.Errorf("unable to communicate with the Teleport Cloud API")
)

// ServiceConfig holds configuration options for the workload cluster gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer used to check access to resources.
	Authorizer authz.Authorizer
	// Emitter is the event emitter.
	Emitter apievents.Emitter
	// Modules contains info about which features are enabled.
	Modules modules.Modules
	// Logger is logger used by service.
	Logger *slog.Logger

	// CloudClientGetter is used to retrieve a client for interacting with Teleport Cloud.
	CloudClientGetter CloudClientGetter
}

// Service implements the gRPC API layer for the WorkloadCluster.
type Service struct {
	workloadcluster.UnimplementedWorkloadClusterServiceServer

	authorizer        authz.Authorizer
	emitter           apievents.Emitter
	cloudClientGetter CloudClientGetter
	modules           modules.Modules
	logger            *slog.Logger
}

// CloudClientGetter is responsible for retrieving a client ready for interacting with Teleport Cloud.
type CloudClientGetter interface {
	GetCloudClient() cloudv1.TenantsServiceClient
}

// NewService returns a new WorkloadCluster API service using the given authorizer.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("Authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("Emitter is required")
	case cfg.CloudClientGetter == nil:
		return nil, trace.BadParameter("CloudClientGetter is required")
	case cfg.Modules == nil:
		return nil, trace.BadParameter("Modules is required")
	case cfg.Logger == nil:
		return nil, trace.BadParameter("Logger is required")
	}
	return &Service{
		authorizer:        cfg.Authorizer,
		emitter:           cfg.Emitter,
		cloudClientGetter: cfg.CloudClientGetter,
		modules:           cfg.Modules,
		logger:            cfg.Logger,
	}, nil
}

func (s *Service) checkWorkloadClustersEnabled() error {
	if s.modules.Features().Entitlements[entitlements.WorkloadClusters].Enabled {
		return nil
	}

	return trace.AccessDenied("only Teleport Cloud users with the WorkloadClusters feature may use workload cluster resources - please reach out to our support team at support@goteleport.com to discuss enabling this feature")
}

// authorizeUser authenticates a local or remote user
func (s *Service) authorizeUser(ctx context.Context) (*authz.Context, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.IsLocalOrRemoteUser(*authCtx) {
		return nil, trace.ConnectionProblem(nil, "workload clusters API is only available to users")
	}

	return authCtx, nil
}

// GetWorkloadCluster gets the requested WorkloadCluster.
//
// GetWorkloadCluster retrieves configuration and status directly from Teleport Cloud.
func (s *Service) GetWorkloadCluster(ctx context.Context, req *workloadcluster.GetWorkloadClusterRequest) (*workloadcluster.WorkloadCluster, error) {
	if err := s.checkWorkloadClustersEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	authCtx, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindWorkloadCluster, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetName() == "" {
		return nil, trace.BadParameter("name is required")
	}

	cloudClient := s.cloudClientGetter.GetCloudClient()
	if cloudClient == nil {
		return nil, errCloudClientNotReady
	}

	cloudReq := cloudv1.GetChildClusterRequest{
		Name: req.GetName(),
	}
	resp, err := cloudClient.GetChildCluster(ctx, &cloudReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	workloadCluster, err := convert(resp.GetCluster())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return workloadCluster, nil
}

// CreateWorkloadCluster creates a WorkloadCluster.
//
// CreateWorkloadCluster instructs Teleport Cloud to provision a new child Teleport Cloud
// cluster.
func (s *Service) CreateWorkloadCluster(ctx context.Context, req *workloadcluster.CreateWorkloadClusterRequest) (workloadCluster *workloadcluster.WorkloadCluster, err error) {
	authCtx, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		code := events.WorkloadClusterCreateCode
		var errMsg string
		if err != nil {
			code = events.WorkloadClusterCreateFailureCode
			errMsg = err.Error()
		}

		payload, marshalErr := apievents.Resource153ToStruct(req.GetCluster())
		if marshalErr != nil {
			s.logger.WarnContext(context.Background(), "Failed to marshal resource for audit event",
				"error", marshalErr,
			)

			return
		}

		userMetadata := authz.ClientUserMetadata(ctx)
		s.emitEvent(ctx, &apievents.WorkloadClusterCreate{
			Metadata: apievents.Metadata{
				Type: events.WorkloadClusterCreateEvent,
				Code: code,
			},
			UserMetadata: userMetadata,
			ResourceMetadata: apievents.ResourceMetadata{
				Name:      req.GetCluster().GetMetadata().GetName(),
				UpdatedBy: userMetadata.User,
			},
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status: apievents.Status{
				Success: err == nil,
				Error:   errMsg,
			},
			Payload: payload,
		})
	}()

	if err := s.checkWorkloadClustersEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindWorkloadCluster, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateWorkloadCluster(req.GetCluster()); err != nil {
		return nil, trace.Wrap(err)
	}

	workloadCluster, err = s.createChildCluster(ctx, req.Cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return workloadCluster, nil
}

// UpdateWorkloadCluster updates the requested WorkloadCluster.
//
// Currently, Teleport Cloud will reject any changes to the configuration.
func (s *Service) UpdateWorkloadCluster(ctx context.Context, req *workloadcluster.UpdateWorkloadClusterRequest) (workloadCluster *workloadcluster.WorkloadCluster, err error) {
	authCtx, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		code := events.WorkloadClusterUpdateCode
		var errMsg string
		if err != nil {
			code = events.WorkloadClusterUpdateFailureCode
			errMsg = err.Error()
		}

		payload, marshalErr := apievents.Resource153ToStruct(req.GetCluster())
		if marshalErr != nil {
			s.logger.WarnContext(context.Background(), "Failed to marshal resource for audit event",
				"error", marshalErr,
			)

			return
		}

		userMetadata := authz.ClientUserMetadata(ctx)
		s.emitEvent(ctx, &apievents.WorkloadClusterUpdate{
			Metadata: apievents.Metadata{
				Type: events.WorkloadClusterUpdateEvent,
				Code: code,
			},
			UserMetadata: userMetadata,
			ResourceMetadata: apievents.ResourceMetadata{
				Name:      req.GetCluster().GetMetadata().GetName(),
				UpdatedBy: userMetadata.User,
			},
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status: apievents.Status{
				Success: err == nil,
				Error:   errMsg,
			},
			Payload: payload,
		})
	}()

	if err := s.checkWorkloadClustersEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindWorkloadCluster, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateWorkloadCluster(req.GetCluster()); err != nil {
		return nil, trace.Wrap(err)
	}

	workloadCluster, err = s.updateChildCluster(ctx, req.Cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return workloadCluster, nil
}

// UpsertWorkloadCluster updates or creates the requested WorkloadCluster.
//
// Teleport Cloud will reject changes to Spec if the child Teleport Cloud cluster already exists.
// Teleport Cloud will return no error if the child cluster already exists and spec matches exactly.
// Teleport Cloud will create the child cluster if it does not exist.
// WorkloadClusters' Spec should be treated as immutable.
func (s *Service) UpsertWorkloadCluster(ctx context.Context, req *workloadcluster.UpsertWorkloadClusterRequest) (workloadCluster *workloadcluster.WorkloadCluster, err error) {
	authCtx, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		code := events.WorkloadClusterUpdateCode
		var errMsg string
		if err != nil {
			code = events.WorkloadClusterUpdateFailureCode
			errMsg = err.Error()
		}

		payload, marshalErr := apievents.Resource153ToStruct(req.GetCluster())
		if marshalErr != nil {
			s.logger.WarnContext(context.Background(), "Failed to marshal resource for audit event",
				"error", marshalErr,
			)

			return
		}

		userMetadata := authz.ClientUserMetadata(ctx)
		s.emitEvent(ctx, &apievents.WorkloadClusterUpdate{
			Metadata: apievents.Metadata{
				Type: events.WorkloadClusterUpdateEvent,
				Code: code,
			},
			UserMetadata: userMetadata,
			ResourceMetadata: apievents.ResourceMetadata{
				Name:      req.GetCluster().GetMetadata().GetName(),
				UpdatedBy: userMetadata.User,
			},
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status: apievents.Status{
				Success: err == nil,
				Error:   errMsg,
			},
			Payload: payload,
		})
	}()

	if err := s.checkWorkloadClustersEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindWorkloadCluster, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateWorkloadCluster(req.GetCluster()); err != nil {
		return nil, trace.Wrap(err)
	}

	workloadCluster, err = s.upsertChildCluster(ctx, req.Cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return workloadCluster, nil
}

// DeleteWorkloadCluster deletes the requested WorkloadCluster.
//
// DeleteWorkloadCluster will instruct Teleport Cloud to suspend the child Teleport Cloud cluster.
func (s *Service) DeleteWorkloadCluster(ctx context.Context, req *workloadcluster.DeleteWorkloadClusterRequest) (resp *emptypb.Empty, err error) {
	authCtx, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		code := events.WorkloadClusterDeleteCode
		var errMsg string
		if err != nil {
			code = events.WorkloadClusterDeleteFailureCode
			errMsg = err.Error()
		}
		userMetadata := authz.ClientUserMetadata(ctx)
		s.emitEvent(ctx, &apievents.WorkloadClusterDelete{
			Metadata: apievents.Metadata{
				Type: events.WorkloadClusterDeleteEvent,
				Code: code,
			},
			UserMetadata: userMetadata,
			ResourceMetadata: apievents.ResourceMetadata{
				Name:      req.GetName(),
				UpdatedBy: userMetadata.User,
			},
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status: apievents.Status{
				Success: err == nil,
				Error:   errMsg,
			},
		})
	}()

	if err := s.checkWorkloadClustersEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindWorkloadCluster, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetName() == "" {
		return nil, trace.BadParameter("name is required")
	}

	cloudClient := s.cloudClientGetter.GetCloudClient()
	if cloudClient == nil {
		return nil, errCloudClientNotReady
	}

	cloudReq := cloudv1.SuspendChildClusterRequest{
		Name: req.Name,
	}
	if _, err := cloudClient.SuspendChildCluster(ctx, &cloudReq); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Service) emitEvent(ctx context.Context, e apievents.AuditEvent) {
	if err := s.emitter.EmitAuditEvent(ctx, e); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit audit event",
			"type", e.GetType(),
			"error", err,
		)
	}
}

// ListWorkloadClusters returns a list of workload clusters.
func (s *Service) ListWorkloadClusters(ctx context.Context, req *workloadcluster.ListWorkloadClustersRequest) (*workloadcluster.ListWorkloadClustersResponse, error) {
	authCtx, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.checkWorkloadClustersEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindWorkloadCluster, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	cloudClient := s.cloudClientGetter.GetCloudClient()
	if cloudClient == nil {
		return nil, errCloudClientNotReady
	}

	resp, err := cloudClient.ListChildClusters(ctx, &cloudv1.ListChildClustersRequest{
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	workloadClusters := make([]*workloadcluster.WorkloadCluster, 0, len(resp.GetClusters()))
	for _, c := range resp.GetClusters() {
		wc, err := convert(c)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		workloadClusters = append(workloadClusters, wc)
	}

	return &workloadcluster.ListWorkloadClustersResponse{
		Clusters:      workloadClusters,
		NextPageToken: resp.GetNextPageToken(),
	}, nil
}

// createChildCluster instructs Teleport Cloud to create a new child Teleport Cloud cluster.
func (s *Service) createChildCluster(ctx context.Context, wc *workloadcluster.WorkloadCluster) (*workloadcluster.WorkloadCluster, error) {
	cloudClient := s.cloudClientGetter.GetCloudClient()
	if cloudClient == nil {
		return nil, errCloudClientNotReady
	}

	// Note: Teleport Cloud API will validate provided configuration for
	// regions, bot, and token.
	// This enables Teleport Cloud supporting new regions without having
	// to update Teleport. So keep this service's validation
	// simple and rely on Teleport Cloud API for validation.
	cloudReq := cloudv1.CreateChildClusterRequest{
		Name:       wc.GetMetadata().GetName(),
		Regions:    convertRegions(wc.GetSpec().GetRegions()),
		BotName:    wc.GetSpec().GetBot().GetName(),
		JoinMethod: wc.GetSpec().GetToken().GetJoinMethod(),
		Allow:      convertAllowRules(wc.GetSpec().GetToken().GetAllow()),
	}

	resp, err := cloudClient.CreateChildCluster(ctx, &cloudReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	workloadCluster, err := convert(resp.GetCluster())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return workloadCluster, nil
}

// validateWorkloadCluster will validate a workload cluster before it's sent to Teleport Cloud for
// full validation.
func validateWorkloadCluster(wc *workloadcluster.WorkloadCluster) error {
	if wc.GetMetadata().GetName() == "" {
		return trace.BadParameter("name is required")
	}

	if len(wc.GetSpec().GetRegions()) == 0 {
		return trace.BadParameter("regions is required")
	}

	for _, r := range wc.GetSpec().GetRegions() {
		if r.GetName() == "" {
			return trace.BadParameter("region name is required")
		}
	}

	// Note: all other fields are considered optional due to the fact that
	// Teleport Cloud allows customers to adopt existing tenants as child clusters,
	// which may have been created without configuring a bot or join token.

	return nil
}

func convertAllowRules(in []*workloadcluster.Allow) []*cloudv1.Allow {
	out := make([]*cloudv1.Allow, 0, len(in))

	for _, a := range in {
		out = append(out, &cloudv1.Allow{
			AwsAccount: a.GetAwsAccount(),
			AwsArn:     a.GetAwsArn(),
		})
	}

	return out
}

func convertRegions(in []*workloadcluster.Region) []*cloudv1.Region {
	out := make([]*cloudv1.Region, 0, len(in))

	for _, r := range in {
		out = append(out, &cloudv1.Region{
			Name: r.GetName(),
		})
	}

	return out
}

// updateChildCluster instructs Teleport Cloud to update a child Teleport Cloud cluster.
func (s *Service) updateChildCluster(ctx context.Context, wc *workloadcluster.WorkloadCluster) (*workloadcluster.WorkloadCluster, error) {
	cloudClient := s.cloudClientGetter.GetCloudClient()
	if cloudClient == nil {
		return nil, errCloudClientNotReady
	}

	// Note: Teleport Cloud API will currently validate provided configuration matches
	// exactly to the existing child cluster.
	cloudReq := cloudv1.UpdateChildClusterRequest{
		Name:       wc.GetMetadata().GetName(),
		Revision:   wc.GetMetadata().GetRevision(),
		Regions:    convertRegions(wc.GetSpec().GetRegions()),
		BotName:    wc.GetSpec().GetBot().GetName(),
		JoinMethod: wc.GetSpec().GetToken().GetJoinMethod(),
		Allow:      convertAllowRules(wc.GetSpec().GetToken().GetAllow()),
	}

	resp, err := cloudClient.UpdateChildCluster(ctx, &cloudReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	workloadCluster, err := convert(resp.GetCluster())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return workloadCluster, nil
}

// upsertsChildCluster instructs Teleport Cloud to upsert a child Teleport Cloud cluster.
func (s *Service) upsertChildCluster(ctx context.Context, wc *workloadcluster.WorkloadCluster) (*workloadcluster.WorkloadCluster, error) {
	cloudClient := s.cloudClientGetter.GetCloudClient()
	if cloudClient == nil {
		return nil, errCloudClientNotReady
	}

	// Note: Teleport Cloud API will currently validate provided configuration matches
	// exactly to the existing child cluster.
	cloudReq := cloudv1.UpsertChildClusterRequest{
		Name:       wc.GetMetadata().GetName(),
		Regions:    convertRegions(wc.GetSpec().GetRegions()),
		BotName:    wc.GetSpec().GetBot().GetName(),
		JoinMethod: wc.GetSpec().GetToken().GetJoinMethod(),
		Allow:      convertAllowRules(wc.GetSpec().GetToken().GetAllow()),
	}

	resp, err := cloudClient.UpsertChildCluster(ctx, &cloudReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	workloadCluster, err := convert(resp.GetCluster())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return workloadCluster, nil
}

// convert transforms a [cloudv1.ChildCluster] to a [workloadcluster.WorkloadCluster].
//
// Typically used to convert responses from Teleport Cloud to a workload_cluster for
// Teleport users.
func convert(in *cloudv1.ChildCluster) (*workloadcluster.WorkloadCluster, error) {
	if in.GetSpec() == nil || in.GetStatus() == nil {
		// this should never happen, but return an actionable error message to the user
		return nil, errors.New("received invalid response from Teleport Cloud - please reach out to our support team at support@goteleport.com")
	}

	out := &workloadcluster.WorkloadCluster{
		Version: types.V1,
		Kind:    types.KindWorkloadCluster,
		Metadata: &headerv1.Metadata{
			Name:     in.GetName(),
			Revision: in.GetRevision(),
		},
		Spec: &workloadcluster.WorkloadClusterSpec{},
		Status: &workloadcluster.WorkloadClusterStatus{
			Domain: in.GetStatus().GetDomain(),
			State:  in.GetStatus().GetState(),
		},
	}

	if in.GetSpec().GetBotName() != "" {
		out.Spec.Bot = &workloadcluster.Bot{
			Name: in.GetSpec().GetBotName(),
		}
	}

	if in.GetSpec().GetJoinMethod() != "" {
		rules := make([]*workloadcluster.Allow, 0, len(in.GetSpec().GetAllow()))
		for _, a := range in.GetSpec().GetAllow() {
			rules = append(rules, &workloadcluster.Allow{
				AwsAccount: a.GetAwsAccount(),
				AwsArn:     a.GetAwsArn(),
			})
		}

		out.Spec.Token = &workloadcluster.Token{
			JoinMethod: in.GetSpec().GetJoinMethod(),
			Allow:      rules,
		}
	}

	out.Spec.Regions = make([]*workloadcluster.Region, 0, len(in.GetSpec().GetRegions()))
	for _, r := range in.GetSpec().GetRegions() {
		out.Spec.Regions = append(out.Spec.Regions, &workloadcluster.Region{
			Name: r.GetName(),
		})
	}

	return out, nil
}
