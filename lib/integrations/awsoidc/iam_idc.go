package awsoidc

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"github.com/gravitational/trace"
)

type IDCClient interface {
	ListInstances(ctx context.Context, params *ssoadmin.ListInstancesInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error)
	DescribeInstance(ctx context.Context, params *ssoadmin.DescribeInstanceInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DescribeInstanceOutput, error)
}

func NewIDCClient(ctx context.Context, req *AWSClientRequest) (IDCClient, error) {
	cfg, err := newAWSConfig(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ssoadmin.NewFromConfig(*cfg), nil
}
