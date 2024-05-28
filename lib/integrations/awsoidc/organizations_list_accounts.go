package awsoidc

import (
	"context"
	"time"

	awsOrgs "github.com/aws/aws-sdk-go-v2/service/organizations"
	awsOrgsTypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/gravitational/trace"
)

type ListOrgAccountsClient interface {
	ListAccounts(ctx context.Context, params *awsOrgs.ListAccountsInput, optFns ...func(*awsOrgs.Options)) (*awsOrgs.ListAccountsOutput, error)
}

func NewListOrgAccountsClient(ctx context.Context, req *AWSClientRequest) (ListOrgAccountsClient, error) {
	clt, err := newOrganizationsClient(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

type OrgAccount struct {
	ARN          string
	ID           string
	Name         string
	JoinedTime   time.Time
	JoinedMethod awsOrgsTypes.AccountJoinedMethod
	Email        string
	Status       awsOrgsTypes.AccountStatus
}

type ListOrgAccountsResponse struct {
	Accounts  []OrgAccount
	NextToken string
}

func unbox[T any](p *T) T {
	var result T
	if p != nil {
		result = *p
	}
	return result
}

func ListOrgAccounts(ctx context.Context, clt ListOrgAccountsClient, nextToken string) (*ListOrgAccountsResponse, error) {
	if clt == nil {
		return nil, trace.BadParameter("client required")
	}

	var listArgs awsOrgs.ListAccountsInput
	if nextToken != "" {
		listArgs.NextToken = &nextToken
	}

	resp, err := clt.ListAccounts(ctx, &listArgs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := &ListOrgAccountsResponse{
		Accounts: make([]OrgAccount, len(resp.Accounts)),
	}
	for i, src := range resp.Accounts {
		result.Accounts[i] = OrgAccount{
			ARN:          unbox(src.Arn),
			ID:           unbox(src.Id),
			Name:         unbox(src.Name),
			Email:        unbox(src.Email),
			JoinedTime:   unbox(src.JoinedTimestamp),
			JoinedMethod: src.JoinedMethod,
			Status:       src.Status,
		}
	}
	if resp.NextToken != nil {
		result.NextToken = *resp.NextToken
	}

	return result, nil
}
