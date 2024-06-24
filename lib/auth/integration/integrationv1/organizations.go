package integrationv1

import (
	"context"

	awsOrgsTypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
)

func box[T any](value T) *T {
	pv := new(T)
	(*pv) = value
	return pv
}

func (s *AWSOIDCService) ListOrganizationAccounts(ctx context.Context, req *integrationpb.ListOrganizationAccountsRequest) (*integrationpb.ListOrganizationAccountsResponse, error) {
	if req == nil {
		return nil, trace.BadParameter("request may not be nil")
	}

	if req.Header == nil {
		return nil, trace.BadParameter("request header may not be nil")
	}

	cr, err := s.awsClientReq(ctx, req.Header.IntegrationId, req.Header.AwsRegion)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := awsoidc.NewListOrgAccountsClient(ctx, cr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := awsoidc.ListOrgAccounts(ctx, clt, req.NextToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := &integrationpb.ListOrganizationAccountsResponse{
		Accounts:  make([]*integrationpb.Account, len(resp.Accounts)),
		NextToken: resp.NextToken,
	}
	for i, src := range resp.Accounts {
		joinMethod, err := AccountJoinedMethodToPB(src.JoinedMethod)
		if err != nil {
			// log error
			continue
		}

		status, err := AccountStatusToPB(src.Status)
		if err != nil {
			// log error
			continue
		}

		result.Accounts[i] = &integrationpb.Account{
			Arn:        src.ARN,
			Id:         src.ID,
			Name:       src.Name,
			Email:      src.Email,
			JoinDate:   timestamppb.New(src.JoinedTime),
			JoinMethod: joinMethod,
			Status:     status,
		}
	}

	return result, nil
}

func AccountJoinedMethodToPB(m awsOrgsTypes.AccountJoinedMethod) (integrationpb.AccountJoinedMethod, error) {
	switch m {
	case awsOrgsTypes.AccountJoinedMethodInvited:
		return integrationpb.AccountJoinedMethod_ACCOUNT_JOINED_METHOD_INVITED, nil

	case awsOrgsTypes.AccountJoinedMethodCreated:
		return integrationpb.AccountJoinedMethod_ACCOUNT_JOINED_METHOD_CREATED, nil
	}

	return integrationpb.AccountJoinedMethod_ACCOUNT_JOINED_METHOD_UNSPECIFIED,
		trace.BadParameter("invalid AccountJoinedMethod value %v", m)
}

func AccountJoinedMethodFromPB(m integrationpb.AccountJoinedMethod) (awsOrgsTypes.AccountJoinedMethod, error) {
	switch m {
	case integrationpb.AccountJoinedMethod_ACCOUNT_JOINED_METHOD_INVITED:
		return awsOrgsTypes.AccountJoinedMethodInvited, nil

	case integrationpb.AccountJoinedMethod_ACCOUNT_JOINED_METHOD_CREATED:
		return awsOrgsTypes.AccountJoinedMethodCreated, nil
	}

	return "", trace.BadParameter("Invalid AccountJoinedMethod enum value %d", m)
}

func AccountStatusToPB(s awsOrgsTypes.AccountStatus) (integrationpb.AccountStatus, error) {
	switch s {
	case awsOrgsTypes.AccountStatusActive:
		return integrationpb.AccountStatus_ACCOUNT_STATUS_ACTIVE, nil

	case awsOrgsTypes.AccountStatusSuspended:
		return integrationpb.AccountStatus_ACCOUNT_STATUS_SUSPENDED, nil

	case awsOrgsTypes.AccountStatusPendingClosure:
		return integrationpb.AccountStatus_ACCOUNT_STATUS_PENDING_CLOSURE, nil
	}
	return integrationpb.AccountStatus_ACCOUNT_STATUS_UNSPECIFIED,
		trace.BadParameter("invalid AccountStatus value %v", s)
}

func AccountStatusFromPB(s integrationpb.AccountStatus) (awsOrgsTypes.AccountStatus, error) {
	switch s {
	case integrationpb.AccountStatus_ACCOUNT_STATUS_ACTIVE:
		return awsOrgsTypes.AccountStatusActive, nil

	case integrationpb.AccountStatus_ACCOUNT_STATUS_SUSPENDED:
		return awsOrgsTypes.AccountStatusSuspended, nil

	case integrationpb.AccountStatus_ACCOUNT_STATUS_PENDING_CLOSURE:
		return awsOrgsTypes.AccountStatusPendingClosure, nil
	}
	return "", trace.BadParameter("invalid AccountStatus enum value %d", s)
}
