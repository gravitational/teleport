package aws_sync

import (
	"context"
	"github.com/aws/aws-sdk-go/service/organizations"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/trace"
)

func (a *awsFetcher) pollAWSAccounts(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		return nil
	}
}

func (a *awsFetcher) fetchAWSAccounts(ctx context.Context, result *Resources) ([]*accessgraphv1alpha.AWSAccountV1, error) {
	var accounts []*accessgraphv1alpha.AWSAccountV1
	orgClient, err := a.CloudClients.GetAWSOrganizationsClient(ctx, "us-west2", a.getAWSOptions()...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = orgClient.ListAccountsPagesWithContext(
		ctx,
		&organizations.ListAccountsInput{},
		func(output *organizations.ListAccountsOutput, lastPage bool) bool {
			for _, account := range output.Accounts {
				tAcct := &accessgraphv1alpha.AWSAccountV1{
					Arn:         *account.Arn,
					AccountId:   *account.Id,
					AccountName: *account.Name,
				}
				accounts = append(accounts, tAcct)
			}
			return output.NextToken != nil
		})
	return accounts, nil
}
