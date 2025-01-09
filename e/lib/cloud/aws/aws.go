package aws

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/integrations/awsoidc/credprovider"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// BuildAWSConfig is a helper function allowing to build AWS config with the given region, role ARN and role tags.
func BuildAWSConfig(ctx context.Context, region, roleARN string, roleTags map[string]string) (aws.Config, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	awsConfig.Credentials = aws.NewCredentialsCache(
		stscreds.NewAssumeRoleProvider(sts.NewFromConfig(awsConfig), roleARN, func(options *stscreds.AssumeRoleOptions) {
			var tags []ststypes.Tag
			for k, v := range roleTags {
				tags = append(tags, ststypes.Tag{
					Key:   aws.String(k),
					Value: aws.String(v),
				})
			}
			options.Tags = tags
		}),
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
	if !awsutils.IsKnownRegion(region) {
		return trace.BadParameter("region %q is invalid", region)
	}
	return nil
}

// matchAWSICEndpointIDField matches an alphanumeric value separated by a hyphen.
var matchAWSICEndpointIDField = regexp.MustCompile(`^[a-zA-Z0-9-]*$`).MatchString

// EnsureAWSICSCIMEndpoint validates dynamic fields of SCIM base URL and returns
// a new base URL constructed from the validated field.
//
// E.g. valid SCIM base URL:
// "https://scim.ca-central-1.amazonaws.com/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v2".
// Dynamic field includes the AWS region and a random ID field:
// "https://scim.<aws-region>.amazonaws.com/<random-id>/scim/v2"
// Region value is validated against known AWS regions and the random ID field is
// validated against an alphanumeric with hyphen regexp.
// Note: The random ID field looks like a UUID field but does not confirm to
// standard UUID format defined in RFC 4122.
func EnsureAWSICSCIMEndpoint(u string) (string, error) {
	baseURL, err := url.ParseRequestURI(u)
	if err != nil {
		return "", trace.BadParameter("invalid SCIM endpoint format: %s", err.Error())
	}
	if baseURL.Scheme != "https" {
		return "", trace.BadParameter("url scheme must be https")
	}

	domainParts := strings.Split(baseURL.Hostname(), ".")
	if len(domainParts) != 4 {
		return "", trace.BadParameter("invalid SCIM endpoint format")
	}
	if domainParts[0] != "scim" {
		return "", trace.BadParameter("unrecognized SCIM endpoint")
	}
	region := domainParts[1]
	if err := ValidateAWSRegion(region); err != nil {
		return "", trace.Wrap(err)
	}
	if domainParts[2] != "amazonaws" || domainParts[3] != "com" {
		return "", trace.BadParameter("SCIM endpoint must be of 'amazonaws.com' domain")
	}

	pathParts := strings.Split(baseURL.Path, "/")
	if len(pathParts) != 4 {
		return "", trace.BadParameter("invalid SCIM endpoint format")
	}
	if !matchAWSICEndpointIDField(pathParts[1]) {
		return "", trace.BadParameter("invalid SCIM endpoint format")
	}
	if pathParts[2] != "scim" {
		return "", trace.BadParameter("unrecognized SCIM endpoint")
	}
	if pathParts[3] != "v2" {
		return "", trace.BadParameter("only v2 SCIM endpoint is supported")
	}

	newBaseURL := url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("scim.%s.amazonaws.com", region),
		Path:   fmt.Sprintf("%s/scim/v2", pathParts[1]),
	}
	return newBaseURL.String(), nil
}
