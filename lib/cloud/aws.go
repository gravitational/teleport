/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package cloud

import (
	"context"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/gravitational/trace"

	cloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
)

// AWSV2Clients is an interface for providing AWS API clients.
type AWSClientsV2 interface {
	// GetAWSConfigV2 returns AWS config for the specified region, optionally
	// assuming AWS IAM Roles.
	GetAWSConfigV2(ctx context.Context, region string, opts ...AWSOptionsFn) (*aws.Config, error)
	// GetAWSSTSClientV2 returns AWS STS client for the specified region.
	GetAWSSTSClientV2(ctx context.Context, region string, opts ...AWSOptionsFn) (cloudaws.STSAPI, error)
}

type awsConfigCacheKey struct {
	region      string
	integration string
	roleARN     string
	externalID  string
}

type awsClientsV2 struct {
	// configsCache is a cache of AWS configs, where the cache key is an
	// instance of awsConfigCacheKey.
	configsCache *utils.FnCache
}

func newAWSClientsV2() (*awsClientsV2, error) {
	configsCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL: 15 * time.Minute,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &awsClientsV2{
		configsCache: configsCache,
	}, nil
}

// GetAWSConfigV2 returns AWS config for the specified region, optionally
// assuming AWS IAM Roles.
func (c *awsClientsV2) GetAWSConfigV2(ctx context.Context, region string, opts ...AWSOptionsFn) (*aws.Config, error) {
	var options awsOptions
	for _, opt := range opts {
		opt(&options)
	}
	var err error
	if options.baseConfigV2 == nil {
		options.baseConfigV2, err = c.getAWSConfigV2ForRegion(ctx, region, options)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if options.assumeRoleARN == "" {
		return options.baseConfigV2, nil
	}
	return c.getAWSConfigV2ForRole(ctx, region, options)
}

// GetAWSSTSClientV2 returns AWS STS client for the specified region.
func (c *awsClientsV2) GetAWSSTSClientV2(ctx context.Context, region string, opts ...AWSOptionsFn) (cloudaws.STSAPI, error) {
	config, err := c.GetAWSConfigV2(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cloudaws.NewSTSClient(*config), nil
}

// getAWSConfigV2ForRegion returns AWS config for the specified region.
func (c *awsClientsV2) getAWSConfigV2ForRegion(ctx context.Context, region string, opts awsOptions) (*aws.Config, error) {
	if err := opts.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cacheKey := awsConfigCacheKey{
		region:      region,
		integration: opts.integration,
	}

	config, err := utils.FnCacheGet(ctx, c.configsCache, cacheKey, func(ctx context.Context) (*aws.Config, error) {
		// TODO support integration.
		if opts.credentialsSource == credentialsSourceIntegration {
			return nil, trace.NotImplemented("missing aws integration config provider")
		}

		slog.DebugContext(ctx, "Initializing AWS config using ambient credentials.", "region", region)
		config, err := awsAmbientConfigV2Provider(ctx, region, nil /*credProvider*/)
		return config, trace.Wrap(err)
	})
	// TODO handle opts.customRetryer and opts.maxRetries
	return config, trace.Wrap(err)
}

// getAWSConfigV2ForRole returns AWS config for the specified region and role.
func (c *awsClientsV2) getAWSConfigV2ForRole(ctx context.Context, region string, options awsOptions) (*aws.Config, error) {
	if err := options.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if options.baseConfigV2 == nil {
		return nil, trace.BadParameter("missing base config")
	}

	cacheKey := awsConfigCacheKey{
		region:      region,
		integration: options.integration,
		roleARN:     options.assumeRoleARN,
		externalID:  options.assumeRoleExternalID,
	}
	return utils.FnCacheGet(ctx, c.configsCache, cacheKey, func(ctx context.Context) (*aws.Config, error) {
		config, err := newAWSConfigForRole(ctx, cloudaws.NewSTSClient(*options.baseConfigV2), region, options)
		return config, trace.Wrap(err)
	})
}

func newAWSConfigForRole(ctx context.Context, client cloudaws.STSAPI, region string, options awsOptions) (*aws.Config, error) {
	provider := stscreds.NewAssumeRoleProvider(client, options.assumeRoleARN, func(o *stscreds.AssumeRoleOptions) {
		if options.assumeRoleExternalID != "" {
			o.ExternalID = aws.String(options.assumeRoleExternalID)
		}
	})

	if _, err := provider.Retrieve(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := awsAmbientConfigV2Provider(ctx, region, provider)
	return config, trace.Wrap(err)
}

// awsAmbientConfigV2Provider loads a new config using the environment variables.
func awsAmbientConfigV2Provider(ctx context.Context, region string, credProvider aws.CredentialsProvider) (*aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsConfigFipsOption(),
	}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	if credProvider != nil {
		opts = append(opts, awsconfig.WithCredentialsProvider(credProvider))
	}
	config, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &config, nil
}

func awsConfigFipsOption() awsconfig.LoadOptionsFunc {
	if modules.GetModules().IsBoringBinary() {
		return awsconfig.WithUseFIPSEndpoint(aws.FIPSEndpointStateEnabled)
	}
	return awsconfig.WithUseFIPSEndpoint(aws.FIPSEndpointStateUnset)
}
