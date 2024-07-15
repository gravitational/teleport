/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package aws

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/modules"
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
	// Tags is a list of AWS STS session tags.
	Tags map[string]string
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

			cred.Tags = make([]*sts.Tag, 0, len(request.Tags))
			for key, value := range request.Tags {
				cred.Tags = append(cred.Tags, &sts.Tag{Key: aws.String(key), Value: aws.String(value)})
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

// credentialRequestCacheKey credentials request cache key.
type credentialRequestCacheKey struct {
	provider    client.ConfigProvider
	expiry      time.Time
	sessionName string
	roleARN     string
	externalID  string
	tags        string
}

// newCredentialRequestCacheKey creates a new cache key for the credentials
// request.
func newCredentialRequestCacheKey(req GetCredentialsRequest) credentialRequestCacheKey {
	k := credentialRequestCacheKey{
		provider:    req.Provider,
		expiry:      req.Expiry,
		sessionName: req.SessionName,
		roleARN:     req.RoleARN,
		externalID:  req.ExternalID,
	}

	tags := make([]string, 0, len(req.Tags))
	for key, value := range req.Tags {
		tags = append(tags, key+"="+value+",")
	}
	sort.Strings(tags)
	k.tags = strings.Join(tags, ",")

	return k
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
	credentials, err := utils.FnCacheGet(ctx, g.cache, newCredentialRequestCacheKey(request), func(ctx context.Context) (*credentials.Credentials, error) {
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

// AWSSessionProvider defines a function that creates an AWS Session.
// It must use ambient credentials if Integration is empty.
// It must use Integration credentials otherwise.
type AWSSessionProvider func(ctx context.Context, region string, integration string) (*session.Session, error)

// StaticAWSSessionProvider is a helper method that returns a static session.
// Must not be used to provide sessions when using Integrations.
func StaticAWSSessionProvider(awsSession *session.Session) AWSSessionProvider {
	return func(ctx context.Context, region, integration string) (*session.Session, error) {
		if integration != "" {
			return nil, trace.BadParameter("integration %q is not allowed to use static sessions", integration)
		}
		return awsSession, nil
	}
}

// SessionProviderUsingAmbientCredentials returns an AWS Session using ambient credentials.
// This is in contrast with AWS Sessions that can be generated using an AWS OIDC Integration.
func SessionProviderUsingAmbientCredentials() AWSSessionProvider {
	return func(ctx context.Context, region, integration string) (*session.Session, error) {
		if integration != "" {
			return nil, trace.BadParameter("integration %q is not allowed to use ambient sessions", integration)
		}
		useFIPSEndpoint := endpoints.FIPSEndpointStateUnset
		if modules.GetModules().IsBoringBinary() {
			useFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
		}
		session, err := session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
			Config: aws.Config{
				UseFIPSEndpoint: useFIPSEndpoint,
			},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return session, nil
	}
}
