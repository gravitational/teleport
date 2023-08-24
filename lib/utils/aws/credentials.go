/*
Copyright 2023 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/utils"
)

// GetCredentialsRequest is the request for obtaining STS credentials.
type GetCredentialsRequest struct {
	// Provider is the user session used to create the STS client.
	Provider client.ConfigProvider
	// Expiry is session expiry to be requested.
	Expiry time.Time
	// SessionName is the session name to be requested.
	SessionName string
	// RoleARN is the role ARN to be requested.
	RoleARN string
	// ExternalID is the external ID to be requested, if not empty.
	ExternalID string
}

// CredentialsGetter defines an interface for obtaining STS credentials.
type CredentialsGetter interface {
	// Get obtains STS credentials.
	Get(ctx context.Context, request GetCredentialsRequest) (*credentials.Credentials, error)
}

type credentialsGetter struct {
}

// NewCredentialsGetter returns a new CredentialsGetter.
func NewCredentialsGetter() CredentialsGetter {
	return &credentialsGetter{}
}

// Get obtains STS credentials.
func (g *credentialsGetter) Get(_ context.Context, request GetCredentialsRequest) (*credentials.Credentials, error) {
	logrus.Debugf("Creating STS session %q for %q.", request.SessionName, request.RoleARN)
	return stscreds.NewCredentials(request.Provider, request.RoleARN,
		func(cred *stscreds.AssumeRoleProvider) {
			cred.RoleSessionName = request.SessionName
			cred.Expiry.SetExpiration(request.Expiry, 0)

			if request.ExternalID != "" {
				cred.ExternalID = aws.String(request.ExternalID)
			}
		},
	), nil
}

// CachedCredentialsGetterConfig is the config for creating a CredentialsGetter that caches credentials.
type CachedCredentialsGetterConfig struct {
	// Getter is the CredentialsGetter for obtaining the STS credentials.
	Getter CredentialsGetter
	// CacheTTL is the cache TTL.
	CacheTTL time.Duration
	// Clock is used to control time.
	Clock clockwork.Clock
}

// SetDefaults sets default values for CachedCredentialsGetterConfig.
func (c *CachedCredentialsGetterConfig) SetDefaults() {
	if c.Getter == nil {
		c.Getter = NewCredentialsGetter()
	}
	if c.CacheTTL <= 0 {
		c.CacheTTL = time.Minute
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
}

type cachedCredentialsGetter struct {
	config CachedCredentialsGetterConfig
	cache  *utils.FnCache
}

// NewCachedCredentialsGetter returns a CredentialsGetter that caches credentials.
func NewCachedCredentialsGetter(config CachedCredentialsGetterConfig) (CredentialsGetter, error) {
	config.SetDefaults()

	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   config.CacheTTL,
		Clock: config.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &cachedCredentialsGetter{
		config: config,
		cache:  cache,
	}, nil
}

// Get returns cached credentials if found, or fetch it from the configured
// getter.
func (g *cachedCredentialsGetter) Get(ctx context.Context, request GetCredentialsRequest) (*credentials.Credentials, error) {
	credentials, err := utils.FnCacheGet(ctx, g.cache, request, func(ctx context.Context) (*credentials.Credentials, error) {
		credentials, err := g.config.Getter.Get(ctx, request)
		return credentials, trace.Wrap(err)
	})
	return credentials, trace.Wrap(err)
}

type staticCredentialsGetter struct {
	credentials *credentials.Credentials
}

// NewStaticCredentialsGetter returns a CredentialsGetter that always returns
// the same provided credentials.
//
// Used in testing to mock CredentialsGetter.
func NewStaticCredentialsGetter(credentials *credentials.Credentials) CredentialsGetter {
	return &staticCredentialsGetter{
		credentials: credentials,
	}
}

// Get returns the credentials provided to NewStaticCredentialsGetter.
func (g *staticCredentialsGetter) Get(_ context.Context, _ GetCredentialsRequest) (*credentials.Credentials, error) {
	if g.credentials == nil {
		return nil, trace.NotFound("no credentials found")
	}
	return g.credentials, nil
}
