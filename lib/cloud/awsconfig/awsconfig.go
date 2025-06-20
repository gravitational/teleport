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
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
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

// OIDCIntegrationClient is an interface that indicates which APIs are
// required to generate an AWS OIDC integration token.
type OIDCIntegrationClient interface {
	// GetIntegration returns the specified integration resource.
	GetIntegration(ctx context.Context, name string) (types.Integration, error)

	// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC
	// Integration action.
	GenerateAWSOIDCToken(ctx context.Context, integrationName string) (string, error)
}

// STSClient is a subset of the AWS STS API.
type STSClient interface {
	stscreds.AssumeRoleAPIClient
	stscreds.AssumeRoleWithWebIdentityAPIClient
}

// STSClientProviderFunc provides an AWS STS assume role API client.
type STSClientProviderFunc func(aws.Config) STSClient

// AssumeRole is an AWS role to assume, optionally with an external ID.
type AssumeRole struct {
	// RoleARN is the ARN of the role to assume.
	RoleARN string `json:"role_arn"`
	// ExternalID is an optional ID to include when assuming the role.
	ExternalID string `json:"external_id,omitempty"`
	// SessionName is an optional session name to use when assuming the role.
	SessionName string `json:"session_name,omitempty"`
	// Tags is a list of STS session tags to pass when assuming the role.
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_session-tags.html
	Tags map[string]string `json:"tags,omitempty"`
	// Duration is the expiry duration of the generated credentials. Empty
	// value will use the AWS SDK default expiration time.
	Duration time.Duration `json:"duration,omitempty"`
}

// Options is a struct of additional Options for assuming an AWS role
// when construction an underlying AWS config.
type Options struct {
	// AssumeRoles are AWS IAM roles that should be assumed one by one in order,
	// as a chain of assumed roles.
	AssumeRoles []AssumeRole
	// credentialsSource describes which source to use to fetch credentials.
	credentialsSource credentialsSource
	// integration is the name of the integration to be used to fetch the credentials.
	integration string
	// oidcIntegrationClient provides APIs to generate AWS OIDC tokens, which
	// can then be exchanged for IAM credentials.
	// Required if integration credentials are requested.
	oidcIntegrationClient OIDCIntegrationClient
	// customRetryer is a custom retryer to use for the config.
	customRetryer func() aws.Retryer
	// maxRetries is the maximum number of retries to use for the config.
	maxRetries *int
	// stsClientProvider sets the STS assume role client provider func.
	stsClientProvider STSClientProviderFunc
	// baseCredentials is the base config used to assume the roles.
	baseCredentials aws.CredentialsProvider
}

func buildOptions(optFns ...OptionsFn) (*Options, error) {
	var opts Options
	for _, optFn := range optFns {
		optFn(&opts)
	}
	if err := opts.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &opts, nil
}

func (o *Options) checkAndSetDefaults() error {
	if o.baseCredentials == nil {
		switch o.credentialsSource {
		case credentialsSourceAmbient:
			if o.integration != "" {
				return trace.BadParameter("integration and ambient credentials cannot be used at the same time")
			}
		case credentialsSourceIntegration:
			if o.integration == "" {
				return trace.BadParameter("missing integration name")
			}
			if o.oidcIntegrationClient == nil {
				return trace.BadParameter("missing AWS OIDC integration client")
			}
		default:
			return trace.BadParameter("missing credentials source (ambient or integration)")
		}
	}
	if len(o.AssumeRoles) > 2 {
		return trace.BadParameter("role chain contains more than 2 roles")
	}

	if o.stsClientProvider == nil {
		o.stsClientProvider = func(cfg aws.Config) STSClient {
			return stsutils.NewFromConfig(cfg, func(o *sts.Options) {
				o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
			})

		}
	}
	return nil
}

// OptionsFn is an option function for setting additional options
// when getting an AWS config.
type OptionsFn func(*Options)

// WithAssumeRole configures options needed for assuming an AWS role.
func WithAssumeRole(roleARN, externalID string) OptionsFn {
	return func(options *Options) {
		if roleARN == "" {
			// ignore empty role ARN for caller convenience.
			return
		}
		options.AssumeRoles = append(options.AssumeRoles, AssumeRole{
			RoleARN:    roleARN,
			ExternalID: externalID,
		})
	}
}

// WithDetailedAssumeRole configures options needed for assuming an AWS role,
// including optional details like session name, duration, and tags.
func WithDetailedAssumeRole(ar AssumeRole) OptionsFn {
	return func(options *Options) {
		if ar.RoleARN == "" {
			// ignore empty role ARN for caller convenience.
			return
		}
		options.AssumeRoles = append(options.AssumeRoles, ar)
	}
}

// WithRetryer sets a custom retryer for the config.
func WithRetryer(retryer func() aws.Retryer) OptionsFn {
	return func(options *Options) {
		options.customRetryer = retryer
	}
}

