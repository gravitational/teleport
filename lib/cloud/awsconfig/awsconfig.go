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

// AssumeRoleClientProviderFunc provides an AWS STS assume role API client.
type AssumeRoleClientProviderFunc func(aws.Config) stscreds.AssumeRoleAPIClient

// AssumeRole is an AWS role to assume, optionally with an external ID.
type AssumeRole struct {
	// RoleARN is the ARN of the role to assume.
	RoleARN string `json:"role_arn"`
	// ExternalID is an optional ID to include when assuming the role.
	ExternalID string `json:"external_id"`
}

// options is a struct of additional options for assuming an AWS role
// when construction an underlying AWS config.
type options struct {
	// assumeRoles are AWS IAM roles that should be assumed one by one in order,
	// as a chain of assumed roles.
	assumeRoles []AssumeRole
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
	// assumeRoleClientProvider sets the STS assume role client provider func.
	assumeRoleClientProvider AssumeRoleClientProviderFunc
}

func buildOptions(optFns ...OptionsFn) (*options, error) {
	var opts options
	for _, optFn := range optFns {
		optFn(&opts)
	}
	if err := opts.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &opts, nil
}

func (o *options) checkAndSetDefaults() error {
	switch o.credentialsSource {
	case credentialsSourceAmbient:
		if o.integration != "" {
			return trace.BadParameter("integration and ambient credentials cannot be used at the same time")
		}
	case credentialsSourceIntegration:
		if o.integration == "" {
			return trace.BadParameter("missing integration name")
		}
	default:
		return trace.BadParameter("missing credentials source (ambient or integration)")
	}
	if len(o.assumeRoles) > 2 {
		return trace.BadParameter("role chain contains more than 2 roles")
	}

	if o.assumeRoleClientProvider == nil {
		o.assumeRoleClientProvider = func(cfg aws.Config) stscreds.AssumeRoleAPIClient {
			return sts.NewFromConfig(cfg, func(o *sts.Options) {
				o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
			})
		}
	}
	return nil
}

// OptionsFn is an option function for setting additional options
// when getting an AWS config.
type OptionsFn func(*options)

// WithAssumeRole configures options needed for assuming an AWS role.
func WithAssumeRole(roleARN, externalID string) OptionsFn {
	return func(options *options) {
		if roleARN == "" {
			// ignore empty role ARN for caller convenience.
			return
		}
		options.assumeRoles = append(options.assumeRoles, AssumeRole{
			RoleARN:    roleARN,
			ExternalID: externalID,
		})
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

// WithAssumeRoleClientProviderFunc sets the STS API client factory func used to
// assume roles.
func WithAssumeRoleClientProviderFunc(fn AssumeRoleClientProviderFunc) OptionsFn {
	return func(options *options) {
		options.assumeRoleClientProvider = fn
	}
}

// GetConfig returns an AWS config for the specified region, optionally
// assuming AWS IAM Roles.
func GetConfig(ctx context.Context, region string, optFns ...OptionsFn) (aws.Config, error) {
	opts, err := buildOptions(optFns...)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	cfg, err := getBaseConfig(ctx, region, opts)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}
	return getConfigForRoleChain(ctx, cfg, opts.assumeRoles, opts.assumeRoleClientProvider)
}

// loadDefaultConfig loads a new config.
func loadDefaultConfig(ctx context.Context, region string, cred aws.CredentialsProvider, opts *options) (aws.Config, error) {
	configOpts := buildConfigOptions(region, cred, opts)
	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	return cfg, trace.Wrap(err)
}

func buildConfigOptions(region string, cred aws.CredentialsProvider, opts *options) []func(*config.LoadOptions) error {
	configOpts := []func(*config.LoadOptions) error{
		config.WithDefaultRegion(defaultRegion),
		config.WithRegion(region),
		config.WithCredentialsProvider(cred),
	}
	if modules.GetModules().IsBoringBinary() {
		configOpts = append(configOpts, config.WithUseFIPSEndpoint(aws.FIPSEndpointStateEnabled))
	}
	if opts.customRetryer != nil {
		configOpts = append(configOpts, config.WithRetryer(opts.customRetryer))
	}
	if opts.maxRetries != nil {
		configOpts = append(configOpts, config.WithRetryMaxAttempts(*opts.maxRetries))
	}
	return configOpts
}

// getBaseConfig returns an AWS config without assuming any roles.
func getBaseConfig(ctx context.Context, region string, opts *options) (aws.Config, error) {
	var cred aws.CredentialsProvider
	if opts.credentialsSource == credentialsSourceIntegration {
		if opts.integrationCredentialsProvider == nil {
			return aws.Config{}, trace.BadParameter("missing aws integration credential provider")
		}

		slog.DebugContext(ctx, "Initializing AWS config with integration", "region", region, "integration", opts.integration)
		var err error
		cred, err = opts.integrationCredentialsProvider(ctx, region, opts.integration)
		if err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
	} else {
		slog.DebugContext(ctx, "Initializing AWS config from default credential chain", "region", region)
	}

	cfg, err := loadDefaultConfig(ctx, region, cred, opts)
	return cfg, trace.Wrap(err)
}

func getConfigForRoleChain(ctx context.Context, cfg aws.Config, roles []AssumeRole, newCltFn AssumeRoleClientProviderFunc) (aws.Config, error) {
	for _, r := range roles {
		cfg.Credentials = getAssumeRoleProvider(ctx, newCltFn(cfg), r)
	}
	if len(roles) > 0 {
		// no point caching every assumed role in the chain, we can just cache
		// the last one.
		cfg.Credentials = aws.NewCredentialsCache(cfg.Credentials, awsCredentialsCacheOptions)
		if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
	}
	return cfg, nil
}

func getAssumeRoleProvider(ctx context.Context, clt stscreds.AssumeRoleAPIClient, role AssumeRole) aws.CredentialsProvider {
	slog.DebugContext(ctx, "Initializing AWS session for assumed role",
		"assumed_role", role.RoleARN,
	)
	return stscreds.NewAssumeRoleProvider(clt, role.RoleARN, func(aro *stscreds.AssumeRoleOptions) {
		if role.ExternalID != "" {
			aro.ExternalID = aws.String(role.ExternalID)
		}
	})
}
