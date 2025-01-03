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
	"encoding/json"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

func awsCredentialsCacheOptions(opts *aws.CredentialsCacheOptions) {
	// expire early to avoid expiration race.
	opts.ExpiryWindow = 2 * time.Minute
}

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
	cacheKey, err := newCacheKey(opts.integration)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
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
		cacheKey, err := newCacheKey(opts.integration, opts.assumeRoles[:i+1]...)
		if err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
		credProvider, err := utils.FnCacheGet(ctx, c.awsConfigCache, cacheKey,
			func(ctx context.Context) (aws.CredentialsProvider, error) {
				clt := opts.assumeRoleClientProvider(cfg)
				credProvider := getAssumeRoleProvider(ctx, clt, r)
				cc := aws.NewCredentialsCache(credProvider,
					awsCredentialsCacheOptions,
				)
				if _, err := cc.Retrieve(ctx); err != nil {
					return nil, trace.Wrap(err)
				}
				return cc, nil
			})
		if err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
		cfg.Credentials = credProvider
	}
	return cfg, nil
}

// newCacheKey returns a cache key for AWS credentials.
// The cache key can be used to get role credentials without calling AWS STS.
// Therefore, we marshal the key as JSON to be sure the input cannot be
// manipulated to retrieve other credentials.
func newCacheKey(integrationName string, roleChain ...AssumeRole) (string, error) {
	type configCacheKey struct {
		Integration string       `json:"integration"`
		RoleChain   []AssumeRole `json:"role_chain"`
	}
	out, err := json.Marshal(configCacheKey{
		Integration: integrationName,
		RoleChain:   roleChain,
	})
	return string(out), trace.Wrap(err)
}