// WithMaxRetries sets the maximum allowed value for the sdk to keep retrying.
func WithMaxRetries(maxRetries int) OptionsFn {
	return func(options *Options) {
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
	return func(options *Options) {
		options.credentialsSource = credentialsSourceIntegration
		options.integration = integration
	}
}

// WithAmbientCredentials configures options to use the ambient credentials.
func WithAmbientCredentials() OptionsFn {
	return func(options *Options) {
		options.credentialsSource = credentialsSourceAmbient
	}
}

// WithSTSClientProvider sets the STS API client factory func.
func WithSTSClientProvider(fn STSClientProviderFunc) OptionsFn {
	return func(options *Options) {
		options.stsClientProvider = fn
	}
}

// WithOIDCIntegrationClient sets the OIDC integration client.
func WithOIDCIntegrationClient(c OIDCIntegrationClient) OptionsFn {
	return func(options *Options) {
		options.oidcIntegrationClient = c
	}
}

// WithBaseCredentialsProvider sets the base provider credentials used for the
// assumed roles.
func WithBaseCredentialsProvider(baseCredentialsProvider aws.CredentialsProvider) OptionsFn {
	return func(o *Options) {
		o.baseCredentials = baseCredentialsProvider
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
	return getConfigForRoleChain(ctx, cfg, opts.AssumeRoles, opts.stsClientProvider)
}

// loadDefaultConfig loads a new config.
func loadDefaultConfig(ctx context.Context, region string, cred aws.CredentialsProvider, opts *Options) (aws.Config, error) {
	configOpts := buildConfigOptions(region, cred, opts)
	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	return cfg, trace.Wrap(err)
}

func buildConfigOptions(region string, cred aws.CredentialsProvider, opts *Options) []func(*config.LoadOptions) error {
	configOpts := []func(*config.LoadOptions) error{
		config.WithDefaultRegion(defaultRegion),
		config.WithRegion(region),
		config.WithCredentialsProvider(cred),
		config.WithCredentialsCacheOptions(awsCredentialsCacheOptions),
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
func getBaseConfig(ctx context.Context, region string, opts *Options) (aws.Config, error) {
	if opts.baseCredentials != nil {
		return loadDefaultConfig(ctx, region, opts.baseCredentials, opts)
	}

	slog.DebugContext(ctx, "Initializing AWS config from default credential chain",
		"region", region,
	)
	cfg, err := loadDefaultConfig(ctx, region, nil, opts)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	if opts.credentialsSource == credentialsSourceIntegration {
		slog.DebugContext(ctx, "Initializing AWS config with OIDC integration credentials",
			"region", region,
			"integration", opts.integration,
		)
		provider := &integrationCredentialsProvider{
			OIDCIntegrationClient: opts.oidcIntegrationClient,
			stsClt:                opts.stsClientProvider(cfg),
			integrationName:       opts.integration,
		}
		cc := aws.NewCredentialsCache(provider, awsCredentialsCacheOptions)
		_, err := cc.Retrieve(ctx)
		if err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
		cfg.Credentials = cc
	}
	return cfg, nil
}

func getConfigForRoleChain(ctx context.Context, cfg aws.Config, roles []AssumeRole, newCltFn STSClientProviderFunc) (aws.Config, error) {
	if len(roles) > 0 {
		for _, r := range roles {
			cfg.Credentials = getAssumeRoleProvider(ctx, newCltFn(cfg), r)
		}
		// No point caching every assumed role in the chain, we can just cache
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
		aro.RoleSessionName = maybeHashRoleSessionName(role.SessionName)
		aro.Duration = role.Duration
		for k, v := range role.Tags {
			aro.Tags = append(aro.Tags, ststypes.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
	})
}

// staticIdentityToken provides itself as a JWT []byte token to implement
// [stscreds.IdentityTokenRetriever].
type staticIdentityToken string

// GetIdentityToken retrieves the JWT token.
func (t staticIdentityToken) GetIdentityToken() ([]byte, error) {
	return []byte(t), nil
}

// integrationCredentialsProvider provides AWS OIDC integration credentials.
type integrationCredentialsProvider struct {
	OIDCIntegrationClient
	stsClt          STSClient
	integrationName string
}

// Retrieve provides [aws.Credentials] for an AWS OIDC integration.
func (p *integrationCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	integration, err := p.GetIntegration(ctx, p.integrationName)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}
	spec := integration.GetAWSOIDCIntegrationSpec()
	if spec == nil {
		return aws.Credentials{}, trace.BadParameter("invalid integration subkind, expected awsoidc, got %s", integration.GetSubKind())
	}
	token, err := p.GenerateAWSOIDCToken(ctx, p.integrationName)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}
	cred, err := stscreds.NewWebIdentityRoleProvider(
		p.stsClt,
		spec.RoleARN,
		staticIdentityToken(token),
	).Retrieve(ctx)
	return cred, trace.Wrap(err)
}

// maybeHashRoleSessionName truncates the role session name and adds a hash
// when the original role session name is greater than AWS character limit
// (64).
func maybeHashRoleSessionName(roleSessionName string) (ret string) {
	// maxRoleSessionNameLength is the maximum length of the role session name
	// used by the AssumeRole call.
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_iam-quotas.html
	const maxRoleSessionNameLength = 64
	if len(roleSessionName) <= maxRoleSessionNameLength {
		return roleSessionName
	}

	const hashLen = 16
	hash := sha1.New()
	hash.Write([]byte(roleSessionName))
	hex := hex.EncodeToString(hash.Sum(nil))[:hashLen]

	// "1" for the delimiter.
	keepPrefixIndex := maxRoleSessionNameLength - len(hex) - 1

	// Sanity check. This should never happen since hash length and
	// MaxRoleSessionNameLength are both constant.
	if keepPrefixIndex < 0 {
		keepPrefixIndex = 0
	}

	ret = fmt.Sprintf("%s-%s", roleSessionName[:keepPrefixIndex], hex)
	slog.DebugContext(context.Background(),
		"AWS role session name is too long. Using a hash instead.",
		"hashed", ret,
		"original", roleSessionName,
	)
	return ret
}
