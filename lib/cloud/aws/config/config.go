// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package config

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/modules"
)

const defaultRegion = "us-east-1"

// credentialsSource defines where the credentials must come from.
type credentialsSource int

const (
	// credentialsSourceAmbient uses the default Cloud SDK method to load the credentials.
	credentialsSourceAmbient = iota + 1
	// credentialsSourceIntegration uses an Integration to load the credentials.
	credentialsSourceIntegration
)

// AWSIntegrationSessionProvider defines a function that creates a credential provider from a region and an integration.
// This is used to generate aws configs for clients that must use an integration instead of ambient credentials.
type AWSIntegrationCredentialProvider func(ctx context.Context, region, integration string) (aws.CredentialsProvider, error)

// awsOptions is a struct of additional options for assuming an AWS role
// when construction an underlying AWS config.
type awsOptions struct {
	// baseConfigis a config to use instead of the default config for an
	// AWS region, which is used to enable role chaining.
	baseConfig *aws.Config
	// assumeRoleARN is the AWS IAM Role ARN to assume.
	assumeRoleARN string
	// assumeRoleExternalID is used to assume an external AWS IAM Role.
	assumeRoleExternalID string
	// credentialsSource describes which source to use to fetch credentials.
	credentialsSource credentialsSource
	// integration is the name of the integration to be used to fetch the credentials.
	integration string
	// awsIntegrationCredentialsProvider is the integration credential provider to use.
	awsIntegrationCredentialsProvider AWSIntegrationCredentialProvider
	// customRetryer is a custom retryer to use for the config.
	customRetryer func() aws.Retryer
	// maxRetries is the maximum number of retries to use for the config.
	maxRetries *int
}

func (a *awsOptions) checkAndSetDefaults() error {
	switch a.credentialsSource {
	case credentialsSourceAmbient:
		if a.integration != "" {
			return trace.BadParameter("integration and ambient credentials cannot be used at the same time")
		}
	case credentialsSourceIntegration:
		if a.integration == "" {
			return trace.BadParameter("missing integration name")
		}
	default:
		return trace.BadParameter("missing credentials source (ambient or integration)")
	}

	return nil
}

// AWSOptionsFn is an option function for setting additional options
// when getting an AWS config.
type AWSOptionsFn func(*awsOptions)

// WithAssumeRole configures options needed for assuming an AWS role.
func WithAssumeRole(roleARN, externalID string) AWSOptionsFn {
	return func(options *awsOptions) {
		options.assumeRoleARN = roleARN
		options.assumeRoleExternalID = externalID
	}
}

// WithRetryer sets a custom retryer for the config.
func WithRetryer(retryer func() aws.Retryer) AWSOptionsFn {
	return func(options *awsOptions) {
		options.customRetryer = retryer
	}
}

// WithMaxRetries sets the maximum allowed value for the sdk to keep retrying.
func WithMaxRetries(maxRetries int) AWSOptionsFn {
	return func(options *awsOptions) {
		options.maxRetries = &maxRetries
	}
}

// WithCredentialsMaybeIntegration sets the credential source to be
// - ambient if the integration is an empty string
// - integration, otherwise
func WithCredentialsMaybeIntegration(integration string) AWSOptionsFn {
	if integration != "" {
		return withIntegrationCredentials(integration)
	}

	return WithAmbientCredentials()
}

// withIntegrationCredentials configures options with an Integration that must be used to fetch Credentials to assume a role.
// This prevents the usage of AWS environment credentials.
func withIntegrationCredentials(integration string) AWSOptionsFn {
	return func(options *awsOptions) {
		options.credentialsSource = credentialsSourceIntegration
		options.integration = integration
	}
}

// WithAmbientCredentials configures options to use the ambient credentials.
func WithAmbientCredentials() AWSOptionsFn {
	return func(options *awsOptions) {
		options.credentialsSource = credentialsSourceAmbient
	}
}

// WithAWSIntegrationCredentialProvider sets the integration credential provider.
func WithAWSIntegrationCredentialProvider(cred AWSIntegrationCredentialProvider) AWSOptionsFn {
	return func(options *awsOptions) {
		options.awsIntegrationCredentialsProvider = cred
	}
}

// GetAWSConfig returns an AWS config for the specified region, optionally
// assuming AWS IAM Roles.
func GetAWSConfig(ctx context.Context, region string, opts ...AWSOptionsFn) (aws.Config, error) {
	var options awsOptions
	for _, opt := range opts {
		opt(&options)
	}
	if options.baseConfig == nil {
		cfg, err := getAWSConfigForRegion(ctx, region, options)
		if err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
		options.baseConfig = &cfg
	}
	if options.assumeRoleARN == "" {
		return *options.baseConfig, nil
	}
	return getAWSConfigForRole(ctx, region, options)
}

// awsAmbientConfigProvider loads a new config using the environment variables.
func awsAmbientConfigProvider(region string, cred aws.CredentialsProvider, options awsOptions) (aws.Config, error) {
	opts := buildAWSConfigOptions(region, cred, options)
	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	return cfg, trace.Wrap(err)
}

func buildAWSConfigOptions(region string, cred aws.CredentialsProvider, options awsOptions) []func(*config.LoadOptions) error {
	opts := []func(*config.LoadOptions) error{
		config.WithDefaultRegion(defaultRegion),
		config.WithRegion(region),
		config.WithCredentialsProvider(cred),
	}
	if modules.GetModules().IsBoringBinary() {
		opts = append(opts, config.WithUseFIPSEndpoint(aws.FIPSEndpointStateEnabled))
	}
	if options.customRetryer != nil {
		opts = append(opts, config.WithRetryer(options.customRetryer))
	}
	if options.maxRetries != nil {
		opts = append(opts, config.WithRetryMaxAttempts(*options.maxRetries))
	}
	return opts
}

// getAWSConfigForRegion returns AWS config for the specified region.
func getAWSConfigForRegion(ctx context.Context, region string, options awsOptions) (aws.Config, error) {
	if err := options.checkAndSetDefaults(); err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	var cred aws.CredentialsProvider
	if options.credentialsSource == credentialsSourceIntegration {
		if options.awsIntegrationCredentialsProvider == nil {
			return aws.Config{}, trace.BadParameter("missing aws integration credential provider")
		}

		slog.DebugContext(ctx, "Initializing AWS config with integration", "region", region, "integration", options.integration)
		var err error
		cred, err = options.awsIntegrationCredentialsProvider(ctx, region, options.integration)
		if err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
	} else {
		slog.DebugContext(ctx, "Initializing AWS config from environment", "region", region)
	}

	cfg, err := awsAmbientConfigProvider(region, cred, options)
	return cfg, trace.Wrap(err)
}

// getAWSConfigForRole returns an AWS config for the specified region and role.
func getAWSConfigForRole(ctx context.Context, region string, options awsOptions) (aws.Config, error) {
	if err := options.checkAndSetDefaults(); err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	stsClient := sts.NewFromConfig(*options.baseConfig)
	cred := stscreds.NewAssumeRoleProvider(stsClient, options.assumeRoleARN, func(aro *stscreds.AssumeRoleOptions) {
		if options.assumeRoleExternalID != "" {
			aro.ExternalID = aws.String(options.assumeRoleExternalID)
		}
	})
	if _, err := cred.Retrieve(ctx); err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	opts := buildAWSConfigOptions(region, cred, options)
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	return cfg, trace.Wrap(err)
}
