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

package awsconfig

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"

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

// IntegrationSessionProviderFunc defines a function that creates a credential provider from a region and an integration.
// This is used to generate aws configs for clients that must use an integration instead of ambient credentials.
type IntegrationCredentialProviderFunc func(ctx context.Context, region, integration string) (aws.CredentialsProvider, error)

// options is a struct of additional options for assuming an AWS role
// when construction an underlying AWS config.
type options struct {
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
	// integrationCredentialsProvider is the integration credential provider to use.
	integrationCredentialsProvider IntegrationCredentialProviderFunc
	// customRetryer is a custom retryer to use for the config.
	customRetryer func() aws.Retryer
	// maxRetries is the maximum number of retries to use for the config.
	maxRetries *int
}

func (a *options) checkAndSetDefaults() error {
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

// OptionsFn is an option function for setting additional options
// when getting an AWS config.
type OptionsFn func(*options)

// WithAssumeRole configures options needed for assuming an AWS role.
func WithAssumeRole(roleARN, externalID string) OptionsFn {
	return func(options *options) {
		options.assumeRoleARN = roleARN
		options.assumeRoleExternalID = externalID
	}
}

// WithRetryer sets a custom retryer for the config.
func WithRetryer(retryer func() aws.Retryer) OptionsFn {
	return func(options *options) {
		options.customRetryer = retryer
	}
}

// WithMaxRetries sets the maximum allowed value for the sdk to keep retrying.
func WithMaxRetries(maxRetries int) OptionsFn {
	return func(options *options) {
		options.maxRetries = &maxRetries
	}
}

// WithCredentialsMaybeIntegration sets the credential source to be
// - ambient if the integration is an empty string
// - integration, otherwise
func WithCredentialsMaybeIntegration(integration string) OptionsFn {
	if integration != "" {
		return withIntegrationCredentials(integration)
	}

	return WithAmbientCredentials()
}

// withIntegrationCredentials configures options with an Integration that must be used to fetch Credentials to assume a role.
// This prevents the usage of AWS environment credentials.
func withIntegrationCredentials(integration string) OptionsFn {
	return func(options *options) {
		options.credentialsSource = credentialsSourceIntegration
		options.integration = integration
	}
}

// WithAmbientCredentials configures options to use the ambient credentials.
func WithAmbientCredentials() OptionsFn {
	return func(options *options) {
		options.credentialsSource = credentialsSourceAmbient
	}
}

// WithIntegrationCredentialProvider sets the integration credential provider.
func WithIntegrationCredentialProvider(cred IntegrationCredentialProviderFunc) OptionsFn {
	return func(options *options) {
		options.integrationCredentialsProvider = cred
	}
}

// GetConfig returns an AWS config for the specified region, optionally
// assuming AWS IAM Roles.
func GetConfig(ctx context.Context, region string, opts ...OptionsFn) (aws.Config, error) {
	var options options
	for _, opt := range opts {
		opt(&options)
	}
	if options.baseConfig == nil {
		cfg, err := getConfigForRegion(ctx, region, options)
		if err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
		options.baseConfig = &cfg
	}
	if options.assumeRoleARN == "" {
		return *options.baseConfig, nil
	}
	return getConfigForRole(ctx, region, options)
}

// ambientConfigProvider loads a new config using the environment variables.
func ambientConfigProvider(region string, cred aws.CredentialsProvider, options options) (aws.Config, error) {
	opts := buildConfigOptions(region, cred, options)
	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	return cfg, trace.Wrap(err)
}

func buildConfigOptions(region string, cred aws.CredentialsProvider, options options) []func(*config.LoadOptions) error {
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

// getConfigForRegion returns AWS config for the specified region.
func getConfigForRegion(ctx context.Context, region string, options options) (aws.Config, error) {
	if err := options.checkAndSetDefaults(); err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	var cred aws.CredentialsProvider
	if options.credentialsSource == credentialsSourceIntegration {
		if options.integrationCredentialsProvider == nil {
			return aws.Config{}, trace.BadParameter("missing aws integration credential provider")
		}

		slog.DebugContext(ctx, "Initializing AWS config with integration", "region", region, "integration", options.integration)
		var err error
		cred, err = options.integrationCredentialsProvider(ctx, region, options.integration)
		if err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
	} else {
		slog.DebugContext(ctx, "Initializing AWS config from environment", "region", region)
	}

	cfg, err := ambientConfigProvider(region, cred, options)
	return cfg, trace.Wrap(err)
}

// getConfigForRole returns an AWS config for the specified region and role.
func getConfigForRole(ctx context.Context, region string, options options) (aws.Config, error) {
	if err := options.checkAndSetDefaults(); err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	stsClient := sts.NewFromConfig(*options.baseConfig, func(o *sts.Options) {
		o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
	})
	cred := stscreds.NewAssumeRoleProvider(stsClient, options.assumeRoleARN, func(aro *stscreds.AssumeRoleOptions) {
		if options.assumeRoleExternalID != "" {
			aro.ExternalID = aws.String(options.assumeRoleExternalID)
		}
	})
	if _, err := cred.Retrieve(ctx); err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	opts := buildConfigOptions(region, cred, options)
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	return cfg, trace.Wrap(err)
}
