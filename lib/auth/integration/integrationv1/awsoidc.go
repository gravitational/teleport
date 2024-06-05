/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package integrationv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// GenerateAWSOIDCToken generates a token to be used when executing an AWS OIDC Integration action.
func (s *Service) GenerateAWSOIDCToken(ctx context.Context, req *integrationpb.GenerateAWSOIDCTokenRequest) (*integrationpb.GenerateAWSOIDCTokenResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, allowedRole := range []types.SystemRole{types.RoleDiscovery, types.RoleAuth, types.RoleProxy} {
		if authz.HasBuiltinRole(*authCtx, string(allowedRole)) {
			return s.generateAWSOIDCTokenWithoutAuthZ(ctx, req.Integration)
		}
	}

	return nil, trace.AccessDenied("token generation is only available to auth, proxy or discovery services")
}

// generateAWSOIDCTokenWithoutAuthZ generates a token to be used when executing an AWS OIDC Integration action.
// Bypasses authz and should only be used by other methods that validate AuthZ.
func (s *Service) generateAWSOIDCTokenWithoutAuthZ(ctx context.Context, integrationName string) (*integrationpb.GenerateAWSOIDCTokenResponse, error) {
	username, err := authz.GetClientUsername(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := awsoidc.GenerateAWSOIDCToken(ctx, s.cache, s.keyStoreManager, awsoidc.GenerateAWSOIDCTokenRequest{
		Integration: integrationName,
		Username:    username,
		Subject:     types.IntegrationAWSOIDCSubject,
		Clock:       s.clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &integrationpb.GenerateAWSOIDCTokenResponse{
		Token: token,
	}, nil
}

// AWSOIDCServiceConfig holds configuration options for the AWSOIDC Integration gRPC service.
type AWSOIDCServiceConfig struct {
	IntegrationService    *Service
	Authorizer            authz.Authorizer
	Cache                 CacheAWSOIDC
	Clock                 clockwork.Clock
	ProxyPublicAddrGetter func() string
	Logger                *slog.Logger
}

// CheckAndSetDefaults checks the AWSOIDCServiceConfig fields and returns an error if a required param is not provided.
// Authorizer and IntegrationService are required params.
func (s *AWSOIDCServiceConfig) CheckAndSetDefaults() error {
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}

	if s.IntegrationService == nil {
		return trace.BadParameter("integration service is required")
	}

	if s.Cache == nil {
		return trace.BadParameter("cache is required")
	}

	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}

	if s.ProxyPublicAddrGetter == nil {
		return trace.BadParameter("proxyPublicAddrGetter is required")
	}

	if s.Logger == nil {
		s.Logger = slog.With(teleport.ComponentKey, "integrations.awsoidc.service")
	}

	return nil
}

// AWSOIDCService implements the teleport.integration.v1.AWSOIDCService RPC service.
type AWSOIDCService struct {
	integrationpb.UnimplementedAWSOIDCServiceServer

	integrationService    *Service
	authorizer            authz.Authorizer
	logger                *slog.Logger
	clock                 clockwork.Clock
	proxyPublicAddrGetter func() string
	cache                 CacheAWSOIDC
}

// CacheAWSOIDC is the subset of the cached resources that the Service queries.
type CacheAWSOIDC interface {
	// GetToken returns a provision token by name.
	GetToken(ctx context.Context, name string) (types.ProvisionToken, error)

	// UpsertToken creates or updates a provision token.
	UpsertToken(ctx context.Context, token types.ProvisionToken) error

	// GetClusterName returns the current cluster name.
	GetClusterName(...services.MarshalOption) (types.ClusterName, error)
}

// NewAWSOIDCService returns a new AWSOIDCService.
func NewAWSOIDCService(cfg *AWSOIDCServiceConfig) (*AWSOIDCService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &AWSOIDCService{
		integrationService:    cfg.IntegrationService,
		logger:                cfg.Logger,
		authorizer:            cfg.Authorizer,
		proxyPublicAddrGetter: cfg.ProxyPublicAddrGetter,
		clock:                 cfg.Clock,
		cache:                 cfg.Cache,
	}, nil
}

var _ integrationpb.AWSOIDCServiceServer = (*AWSOIDCService)(nil)

func (s *AWSOIDCService) awsClientReq(ctx context.Context, integrationName, region string) (*awsoidc.AWSClientRequest, error) {
	integration, err := s.integrationService.GetIntegration(ctx, &integrationpb.GetIntegrationRequest{
		Name: integrationName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if integration.GetSubKind() != types.IntegrationSubKindAWSOIDC {
		return nil, trace.BadParameter("integration subkind (%s) mismatch", integration.GetSubKind())
	}

	if integration.GetAWSOIDCIntegrationSpec() == nil {
		return nil, trace.BadParameter("missing spec fields for %q (%q) integration", integration.GetName(), integration.GetSubKind())
	}

	token, err := s.integrationService.generateAWSOIDCTokenWithoutAuthZ(ctx, integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &awsoidc.AWSClientRequest{
		IntegrationName: integrationName,
		Token:           token.Token,
		RoleARN:         integration.GetAWSOIDCIntegrationSpec().RoleARN,
		Region:          region,
	}, nil
}

// ListEICE returns a paginated list of EC2 Instance Connect Endpoints.
func (s *AWSOIDCService) ListEICE(ctx context.Context, req *integrationpb.ListEICERequest) (*integrationpb.ListEICEResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := s.awsClientReq(ctx, req.Integration, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listClient, err := awsoidc.NewListEC2ICEClient(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listResp, err := awsoidc.ListEC2ICE(ctx, listClient, awsoidc.ListEC2ICERequest{
		Region:    req.Region,
		VPCIDs:    req.VpcIds,
		NextToken: req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	eiceList := make([]*integrationpb.EC2InstanceConnectEndpoint, 0, len(listResp.EC2ICEs))
	for _, e := range listResp.EC2ICEs {
		eiceList = append(eiceList, &integrationpb.EC2InstanceConnectEndpoint{
			Name:          e.Name,
			State:         e.State,
			StateMessage:  e.StateMessage,
			DashboardLink: e.DashboardLink,
			SubnetId:      e.SubnetID,
			VpcId:         e.VPCID,
		})
	}

	return &integrationpb.ListEICEResponse{
		NextToken:     listResp.NextToken,
		Ec2Ices:       eiceList,
		DashboardLink: listResp.DashboardLink,
	}, nil
}

// CreateEICE creates multiple EC2 Instance Connect Endpoint using the provided Subnets and Security Group IDs.
func (s *AWSOIDCService) CreateEICE(ctx context.Context, req *integrationpb.CreateEICERequest) (*integrationpb.CreateEICEResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := s.awsClientReq(ctx, req.Integration, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	createClient, err := awsoidc.NewCreateEC2ICEClient(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.cache.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	endpoints := make([]awsoidc.EC2ICEEndpoint, 0, len(req.Endpoints))
	for _, endpoint := range req.Endpoints {
		endpoints = append(endpoints, awsoidc.EC2ICEEndpoint{
			Name:             endpoint.Name,
			SubnetID:         endpoint.SubnetId,
			SecurityGroupIDs: endpoint.SecurityGroupIds,
		})
	}

	createResp, err := awsoidc.CreateEC2ICE(ctx, createClient, awsoidc.CreateEC2ICERequest{
		Cluster:         clusterName.GetClusterName(),
		IntegrationName: req.Integration,
		Endpoints:       endpoints,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	eiceList := make([]*integrationpb.EC2ICEndpoint, 0, len(createResp.CreatedEndpoints))
	for _, e := range createResp.CreatedEndpoints {
		eiceList = append(eiceList, &integrationpb.EC2ICEndpoint{
			Name:             e.Name,
			SubnetId:         e.SubnetID,
			SecurityGroupIds: e.SecurityGroupIDs,
		})
	}

	return &integrationpb.CreateEICEResponse{
		Name:             createResp.Name,
		CreatedEndpoints: eiceList,
	}, nil
}

// ListDatabases returns a paginated list of Databases.
func (s *AWSOIDCService) ListDatabases(ctx context.Context, req *integrationpb.ListDatabasesRequest) (*integrationpb.ListDatabasesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := s.awsClientReq(ctx, req.Integration, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listDBsClient, err := awsoidc.NewListDatabasesClient(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listDBsResp, err := awsoidc.ListDatabases(ctx, listDBsClient, awsoidc.ListDatabasesRequest{
		Region:    req.Region,
		RDSType:   req.RdsType,
		Engines:   req.Engines,
		NextToken: req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbList := make([]*types.DatabaseV3, 0, len(listDBsResp.Databases))
	for _, db := range listDBsResp.Databases {
		dbV3, ok := db.(*types.DatabaseV3)
		if !ok {
			s.logger.WarnContext(ctx, "Skipping database because conversion to DatabaseV3 failed",
				"database", db.GetName(),
				"type", logutils.TypeAttr(db),
				"error", err,
			)
			continue
		}
		dbList = append(dbList, dbV3)
	}

	return &integrationpb.ListDatabasesResponse{
		Databases: dbList,
		NextToken: listDBsResp.NextToken,
	}, nil
}

// ListSecurityGroups returns a paginated list of SecurityGroups.
func (s *AWSOIDCService) ListSecurityGroups(ctx context.Context, req *integrationpb.ListSecurityGroupsRequest) (*integrationpb.ListSecurityGroupsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := s.awsClientReq(ctx, req.Integration, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listSGsClient, err := awsoidc.NewListSecurityGroupsClient(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listSGsResp, err := awsoidc.ListSecurityGroups(ctx, listSGsClient, awsoidc.ListSecurityGroupsRequest{
		VPCID:     req.VpcId,
		NextToken: req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sgList := make([]*integrationpb.SecurityGroup, 0, len(listSGsResp.SecurityGroups))
	for _, sg := range listSGsResp.SecurityGroups {
		sgList = append(sgList, &integrationpb.SecurityGroup{
			Name:          sg.Name,
			Id:            sg.ID,
			Description:   sg.Description,
			InboundRules:  convertSecurityGroupRulesToProto(sg.InboundRules),
			OutboundRules: convertSecurityGroupRulesToProto(sg.OutboundRules),
		})
	}

	return &integrationpb.ListSecurityGroupsResponse{
		SecurityGroups: sgList,
		NextToken:      listSGsResp.NextToken,
	}, nil
}

func convertSecurityGroupRulesToProto(inRules []awsoidc.SecurityGroupRule) []*integrationpb.SecurityGroupRule {
	out := make([]*integrationpb.SecurityGroupRule, 0, len(inRules))
	for _, r := range inRules {
		cidrs := make([]*integrationpb.SecurityGroupRuleCIDR, 0, len(r.CIDRs))
		for _, cidr := range r.CIDRs {
			cidrs = append(cidrs, &integrationpb.SecurityGroupRuleCIDR{
				Cidr:        cidr.CIDR,
				Description: cidr.Description,
			})
		}
		out = append(out, &integrationpb.SecurityGroupRule{
			IpProtocol: r.IPProtocol,
			FromPort:   int32(r.FromPort),
			ToPort:     int32(r.ToPort),
			Cidrs:      cidrs,
		})
	}
	return out
}

// DeployDatabaseService deploys Database Services into Amazon ECS.
func (s *AWSOIDCService) DeployDatabaseService(ctx context.Context, req *integrationpb.DeployDatabaseServiceRequest) (*integrationpb.DeployDatabaseServiceResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.cache.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := s.awsClientReq(ctx, req.Integration, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deployServiceClient, err := awsoidc.NewDeployServiceClient(ctx, awsClientReq, s.cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deployments := make([]awsoidc.DeployDatabaseServiceRequestDeployment, 0, len(req.Deployments))
	for _, d := range req.Deployments {
		deployments = append(deployments, awsoidc.DeployDatabaseServiceRequestDeployment{
			VPCID:               d.VpcId,
			SubnetIDs:           d.SubnetIds,
			SecurityGroupIDs:    d.SecurityGroups,
			DeployServiceConfig: d.TeleportConfigString,
		})
	}

	deployDBResp, err := awsoidc.DeployDatabaseService(ctx, deployServiceClient, awsoidc.DeployDatabaseServiceRequest{
		Region:                  req.Region,
		TaskRoleARN:             req.TaskRoleArn,
		TeleportVersionTag:      req.TeleportVersion,
		DeploymentJoinTokenName: req.DeploymentJoinTokenName,
		Deployments:             deployments,
		TeleportClusterName:     clusterName.GetClusterName(),
		IntegrationName:         req.Integration,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &integrationpb.DeployDatabaseServiceResponse{
		ClusterArn:          deployDBResp.ClusterARN,
		ClusterDashboardUrl: deployDBResp.ClusterDashboardURL,
	}, nil
}

// EnrollEKSClusters enrolls EKS clusters into Teleport by installing teleport-kube-agent chart on the clusters.
func (s *AWSOIDCService) EnrollEKSClusters(ctx context.Context, req *integrationpb.EnrollEKSClustersRequest) (*integrationpb.EnrollEKSClustersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	publicProxyAddr := s.proxyPublicAddrGetter()
	if publicProxyAddr == "" {
		return nil, trace.BadParameter("could not get public proxy address.")
	}

	awsClientReq, err := s.awsClientReq(ctx, req.Integration, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	credsProvider, err := awsoidc.NewAWSCredentialsProvider(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	enrollEKSClient, err := awsoidc.NewEnrollEKSClustersClient(ctx, awsClientReq, s.cache.UpsertToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	features := modules.GetModules().Features()

	enrollmentResponse, err := awsoidc.EnrollEKSClusters(ctx, s.logger, s.clock, publicProxyAddr, credsProvider, enrollEKSClient, awsoidc.EnrollEKSClustersRequest{
		Region:             req.Region,
		ClusterNames:       req.GetEksClusterNames(),
		EnableAppDiscovery: req.EnableAppDiscovery,
		EnableAutoUpgrades: features.AutomaticUpgrades,
		IsCloud:            features.Cloud,
		AgentVersion:       req.AgentVersion,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := make([]*integrationpb.EnrollEKSClusterResult, 0, len(enrollmentResponse.Results))
	for _, r := range enrollmentResponse.Results {
		results = append(results, &integrationpb.EnrollEKSClusterResult{
			EksClusterName: r.ClusterName,
			ResourceId:     r.ResourceId,
			Error:          trace.UserMessage(r.Error),
		})
	}

	return &integrationpb.EnrollEKSClustersResponse{
		Results: results,
	}, nil
}

// DeployService deploys Services into Amazon ECS.
func (s *AWSOIDCService) DeployService(ctx context.Context, req *integrationpb.DeployServiceRequest) (*integrationpb.DeployServiceResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.cache.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := s.awsClientReq(ctx, req.Integration, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deployServiceClient, err := awsoidc.NewDeployServiceClient(ctx, awsClientReq, s.cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deployServiceResp, err := awsoidc.DeployService(ctx, deployServiceClient, awsoidc.DeployServiceRequest{
		DeploymentJoinTokenName: req.DeploymentJoinTokenName,
		DeploymentMode:          req.DeploymentMode,
		TeleportConfigString:    req.TeleportConfigString,
		IntegrationName:         req.Integration,
		Region:                  req.Region,
		SecurityGroups:          req.SecurityGroups,
		SubnetIDs:               req.SubnetIds,
		TaskRoleARN:             req.TaskRoleArn,
		TeleportClusterName:     clusterName.GetClusterName(),
		TeleportVersionTag:      req.TeleportVersion,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &integrationpb.DeployServiceResponse{
		ClusterArn:          deployServiceResp.ClusterARN,
		ServiceArn:          deployServiceResp.ServiceARN,
		TaskDefinitionArn:   deployServiceResp.TaskDefinitionARN,
		ServiceDashboardUrl: deployServiceResp.ServiceDashboardURL,
	}, nil
}

// ListEC2 returns a paginated list of AWS EC2 instances.
func (s *AWSOIDCService) ListEC2(ctx context.Context, req *integrationpb.ListEC2Request) (*integrationpb.ListEC2Response, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := s.awsClientReq(ctx, req.Integration, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listEC2Client, err := awsoidc.NewListEC2Client(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listEC2Resp, err := awsoidc.ListEC2(ctx, listEC2Client, awsoidc.ListEC2Request{
		Region:      req.Region,
		Integration: req.Integration,
		NextToken:   req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverList := make([]*types.ServerV2, 0, len(listEC2Resp.Servers))
	for _, server := range listEC2Resp.Servers {
		serverV2, ok := server.(*types.ServerV2)
		if !ok {
			s.logger.WarnContext(ctx, "Skipping server because conversion to ServerV2 failed",
				"server", server.GetName(),
				"type", logutils.TypeAttr(server),
				"error", err,
			)
			continue
		}
		serverList = append(serverList, serverV2)
	}

	return &integrationpb.ListEC2Response{
		Servers:   serverList,
		NextToken: listEC2Resp.NextToken,
	}, nil
}

// ListEKSCluster returns a paginated list of AWS EKS Clusters.
func (s *AWSOIDCService) ListEKSClusters(ctx context.Context, req *integrationpb.ListEKSClustersRequest) (*integrationpb.ListEKSClustersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := s.awsClientReq(ctx, req.Integration, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listEKSClustersClient, err := awsoidc.NewListEKSClustersClient(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listEKSClustersResp, err := awsoidc.ListEKSClusters(ctx, listEKSClustersClient, awsoidc.ListEKSClustersRequest{
		Region:    req.Region,
		NextToken: req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clustersList := make([]*integrationpb.EKSCluster, 0, len(listEKSClustersResp.Clusters))
	for _, cluster := range listEKSClustersResp.Clusters {
		clusterPb := &integrationpb.EKSCluster{
			Name:       cluster.Name,
			Region:     cluster.Region,
			Arn:        cluster.Arn,
			Labels:     cluster.Labels,
			JoinLabels: cluster.JoinLabels,
			Status:     cluster.Status,
		}
		clustersList = append(clustersList, clusterPb)
	}

	return &integrationpb.ListEKSClustersResponse{
		Clusters:  clustersList,
		NextToken: listEKSClustersResp.NextToken,
	}, nil
}
