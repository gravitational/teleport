// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package externalcloudaudit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types/externalcloudaudit"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	refreshBeforeExpirationPeriod = 15 * time.Minute
	refreshIntervalCheck          = 30 * time.Second
	retrieveTimeout               = 30 * time.Second
)

// Configurator provides functionality necessary for configuring the External
// Cloud Audit feature.
//
// Specifically:
//   - IsUsed() reports whether the feature is currently activated and in use.
//   - Access to the current cluster ExternalCloudAudit config via GetSpec()
//   - Credentials providers for v1 and v2 of the AWS SDK for Go that grant access
//     to necessary resources in the external AWS account via an existing OIDC
//     integration.
type Configurator struct {
	// spec is set during initialization of the Configurator. It won't
	// change, because every change of spec triggers an Auth service reload.
	spec   *externalcloudaudit.ExternalCloudAuditSpec
	isUsed bool

	credentialsCache *credentialsCache
}

// NewConfigurator returns a new Configurator set up with the current active
// cluster ExternalCloudAudit spec from [bk].
//
// If the External Cloud Audit feature is not used in this cluster then a valid
// instance will be returned where IsUsed() will return false.
func NewConfigurator(ctx context.Context, bk backend.Backend) (*Configurator, error) {
	// ExternalCloudAudit is only available in Cloud (not Team).
	if !modules.GetModules().Features().Cloud || modules.GetModules().Features().IsUsageBasedBilling {
		return &Configurator{isUsed: false}, nil
	}

	svc := local.NewExternalCloudAuditService(bk)
	externalAudit, err := svc.GetClusterExternalCloudAudit(ctx)
	if err != nil {
		if trace.IsNotFound(err) {
			return &Configurator{isUsed: false}, nil
		}
		return nil, trace.Wrap(err)
	}

	credentialsCache, err := newCredentialsCache(externalAudit.Spec.IntegrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go credentialsCache.run(ctx)

	return &Configurator{
		isUsed:           true,
		spec:             &externalAudit.Spec,
		credentialsCache: credentialsCache,
	}, nil
}

func (c *Configurator) IsUsed() bool {
	return c.isUsed
}

func (c *Configurator) GetSpec() *externalcloudaudit.ExternalCloudAuditSpec {
	return c.spec
}

// RetrieveCredentialsFn is a function that returns aws.Credentials provided by
// the OIDC integration named [integration].
type RetrieveCredentialsFn func(ctx context.Context, integration string) (aws.Credentials, error)

func (c *Configurator) SetRetrieveCredentialsFn(fn RetrieveCredentialsFn) {
	c.credentialsCache.SetRetrieveCredentialsFn(fn)
}

func (p *Configurator) CredentialsSDKV2() *v2Provider {
	return &v2Provider{retrieveFn: p.credentialsCache.getCredentialsFromCache}
}

func (p *Configurator) CredentialsSDKV1() *credentials.Credentials {
	return credentials.NewCredentials(&v1Provider{retrieveFn: p.credentialsCache.getCredentialsFromCache})
}

// credentialsCache is used to store and refresh AWS credentials used with
// AWS OIDC integration.
//
// Credentials are valid for 1h, but they cannot be refreshed if Proxy is down,
// so we attempt to refresh the credentials early and retry on failure.
//
// credentialsCache is dependency to both s3 session uploader and athena audit
// logger. They are both initialized before auth. However AWS credentials using
// OIDC integration can be obtained only after auth is initialized.
// That's why retrieveFn is injected dynamically after auth is initialized.
// Before initialization, credentialsCache will return error on Retrive call.
type credentialsCache struct {
	oidcIntegration string
	log             *logrus.Entry

	// retrieveFn is dynamically set after auth is initialized.
	retrieveFn RetrieveCredentialsFn

	// initialized is used to communicate (via closing channel) that cache is
	// initialized, after retrieveFn is set.
	initialized chan struct{}

	credsOrErr   credsOrErr
	credsOrErrMu sync.RWMutex
}

type credsOrErr struct {
	creds aws.Credentials
	err   error
}

func newCredentialsCache(oidcIntegration string) (*credentialsCache, error) {
	return &credentialsCache{
		oidcIntegration: oidcIntegration,
		log:             logrus.WithField(trace.Component, "ExternalCloudAudit.CredentialsCache"),
		initialized:     make(chan struct{}),
		credsOrErr: credsOrErr{
			err: errors.New("cache not yet initialized"),
		},
	}, nil
}

// SetRetrieveCredentialsFn sets RetrieveCredentialsFn. It should be called only once.
func (cc *credentialsCache) SetRetrieveCredentialsFn(fn RetrieveCredentialsFn) {
	cc.retrieveFn = fn
	close(cc.initialized)
}

func (cc *credentialsCache) getCredentialsFromCache(ctx context.Context) (aws.Credentials, error) {
	cc.credsOrErrMu.RLock()
	defer cc.credsOrErrMu.RUnlock()
	return cc.credsOrErr.creds, cc.credsOrErr.err
}

func (cc *credentialsCache) retrieve(ctx context.Context) (aws.Credentials, error) {
	ctx, cancel := context.WithTimeout(ctx, retrieveTimeout)
	defer cancel()
	return cc.retrieveFn(ctx, cc.oidcIntegration)
}

func (cc *credentialsCache) retrieveIfNeeded(ctx context.Context) {
	credsFromCache, err := cc.getCredentialsFromCache(ctx)
	if err == nil && credsFromCache.HasKeys() && time.Now().Add(refreshBeforeExpirationPeriod).Before(credsFromCache.Expires) {
		// No need to refresh, valid credentials in cache
		return
	}
	cc.log.Debugf("Refreshing credentials.")

	creds, err := cc.retrieve(ctx)
	if err != nil {
		// If we were not able to retrieve, check if existing credentials in cache are still valid.
		// If yes, just log debug, it will be retried on next interval check.
		if credsFromCache.HasKeys() && !credsFromCache.Expired() {
			cc.log.Warnf("Failed to retrieve new credentials: %v", err)
			cc.log.Debugf("Using existing credentials expiring in %s.", time.Until(credsFromCache.Expires).Round(time.Second).String())
			return
		}
		// If existing creds are expired, update cached error.
		cc.credsOrErrMu.Lock()
		cc.credsOrErr = credsOrErr{err: trace.Wrap(err)}
		cc.credsOrErrMu.Unlock()
		return
	}
	// Refresh went well, update cached creds.
	cc.credsOrErrMu.Lock()
	cc.credsOrErr = credsOrErr{creds: creds}
	cc.credsOrErrMu.Unlock()
}

func (cc *credentialsCache) run(ctx context.Context) {
	// Wait for initialized signal before running loop.
	select {
	case <-cc.initialized:
	case <-ctx.Done():
		cc.log.Debug("Context cancelled before initialized.")
		return
	}

	cc.retrieveIfNeeded(ctx)

	checkInterval := interval.New(interval.Config{
		Duration: refreshIntervalCheck,
		Jitter:   retryutils.NewSeventhJitter(),
	})
	defer checkInterval.Stop()
	for {
		select {
		case <-checkInterval.Next():
			cc.retrieveIfNeeded(ctx)
		case <-ctx.Done():
			cc.log.Debugf("Context cancelled, stopping refresh loop.")
			return
		}
	}
}

type v2Provider struct {
	retrieveFn func(context.Context) (aws.Credentials, error)
}

var _ aws.CredentialsProvider = (*v2Provider)(nil)

func (p *v2Provider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	credsV2, err := p.retrieveFn(ctx)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}
	return credsV2, nil
}

type v1Provider struct {
	retrieveFn func(context.Context) (aws.Credentials, error)
}

var _ credentials.ProviderWithContext = (*v1Provider)(nil)

func (p *v1Provider) RetrieveWithContext(ctx context.Context) (credentials.Value, error) {
	credsV2, err := p.retrieveFn(ctx)
	if err != nil {
		return credentials.Value{}, trace.Wrap(err)
	}

	return credentials.Value{
		AccessKeyID:     credsV2.AccessKeyID,
		SecretAccessKey: credsV2.SecretAccessKey,
		SessionToken:    credsV2.SessionToken,
		ProviderName:    credsV2.Source,
	}, nil
}

func (p *v1Provider) Retrieve() (credentials.Value, error) {
	return p.RetrieveWithContext(context.Background())
}

func (c *v1Provider) IsExpired() bool {
	// Opt out of AWS SDK credential caching by always returning true.
	// Retrieve(WithContext) already caches credentials.
	return true
}
