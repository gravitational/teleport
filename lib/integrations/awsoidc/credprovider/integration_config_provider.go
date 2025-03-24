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

package credprovider

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

// Options represents additional options for configuring the AWS credentials provider.
// There are currently no options but this type is still referenced from
// teleport.e.
type Options struct{}

// Option is a function that modifies the Options struct for the AWS configuration.
type Option func(*Options)

// CreateAWSConfigForIntegration returns a new AWS credentials provider that
// uses the AWS OIDC integration to generate temporary credentials.
// The provider will periodically refresh the credentials before they expire.
func CreateAWSConfigForIntegration(ctx context.Context, config Config, option ...Option) (*aws.Config, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cacheAWSConfig, err := newAWSConfig(ctx, config.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.STSClient == nil {
		config.STSClient = stsutils.NewFromConfig(*cacheAWSConfig)
	}
	credCache, err := newAWSCredCache(ctx, config, config.STSClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go credCache.Run(ctx)

	awsCfg, err := newAWSConfig(ctx, config.Region, awsConfig.WithCredentialsProvider(credCache))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return awsCfg, nil
}

// Config is a configuration struct for creating a new
// AWS credentials provider that uses the AWS OIDC integration to generate
// temporary credentials.
type Config struct {
	// Region is the AWS region to use for the STS client.
	Region string
	// IntegrationName is the name of the AWS OIDC integration to use.
	IntegrationName string
	// IntegrationGetter is used to fetch the AWS OIDC integration.
	IntegrationGetter integrationGetter
	// AWSOIDCTokenGenerator is used to generate OIDC tokens for the AWS integration.
	AWSOIDCTokenGenerator tokenGenerator
	// STSClient is the AWS Security Token Service client.
	STSClient stscreds.AssumeRoleWithWebIdentityAPIClient
	// Logger is the logger to use for logging.
	Logger *slog.Logger
	// Clock is the clock to use for timekeeping.
	Clock clockwork.Clock
}

type integrationGetter interface {
	// GetIntegration returns an integration by name from the backend.
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
}

type tokenGenerator interface {
	// GenerateAWSOIDCToken generates an OIDC token for the given integration.
	// The token is used to authenticate to AWS via OIDC.
	GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error)
}

func (c *Config) checkAndSetDefaults() error {
	if c.Region == "" {
		return trace.BadParameter("missing region")
	}
	if c.IntegrationName == "" {
		return trace.BadParameter("missing integration name")
	}
	if c.IntegrationGetter == nil {
		return trace.BadParameter("missing integration getter")
	}
	if c.AWSOIDCTokenGenerator == nil {
		return trace.BadParameter("missing token generator")
	}
	if c.Logger == nil {
		c.Logger = slog.Default().With(teleport.ComponentKey, "AWS_OIDC_CONFIG_PROVIDER")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

func newAWSCredCache(ctx context.Context, cfg Config, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (*CredentialsCache, error) {
	integration, err := cfg.IntegrationGetter.GetIntegration(ctx, cfg.IntegrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleARN, err := arn.Parse(integration.GetAWSOIDCIntegrationSpec().RoleARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	credCache, err := NewCredentialsCache(
		CredentialsCacheOptions{
			Log:                 cfg.Logger,
			Clock:               cfg.Clock,
			STSClient:           stsClient,
			RoleARN:             roleARN,
			Integration:         cfg.IntegrationName,
			GenerateOIDCTokenFn: cfg.AWSOIDCTokenGenerator.GenerateAWSOIDCToken,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating OIDC credentials cache")
	}
	return credCache, nil
}

func newAWSConfig(ctx context.Context, awsRegion string, options ...func(*awsConfig.LoadOptions) error) (*aws.Config, error) {
	var useFIPS aws.FIPSEndpointState
	if modules.GetModules().IsBoringBinary() {
		useFIPS = aws.FIPSEndpointStateEnabled
	}
	options = append(options,
		awsConfig.WithRegion(awsRegion),
		awsConfig.WithUseFIPSEndpoint(useFIPS),
		awsConfig.WithRetryMaxAttempts(10),
	)
	cfg, err := awsConfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &cfg, nil
}
