package integrationv1

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"google.golang.org/protobuf/types/known/timestamppb"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/trace"
)

func (s *AWSOIDCService) clientFromHeader(ctx context.Context, h *integrationpb.RequestHeader) (awsoidc.IDCClient, error) {
	if h == nil {
		return nil, trace.BadParameter("request header must not be nil")
	}

	cr, err := s.awsClientReq(ctx, h.IntegrationId, h.AwsRegion)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := awsoidc.NewIDCClient(ctx, cr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

func (s *AWSOIDCService) DescribeIdentityCenterInstance(
	ctx context.Context,
	req *integrationpb.DescribeIdentityCenterInstanceRequest) (*integrationpb.DescribeIdentityCenterInstanceResponse, error) {

	awsClient, err := s.clientFromHeader(ctx, req.Header)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	srcInstance, err := awsClient.DescribeInstance(ctx, &ssoadmin.DescribeInstanceInput{
		InstanceArn: &req.Arn,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	status, err := InstanceStatusToPB(srcInstance.Status)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := &integrationpb.DescribeIdentityCenterInstanceResponse{
		Name:            valOrEmpty(srcInstance.Name),
		InstanceArn:     valOrEmpty(srcInstance.InstanceArn),
		IdentityStoreId: valOrEmpty(srcInstance.IdentityStoreId),
		OwnerAccountId:  valOrEmpty(srcInstance.OwnerAccountId),
		CreatedDate:     maybeTimestampPB(srcInstance.CreatedDate),
		Status:          status,
	}

	return result, nil
}

func maybeTimestampPB(src *time.Time) *timestamppb.Timestamp {
	if src == nil {
		return nil
	}
	return timestamppb.New(*src)
}

func valOrEmpty(src *string) string {
	if src == nil {
		return ""
	}
	return *src
}

func nilIfEmpty(ps *string) *string {
	if *ps == "" {
		return nil
	}
	return ps
}

func InstanceStatusToPB(src ssoadmintypes.InstanceStatus) (integrationpb.IdentityCenterInstanceStatus, error) {
	switch src {
	case ssoadmintypes.InstanceStatusActive:
		return integrationpb.IdentityCenterInstanceStatus_IDENTITY_CENTER_INSTANCE_STATUS_ACTIVE, nil

	case ssoadmintypes.InstanceStatusCreateInProgress:
		return integrationpb.IdentityCenterInstanceStatus_IDENTITY_CENTER_INSTANCE_STATUS_CREATE_IN_PROGRESS, nil

	case ssoadmintypes.InstanceStatusDeleteInProgress:
		return integrationpb.IdentityCenterInstanceStatus_IDENTITY_CENTER_INSTANCE_STATUS_DELETE_IN_PROGRESS, nil
	}

	return integrationpb.IdentityCenterInstanceStatus_IDENTITY_CENTER_INSTANCE_STATUS_UNSPECIFIED,
		trace.BadParameter("invalid InstanceStatus value %v", src)
}

func InstanceStatusFromPB(src integrationpb.IdentityCenterInstanceStatus) (ssoadmintypes.InstanceStatus, error) {
	switch src {
	case integrationpb.IdentityCenterInstanceStatus_IDENTITY_CENTER_INSTANCE_STATUS_ACTIVE:
		return ssoadmintypes.InstanceStatusActive, nil

	case integrationpb.IdentityCenterInstanceStatus_IDENTITY_CENTER_INSTANCE_STATUS_CREATE_IN_PROGRESS:
		return ssoadmintypes.InstanceStatusCreateInProgress, nil

	case integrationpb.IdentityCenterInstanceStatus_IDENTITY_CENTER_INSTANCE_STATUS_DELETE_IN_PROGRESS:
		return ssoadmintypes.InstanceStatusDeleteInProgress, nil
	}

	return "", trace.BadParameter("Invalid InstanceStatus enum value %d", src)
}

func (s *AWSOIDCService) identityStoreClientFromHeader(ctx context.Context, h *integrationpb.RequestHeader) (awsoidc.IdentityStoreClient, error) {
	if h == nil {
		return nil, trace.BadParameter("request header must not be nil")
	}

	cr, err := s.awsClientReq(ctx, h.IntegrationId, h.AwsRegion)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := awsoidc.NewIdentityStoreClient(ctx, cr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

func (s *AWSOIDCService) ListIdentityStoreUsers(ctx context.Context, req *integrationpb.ListIdentityStoreUsersRequest) (*integrationpb.ListIdentityStoreUsersResponse, error) {

	awsClient, err := s.identityStoreClientFromHeader(ctx, req.Header)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := awsClient.ListUsers(ctx, &identitystore.ListUsersInput{
		IdentityStoreId: &req.IdentityStoreId,
		NextToken:       nilIfEmpty(&req.NextToken),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var users []*integrationpb.IdentityStoreUser
	for _, src := range resp.Users {
		dst := integrationpb.IdentityStoreUser{
			IdentityStoreId: valOrEmpty(src.UserId),
			UserId:          valOrEmpty(src.UserId),
			Name:            valOrEmpty(src.UserName),
		}
		users = append(users, &dst)
	}

	result := &integrationpb.ListIdentityStoreUsersResponse{
		Users:     users,
		NextToken: valOrEmpty(resp.NextToken),
	}

	return result, nil
}
