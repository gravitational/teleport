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
	"google.golang.org/protobuf/types/known/durationpb"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsra"
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

// IntegrationGetter is an interface that indicates which APIs are
// required to get an integration.
// Required when using integration credentials.
type IntegrationGetter interface {
	// GetIntegration returns the specified integration resource.
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
}

// OIDCIntegrationClient is an interface that indicates which APIs are
// required to generate an AWS OIDC integration token.
type OIDCIntegrationClient interface {
	IntegrationGetter
	// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC
	// Integration action.
	GenerateAWSOIDCToken(ctx context.Context, integrationName string) (string, error)
}

// RolesAnywhereIntegrationClient is an interface that indicates which APIs are
// required to generate a set of AWS credentials using the AWS IAM Roles Anywhere integration.
type RolesAnywhereIntegrationClient interface {
	IntegrationGetter
	// GenerateAWSRACredentials generates a token to be used to execute an AWS IAM Roles Anywhere integration.
	GenerateAWSRACredentials(ctx context.Context, req *integrationpb.GenerateAWSRACredentialsRequest) (*integrationpb.GenerateAWSRACredentialsResponse, error)
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
	// integrationGetter provides APIs to get the AWS integration.
	// Required if integration credentials are requested.
	integrationGetter IntegrationGetter

	// oidcIntegrationClient provides APIs to generate AWS OIDC tokens, which
	// can then be exchanged for IAM credentials.
	// Required when integration uses IAM OIDC IdP to obtain credentials.
	oidcIntegrationClient OIDCIntegrationClient

	// rolesAnywhereIntegrationClient provides APIs to generate AWS credentials.
	// Required when integration uses IAM Roles Anywhere service to obtain credentials.
	rolesAnywhereIntegrationClient RolesAnywhereIntegrationClient
	// rolesAnywhereIntegrationMetadata contains the Roles Anywhere Profile and IAM Role to use.
	rolesAnywhereIntegrationMetadata RolesAnywhereMetadata

	// customRetryer is a custom retryer to use for the config.
	customRetryer func() aws.Retryer
	// maxRetries is the maximum number of retries to use for the config.
	maxRetries *int
	// stsClientProvider sets the STS assume role client provider func.
	stsClientProvider STSClientProviderFunc
	// baseCredentials is the base config used to assume the roles.
	baseCredentials aws.CredentialsProvider
	// withFallbackRegionResolver is a fallback region resolver func that is
	// called if a config does not resolve a region. If the resolver returns an
	// error, then the default region (us-east-1) will be used.
	withFallbackRegionResolver func(ctx context.Context) (string, error)
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
	if o.baseCredentials == nil {
		switch o.credentialsSource {
		case credentialsSourceAmbient:
			if o.integration != "" {
				return trace.BadParameter("integration and ambient credentials cannot be used at the same time")
			}
		case credentialsSourceIntegration:
			if err := o.checkIntegrationCredentials(); err != nil {
				return trace.Wrap(err)
			}
		default:
			return trace.BadParameter("missing credentials source (ambient or integration)")
		}
	}
	if len(o.assumeRoles) > 2 {
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

func (o *options) checkIntegrationCredentials() error {
	if o.integration == "" {
		return trace.BadParameter("missing integration name")
	}

	if o.integrationGetter == nil {
		return trace.BadParameter("missing integration getter")
	}

	if o.oidcIntegrationClient == nil && o.rolesAnywhereIntegrationClient == nil {
		return trace.BadParameter("missing AWS integration client")
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

// WithDetailedAssumeRole configures options needed for assuming an AWS role,
// including optional details like session name, duration, and tags.
func WithDetailedAssumeRole(ar AssumeRole) OptionsFn {
	return func(options *options) {
		if ar.RoleARN == "" {
			// ignore empty role ARN for caller convenience.
			return
		}
		options.assumeRoles = append(options.assumeRoles, ar)
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

// IntegrationMetadata contains the metadata about the Integration to use
// when using the integration credentials source.
type IntegrationMetadata struct {
	// Name of the integration.
	// Will be empty when using ambient credentials.
	Name string

	// RolesAnywhereMetadata contains the metadata about the Roles Anywhere.
	// Only set when the Integration is of AWS IAM Roles Anywhere subkind.
	RolesAnywhereMetadata RolesAnywhereMetadata
}

// RolesAnywhereMetadata contains the metadata required to use AWS IAM Roles Anywhere
// to generate credentials.
type RolesAnywhereMetadata struct {
	// ProfileARN is the ARN of the Roles Anywhere profile.
	ProfileARN string
	// ProfileAcceptsRoleSessionName indicates whether the profile accepts a role session name.
	ProfileAcceptsRoleSessionName bool
	// RoleARN is the ARN of the role to assume.
	RoleARN string
	// IdentityUsername is the username to use when generating the AWS credentials.
	// This will be used as the Subject Common Name (CN) in the certificate, and logged in CloudTrail if ProfileAcceptsRoleSessionName is true.
	// Should be set to the teleport's username.
	IdentityUsername string
	// SessionDuration is used to calculate the expiration time for the AWS session.
	// Must be lower or equal to the maximum session duration of the role.
	// The actual session duration will be the minimum between this value (if not zero) and the Profile's max session duration.
	SessionDuration time.Duration
}

// WithCredentialsMaybeIntegration sets the credential source to be
// - ambient if the integration is an empty string
// - integration, otherwise
// When using integration, relevant integration metadata must be provided.
func WithCredentialsMaybeIntegration(integrationMetadata IntegrationMetadata) OptionsFn {
	if integrationMetadata.Name == "" {
		return WithAmbientCredentials()
	}

	return func(options *options) {
		options.credentialsSource = credentialsSourceIntegration
		options.integration = integrationMetadata.Name
		options.rolesAnywhereIntegrationMetadata = integrationMetadata.RolesAnywhereMetadata
	}
}

// WithRolesAnywhereIntegrationClient sets the Roles Anywhere integration client.
func WithRolesAnywhereIntegrationClient(c RolesAnywhereIntegrationClient) OptionsFn {
	return func(options *options) {
		options.rolesAnywhereIntegrationClient = c
		options.integrationGetter = c
	}
}

// WithAmbientCredentials configures options to use the ambient credentials.
func WithAmbientCredentials() OptionsFn {
	return func(options *options) {
		options.credentialsSource = credentialsSourceAmbient
	}
}

// WithSTSClientProvider sets the STS API client factory func.
func WithSTSClientProvider(fn STSClientProviderFunc) OptionsFn {
	return func(options *options) {
		options.stsClientProvider = fn
	}
}

// WithOIDCIntegrationClient sets the OIDC integration client.
func WithOIDCIntegrationClient(c OIDCIntegrationClient) OptionsFn {
	return func(options *options) {
		options.oidcIntegrationClient = c
		options.integrationGetter = c
	}
}

// WithBaseCredentialsProvider sets the base provider credentials used for the
// assumed roles.
func WithBaseCredentialsProvider(baseCredentialsProvider aws.CredentialsProvider) OptionsFn {
	return func(o *options) {
		o.baseCredentials = baseCredentialsProvider
	}
}

// WithFallbackRegionResolver sets a fallback region resolver func that is
// called if a config does not resolve a region. If the resolver returns an
// error, then the default region (us-east-1) will be used.
func WithFallbackRegionResolver(fn func(ctx context.Context) (string, error)) OptionsFn {
	return func(o *options) {
		o.withFallbackRegionResolver = fn
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
	return getConfigForRoleChain(ctx, cfg, opts.assumeRoles, opts.stsClientProvider)
}

// loadDefaultConfig loads a new config.
func loadDefaultConfig(ctx context.Context, region string, cred aws.CredentialsProvider, opts *options) (aws.Config, error) {
	configOpts := buildConfigOptions(region, cred, opts)
	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}
	if len(cfg.Region) == 0 && opts.withFallbackRegionResolver != nil {
		region, err := opts.withFallbackRegionResolver(ctx)
		if err == nil {
			cfg.Region = region
		} else {
			slog.DebugContext(ctx, "fallback region resolver failed, using the default region",
				"default_region", defaultRegion,
				"error", err,
			)
			cfg.Region = defaultRegion
		}
	}
	return cfg, nil
}

func buildConfigOptions(region string, cred aws.CredentialsProvider, opts *options) []func(*config.LoadOptions) error {
	configOpts := []func(*config.LoadOptions) error{
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
	if opts.withFallbackRegionResolver == nil {
		// if we pass WithDefaultRegion, then we will never get back an empty
		// region, so we have to skip passing it here if we want to make use of
		// a custom fallback resolver.
		configOpts = append(configOpts, config.WithDefaultRegion(defaultRegion))
	}
	return configOpts
}

// getBaseConfig returns an AWS config without assuming any roles.
func getBaseConfig(ctx context.Context, region string, opts *options) (aws.Config, error) {
	if opts.baseCredentials != nil {
		return loadDefaultConfig(ctx, region, opts.baseCredentials, opts)
	}

	slog.DebugContext(ctx, "Initializing AWS config from default credential chain")
	cfg, err := loadDefaultConfig(ctx, region, nil, opts)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Loaded AWS config from default credential chain",
		"region", cfg.Region,
	)

	if opts.credentialsSource == credentialsSourceIntegration {
		slog.DebugContext(ctx, "Initializing AWS config with integration credentials",
			"region", region,
			"integration", opts.integration,
		)
		provider := &integrationCredentialsProvider{
			stsClt:                         opts.stsClientProvider(cfg),
			integrationName:                opts.integration,
			integrationGetter:              opts.integrationGetter,
			oidcIntegrationClient:          opts.oidcIntegrationClient,
			rolesAnywhereIntegrationClient: opts.rolesAnywhereIntegrationClient,
			rolesAnywhereProfileMetadata:   opts.rolesAnywhereIntegrationMetadata,
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

// integrationCredentialsProvider provides AWS integration credentials.
type integrationCredentialsProvider struct {
	stsClt          STSClient
	integrationName string

	integrationGetter IntegrationGetter

	oidcIntegrationClient OIDCIntegrationClient

	rolesAnywhereIntegrationClient RolesAnywhereIntegrationClient
	rolesAnywhereProfileMetadata   RolesAnywhereMetadata
}

// Retrieve provides [aws.Credentials] for an AWS integration.
func (p *integrationCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	integration, err := p.integrationGetter.GetIntegration(ctx, p.integrationName)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}

	switch integration.GetSubKind() {
	case types.IntegrationSubKindAWSOIDC:
		if p.oidcIntegrationClient == nil {
			return aws.Credentials{}, trace.BadParameter("missing OIDC integration client")
		}

		spec := integration.GetAWSOIDCIntegrationSpec()
		if spec == nil {
			return aws.Credentials{}, trace.BadParameter("invalid integration subkind, expected awsoidc, got %s", integration.GetSubKind())
		}
		token, err := p.oidcIntegrationClient.GenerateAWSOIDCToken(ctx, p.integrationName)
		if err != nil {
			return aws.Credentials{}, trace.Wrap(err)
		}
		cred, err := stscreds.NewWebIdentityRoleProvider(
			p.stsClt,
			spec.RoleARN,
			staticIdentityToken(token),
		).Retrieve(ctx)
		return cred, trace.Wrap(err)

	case types.IntegrationSubKindAWSRolesAnywhere:
		if p.rolesAnywhereIntegrationClient == nil {
			return aws.Credentials{}, trace.BadParameter("missing roles anywhere integration client")
		}

		resp, err := p.rolesAnywhereIntegrationClient.GenerateAWSRACredentials(ctx, &integrationpb.GenerateAWSRACredentialsRequest{
			Integration:                   p.integrationName,
			ProfileArn:                    p.rolesAnywhereProfileMetadata.ProfileARN,
			ProfileAcceptsRoleSessionName: p.rolesAnywhereProfileMetadata.ProfileAcceptsRoleSessionName,
			RoleArn:                       p.rolesAnywhereProfileMetadata.RoleARN,
			SubjectName:                   p.rolesAnywhereProfileMetadata.IdentityUsername,
			SessionMaxDuration:            durationpb.New(p.rolesAnywhereProfileMetadata.SessionDuration),
		})
		if err != nil {
			return aws.Credentials{}, trace.Wrap(err)
		}

		return aws.Credentials{
			AccessKeyID:     resp.AccessKeyId,
			SecretAccessKey: resp.SecretAccessKey,
			SessionToken:    resp.SessionToken,
			Expires:         resp.Expiration.AsTime(),
			Source:          awsra.AWSCredentialsSourceRolesAnywhere,
		}, nil

	default:
		return aws.Credentials{}, trace.BadParameter("invalid integration subkind, expected AWS OIDC or AWS Roles Anywhere, got %s", integration.GetSubKind())
	}
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
