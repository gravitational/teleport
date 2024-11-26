package identitycenter

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/integrations/awsoidc/credprovider"
)

// passThrough generates a function that casts a strongly typed map into its
// underlying map type and returns it. For use with the reconciler data getters.
func passThrough[M ~map[K]V, K comparable, V any](m M) func() map[K]V {
	return func() map[K]V {
		return map[K]V(m)
	}
}

var allowCharacters = regexp.MustCompile(`^[0-9a-z\-@:]+$`)

func normalizeResourceName(name string) string {
	name = strings.ToLower(name)

	var sb strings.Builder
	for _, r := range name {
		if allowCharacters.MatchString(string(r)) {
			sb.WriteRune(r)
			continue
		}
		// Replace disallowed characters with '_'
		if sb.Len() > 0 && sb.String()[sb.Len()-1] != '_' {
			sb.WriteRune('_')
		}
	}
	return strings.Trim(sb.String(), "_")
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

// validAWSRegions is a list of valid AWS regions result of:
// curl -s https://raw.githubusercontent.com/aws/aws-sdk-go-v2/main/internal/endpoints/awsrulesfn/partitions.json | jq -r '.partitions[].regions | keys[]' | awk 'BEGIN { print "var validAWSRegions = []string{" } { printf "\t\"%s\",\n", $0 } END { print "}" }'
// TODO(smallinsky) move to OSS and replace by go generate.
var validAWSRegions = []string{
	"af-south-1",
	"ap-east-1",
	"ap-northeast-1",
	"ap-northeast-2",
	"ap-northeast-3",
	"ap-south-1",
	"ap-south-2",
	"ap-southeast-1",
	"ap-southeast-2",
	"ap-southeast-3",
	"ap-southeast-4",
	"ap-southeast-5",
	"aws-global",
	"ca-central-1",
	"ca-west-1",
	"eu-central-1",
	"eu-central-2",
	"eu-north-1",
	"eu-south-1",
	"eu-south-2",
	"eu-west-1",
	"eu-west-2",
	"eu-west-3",
	"il-central-1",
	"me-central-1",
	"me-south-1",
	"sa-east-1",
	"us-east-1",
	"us-east-2",
	"us-west-1",
	"us-west-2",
	"aws-cn-global",
	"cn-north-1",
	"cn-northwest-1",
	"aws-us-gov-global",
	"us-gov-east-1",
	"us-gov-west-1",
	"aws-iso-global",
	"us-iso-east-1",
	"us-iso-west-1",
	"aws-iso-b-global",
	"us-isob-east-1",
	"eu-isoe-west-1",
}

// ValidateAWSRegion checks if the given region is a valid AWS region
func ValidateAWSRegion(region string) error {
	if !slices.Contains(validAWSRegions, region) {
		return trace.BadParameter("region %q is invalid", region)
	}
	return nil
}
