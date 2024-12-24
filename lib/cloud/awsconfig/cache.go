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
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// Cache is an AWS config [Provider] that caches credentials by integration and
// role.
type Cache struct {
	awsConfigCache *utils.FnCache
}

var _ Provider = (*Cache)(nil)

// NewCache returns a new [Cache].
func NewCache() (*Cache, error) {
	c, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         15 * time.Minute,
		ReloadOnErr: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Cache{
		awsConfigCache: c,
	}, nil
}

// GetConfig returns an [aws.Config] for the given region and options.
func (c *Cache) GetConfig(ctx context.Context, region string, optFns ...OptionsFn) (aws.Config, error) {
	opts, err := buildOptions(optFns...)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	cfg, err := c.getBaseConfig(ctx, region, opts)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}
	cfg, err = c.getConfigForRoleChain(ctx, cfg, opts)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}
	return cfg, nil
}

func (c *Cache) getBaseConfig(ctx context.Context, region string, opts *options) (aws.Config, error) {
	// The AWS SDK combines config loading with default credential chain
	// loading.
	// We cache the entire config by integration name, which is empty for
	// non-integration config, but only use credentials from it on cache hit.
	cacheKey := configCacheKey{
		integration: opts.integration,
	}
	var reloaded bool
	cfg, err := utils.FnCacheGet(ctx, c.awsConfigCache, cacheKey,
		func(ctx context.Context) (aws.Config, error) {
			reloaded = true
			cfg, err := getBaseConfig(ctx, region, opts)
			return cfg, trace.Wrap(err)
		})
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	if reloaded {
		// If the cache reload func was called, then the config we got back has
		// already applied our options so we can return the config itself.
		return cfg, nil
	}

	// On cache hit we just take the credentials from the cached config.
	// Then, we apply those credentials while loading config with current
	// options.
	cfg, err = loadDefaultConfig(ctx, region, cfg.Credentials, opts)
	return cfg, trace.Wrap(err)
}

func (c *Cache) getConfigForRoleChain(ctx context.Context, cfg aws.Config, opts *options) (aws.Config, error) {
	for i, r := range opts.assumeRoles {
		// cache credentials by integration and assumed-role chain.
		roleChain, err := getCacheKeyForRoles(opts.assumeRoles[:i+1])
		if err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
		cacheKey := configCacheKey{
			integration: opts.integration,
			roleChain:   roleChain,
		}
		credProvider, err := utils.FnCacheGet(ctx, c.awsConfigCache, cacheKey,
			func(ctx context.Context) (aws.CredentialsProvider, error) {
				clt := opts.assumeRoleClientProvider(cfg)
				credProvider := getAssumeRoleProvider(ctx, clt, r)
				return aws.NewCredentialsCache(credProvider,
					func(cacheOpts *aws.CredentialsCacheOptions) {
						// expire early to avoid expiration race.
						cacheOpts.ExpiryWindow = 5 * time.Minute
					},
				), nil
			})
		if err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
		cfg.Credentials = credProvider
	}
	if len(opts.assumeRoles) > 0 {
		if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
	}
	return cfg, nil
}

// configCacheKey defines the cache key used for AWS config.
// Config is cached by integration and AWS IAM role chain.
type configCacheKey struct {
	// integration is the name of an AWS integration.
	integration string
	// roleChain is the AWS IAM role chain as a string of roles.
	roleChain string
}

// getCacheKeyForRoles makes a cache key for roles.
// cache key format: role1|ext1|role2|ext2|...
func getCacheKeyForRoles(roles []assumeRole) (string, error) {
	// The cache key can be used to get role credentials without calling AWS
	// STS.
	// Therefore, we should be paranoid and do some validation here to be sure
	// that the cache cannot be exploited.
	// Neither role ARN nor external ID can contain this delimiter:
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_iam-quotas.html
	const delimiter = "|"
	var sb strings.Builder
	for _, r := range roles {
		if strings.Contains(r.roleARN, delimiter) {
			return "", trace.BadParameter("invalid role ARN %s", r.roleARN)
		}
		if strings.Contains(r.externalID, delimiter) {
			return "", trace.BadParameter("invalid external ID %s", r.externalID)
		}
		sb.WriteString(r.roleARN)
		sb.WriteString(delimiter)
		sb.WriteString(r.externalID)
		sb.WriteString(delimiter)
	}
	return sb.String(), nil
}
