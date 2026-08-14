package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssdkconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/aws/config"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/credprovider"
	awsregion "github.com/gravitational/teleport/lib/utils/aws/region"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

// BuildAWSConfig is a helper function allowing to build AWS config with the given region, role ARN and role tags.
func BuildAWSConfig(ctx context.Context, region, roleARN string, roleTags map[string]string, option ...func(*awssdkconfig.LoadOptions) error) (aws.Config, error) {
	option = append(option, awssdkconfig.WithRegion(region))
	awsConfig, err := config.LoadDefaultConfig(ctx, option...)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	awsConfig.Credentials = aws.NewCredentialsCache(
		stscreds.NewAssumeRoleProvider(
			stsutils.NewFromConfig(awsConfig),
			roleARN,
			func(options *stscreds.AssumeRoleOptions) {
				var tags []ststypes.Tag
				for k, v := range roleTags {
					tags = append(tags, ststypes.Tag{
						Key:   aws.String(k),
						Value: aws.String(v),
					})
				}
				options.Tags = tags
			},
		),
	)
	return awsConfig, nil
}

// CreateAWSConfigForIntegration wraps the credprovider.CreateAWSConfigForIntegration function to include region validation.
//
// WARNING: The region input, if taken from an external source like a user HTTP request, can be exploited for SSRF attacks.
// The AWS SDK does not validate the region and allows URLs to be passed, which can be used to make requests
// to AWS services with valid AWS credentials.
//
// TODO(smallinsky) Remove and add validation to credprovider.CreateAWSConfigForIntegration after SSRF vulnerability is fixed.
func CreateAWSConfigForIntegration(ctx context.Context, config credprovider.Config, option ...credprovider.Option) (*aws.Config, error) {
	if err := ValidateAWSRegion(config.Region); err != nil {
		return nil, trace.Wrap(err)
	}
	c, err := credprovider.CreateAWSConfigForIntegration(ctx, config, option...)
	return c, trace.Wrap(err)
}

// ValidateAWSRegion checks if the given region is a valid AWS region
func ValidateAWSRegion(region string) error {
	if !awsregion.IsKnownRegion(region) {
		return trace.BadParameter("region %q is invalid", region)
	}
	return nil
}
