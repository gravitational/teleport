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

package externalauditstorage

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// TokenLifetime is the lifetime of OIDC tokens used by the
	// ExternalAuditStorage service with the AWS OIDC integration.
	TokenLifetime = time.Hour

	refreshBeforeExpirationPeriod = 15 * time.Minute
	refreshCheckInterval          = 30 * time.Second
	retrieveTimeout               = 30 * time.Second
)

// Configurator provides functionality necessary for configuring the External
// Cloud Audit feature.
//
// Specifically:
//   - IsUsed() reports whether the feature is currently activated and in use.
//   - GetSpec() provides the current cluster ExternalAuditStorageSpec
//   - CredentialsProvider() provides AWS credentials for the necessary customer
//     resources that can be used with aws-sdk-go-v2
//   - CredentialsProviderSDKV1() provides AWS credentials for the necessary customer
//     resources that can be used with aws-sdk-go
//
// Configurator is a dependency to both the S3 session uploader and the Athena
// audit logger. They are both initialized before Auth. However, Auth needs to
// be initialized in order to provide signatures for the OIDC tokens.  That's
// why SetGenerateOIDCTokenFn() must be called after auth is initialized to inject
// the OIDC token source dynamically.
//
// If auth needs to emit any events during initialization (before
// SetGenerateOIDCTokenFn is called) that is okay. Events are written to
// SQS first, credentials from the Configurator are not needed until the batcher
// reads the events from SQS and tries to write a batch to the customer S3
// bucket. If the batcher tries to write a batch before the Configurator is
// initialized and gets an error when trying to retrieve credentials, that's
// still okay, it will always retry.
type Configurator struct {
	// ErrorCounter provides audit middlewares that count errors and raise or clear
	// cluster alerts based on recent error rates.
	// It will be nil if created via NewDraftConfigurator.
	ErrorCounter *ErrorCounter

	// spec is set during initialization of the Configurator. It won't
	// change, because every change of spec triggers an Auth service reload.
	spec   *externalauditstorage.ExternalAuditStorageSpec
	isUsed bool

	credentialsCache *credentialsCache
}

// Options holds options for the Configurator.
type Options struct {
	clock     clockwork.Clock
	stsClient stscreds.AssumeRoleWithWebIdentityAPIClient
}

func (o *Options) setDefaults(ctx context.Context, region string) error {
	if o.clock == nil {
		o.clock = clockwork.NewRealClock()
	}
	if o.stsClient == nil {
		var useFips aws.FIPSEndpointState
		if modules.GetModules().IsBoringBinary() {
			useFips = aws.FIPSEndpointStateEnabled
		}
		cfg, err := config.LoadDefaultConfig(
			ctx,
			config.WithRegion(region),
			config.WithUseFIPSEndpoint(useFips),
			config.WithRetryMaxAttempts(10),
		)
		if err != nil {
			return trace.Wrap(err)
		}
		o.stsClient = sts.NewFromConfig(cfg)
	}
	return nil
}

// WithClock is a functional option to set the clock.
func WithClock(clock clockwork.Clock) func(*Options) {
	return func(opts *Options) {
		opts.clock = clock
	}
}

// WithSTSClient is a functional option to set the sts client.
func WithSTSClient(clt stscreds.AssumeRoleWithWebIdentityAPIClient) func(*Options) {
	return func(opts *Options) {
		opts.stsClient = clt
	}
}

// ExternalAuditStorageGetter is an interface for a service that can retrieve
// External Audit Storage configuration.
type ExternalAuditStorageGetter interface {
	// GetClusterExternalAuditStorage returns the current cluster External Audit
	// Storage configuration.
	GetClusterExternalAuditStorage(context.Context) (*externalauditstorage.ExternalAuditStorage, error)
	// GetDraftExternalAuditStorage returns the current draft External Audit
	// Storage configuration.
	GetDraftExternalAuditStorage(context.Context) (*externalauditstorage.ExternalAuditStorage, error)
}

// IntegrationGetter is an interface for a service that can retrieve an
// integration by name.
type IntegrationGetter interface {
	// GetIntegration returns the specified integration resources.
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
}

// NewConfigurator returns a new Configurator set up with the current active
// cluster ExternalAuditStorage spec from [ecaSvc].
//
// If the External Audit Storage feature is not used in this cluster then a valid
// instance will be returned where IsUsed() will return false.
func NewConfigurator(ctx context.Context, ecaSvc ExternalAuditStorageGetter, integrationSvc services.IntegrationsGetter, alertService ClusterAlertService, optFns ...func(*Options)) (*Configurator, error) {
	active, err := ecaSvc.GetClusterExternalAuditStorage(ctx)
	if err != nil {
		if trace.IsNotFound(err) {
			return &Configurator{isUsed: false}, nil
		}
		return nil, trace.Wrap(err)
	}
	return newConfigurator(ctx, &active.Spec, integrationSvc, alertService, optFns...)
}

// NewDraftConfigurator is equivalent to NewConfigurator but is based on the
// current *draft* ExternalAuditStorage configuration instead of the active
// configuration.
//
// If a draft ExternalAuditStorage configuration is not found, an error will be
// returned.
func NewDraftConfigurator(ctx context.Context, ecaSvc ExternalAuditStorageGetter, integrationSvc services.IntegrationsGetter, optFns ...func(*Options)) (*Configurator, error) {
	draft, err := ecaSvc.GetDraftExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Draft configurator never needs to set up cluster alerts.
	return newConfigurator(ctx, &draft.Spec, integrationSvc, nil /* alertService */, optFns...)
}

func newConfigurator(ctx context.Context, spec *externalauditstorage.ExternalAuditStorageSpec, integrationSvc services.IntegrationsGetter, alertService ClusterAlertService, optFns ...func(*Options)) (*Configurator, error) {
	// ExternalAuditStorage is only available in Cloud Enterprise
	if !modules.GetModules().Features().Cloud || modules.GetModules().Features().IsTeam() {
		return &Configurator{isUsed: false}, nil
	}

	oidcIntegrationName := spec.IntegrationName
	integration, err := integrationSvc.GetIntegration(ctx, oidcIntegrationName)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound(
				"ExternalAuditStorage: configured AWS OIDC integration %q not found",
				oidcIntegrationName)
		}
		return nil, trace.Wrap(err)
	}
	awsOIDCSpec := integration.GetAWSOIDCIntegrationSpec()
	if awsOIDCSpec == nil {
		return nil, trace.NotFound(
			"ExternalAuditStorage: configured integration %q does not appear to be an AWS OIDC integration",
			oidcIntegrationName)
	}
	awsRoleARN := awsOIDCSpec.RoleARN

	options := &Options{}
	for _, optFn := range optFns {
		optFn(options)
	}
	if err := options.setDefaults(ctx, spec.Region); err != nil {
		return nil, trace.Wrap(err)
	}

	credentialsCache, err := newCredentialsCache(oidcIntegrationName, awsRoleARN, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go credentialsCache.run(ctx)

	// Draft configurator does not need to count errors or create cluster
	// alerts.
	var errorCounter *ErrorCounter
	if alertService != nil {
		errorCounter = NewErrorCounter(alertService)
		go errorCounter.run(ctx)
	}

	return &Configurator{
		ErrorCounter:     errorCounter,
		isUsed:           true,
		spec:             spec,
		credentialsCache: credentialsCache,
	}, nil
}

// IsUsed returns a boolean indicating whether the ExternalAuditStorage feature is
// currently in active use.
func (c *Configurator) IsUsed() bool {
	return c != nil && c.isUsed
}

// GetSpec returns the current active ExternalAuditStorageSpec.
func (c *Configurator) GetSpec() *externalauditstorage.ExternalAuditStorageSpec {
	return c.spec
}

// GenerateOIDCTokenFn is a function that should return a valid, signed JWT for
// authenticating to AWS via OIDC.
type GenerateOIDCTokenFn func(ctx context.Context, integration string) (string, error)

// SetGenerateOIDCTokenFn sets the source of OIDC tokens for this Configurator.
func (c *Configurator) SetGenerateOIDCTokenFn(fn GenerateOIDCTokenFn) {
	c.credentialsCache.setGenerateOIDCTokenFn(fn)
}

// CredentialsProvider returns an aws.CredentialsProvider that can be used to
// authenticate with the customer AWS account via the configured AWS OIDC
// integration with aws-sdk-go-v2.
func (p *Configurator) CredentialsProvider() aws.CredentialsProvider {
	return p.credentialsCache
}

// CredentialsProviderSDKV1 returns a credentials.ProviderWithContext that can be used to
// authenticate with the customer AWS account via the configured AWS OIDC
// integration with aws-sdk-go.
func (p *Configurator) CredentialsProviderSDKV1() credentials.ProviderWithContext {
	return &v1Adapter{cc: p.credentialsCache}
}

// WaitForFirstCredentials waits for the internal credentials cache to finish
// fetching its first credentials (or getting an error attempting to do so).
// This can be called after SetGenerateOIDCTokenFn to make sure any returned
// credential providers won't return errors simply due to the cache not being
// ready yet.
func (p *Configurator) WaitForFirstCredentials(ctx context.Context) {
	p.credentialsCache.waitForFirstCredsOrErr(ctx)
}

// credentialsCache is used to store and refresh AWS credentials used with
// AWS OIDC integration.
//
// Credentials are valid for 1h, but they cannot be refreshed if Proxy is down,
// so we attempt to refresh the credentials early and retry on failure.
//
// credentialsCache is a dependency to both the s3 session uploader and the
// athena audit logger. They are both initialized before auth. However AWS
// credentials using OIDC integration can be obtained only after auth is
// initialized. That's why generateOIDCTokenFn is injected dynamically after
// auth is initialized. Before initialization, credentialsCache will return
// an error on any Retrieve call.
type credentialsCache struct {
	log *logrus.Entry

	roleARN     string
	integration string

	// generateOIDCTokenFn is dynamically set after auth is initialized.
	generateOIDCTokenFn GenerateOIDCTokenFn

	// initialized communicates (via closing channel) that generateOIDCTokenFn is set.
	initialized      chan struct{}
	closeInitialized func()

	// gotFirstCredsOrErr communicates (via closing channel) that the first
	// credsOrErr has been set.
	gotFirstCredsOrErr      chan struct{}
	closeGotFirstCredsOrErr func()

	credsOrErr   credsOrErr
	credsOrErrMu sync.RWMutex

	stsClient stscreds.AssumeRoleWithWebIdentityAPIClient
	clock     clockwork.Clock
}

type credsOrErr struct {
	creds aws.Credentials
	err   error
}

func newCredentialsCache(integration, roleARN string, options *Options) (*credentialsCache, error) {
	initialized := make(chan struct{})
	gotFirstCredsOrErr := make(chan struct{})
	return &credentialsCache{
		roleARN:                 roleARN,
		integration:             integration,
		log:                     logrus.WithField(teleport.ComponentKey, "ExternalAuditStorage.CredentialsCache"),
		initialized:             initialized,
		closeInitialized:        sync.OnceFunc(func() { close(initialized) }),
		gotFirstCredsOrErr:      gotFirstCredsOrErr,
		closeGotFirstCredsOrErr: sync.OnceFunc(func() { close(gotFirstCredsOrErr) }),
		credsOrErr: credsOrErr{
			err: errors.New("ExternalAuditStorage: credential cache not yet initialized"),
		},
		clock:     options.clock,
		stsClient: options.stsClient,
	}, nil
}

func (cc *credentialsCache) setGenerateOIDCTokenFn(fn GenerateOIDCTokenFn) {
	cc.generateOIDCTokenFn = fn
	cc.closeInitialized()
}

// Retrieve implements [aws.CredentialsProvider] and returns the latest cached
// credentials, or an error if no credentials have been generated yet or the
// last generated credentials have expired.
func (cc *credentialsCache) Retrieve(ctx context.Context) (aws.Credentials, error) {
	cc.credsOrErrMu.RLock()
	defer cc.credsOrErrMu.RUnlock()
	return cc.credsOrErr.creds, cc.credsOrErr.err
}

func (cc *credentialsCache) run(ctx context.Context) {
	// Wait for initialized signal before running loop.
	select {
	case <-cc.initialized:
	case <-ctx.Done():
		cc.log.Debug("Context canceled before initialized.")
		return
	}

	cc.refreshIfNeeded(ctx)

	ticker := cc.clock.NewTicker(refreshCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			cc.refreshIfNeeded(ctx)
		case <-ctx.Done():
			cc.log.Debugf("Context canceled, stopping refresh loop.")
			return
		}
	}
}

func (cc *credentialsCache) refreshIfNeeded(ctx context.Context) {
	credsFromCache, err := cc.Retrieve(ctx)
	if err == nil &&
		credsFromCache.HasKeys() &&
		cc.clock.Now().Add(refreshBeforeExpirationPeriod).Before(credsFromCache.Expires) {
		// No need to refresh, credentials in cache are still valid for longer
		// than refreshBeforeExpirationPeriod
		return
	}
	cc.log.Debugf("Refreshing credentials.")

	creds, err := cc.refresh(ctx)
	if err != nil {
		// If we were not able to refresh, check if existing credentials in cache are still valid.
		// If yes, just log debug, it will be retried on next interval check.
		if credsFromCache.HasKeys() && cc.clock.Now().Before(credsFromCache.Expires) {
			cc.log.Warnf("Failed to retrieve new credentials: %v", err)
			cc.log.Debugf("Using existing credentials expiring in %s.", credsFromCache.Expires.Sub(cc.clock.Now()).Round(time.Second).String())
			return
		}
		// If existing creds are expired, update cached error.
		cc.setCredsOrErr(credsOrErr{err: trace.Wrap(err)})
		return
	}
	// Refresh went well, update cached creds.
	cc.setCredsOrErr(credsOrErr{creds: creds})
	cc.log.Debugf("Successfully refreshed credentials, new expiry at %v", creds.Expires)
}

func (cc *credentialsCache) setCredsOrErr(coe credsOrErr) {
	cc.credsOrErrMu.Lock()
	defer cc.credsOrErrMu.Unlock()
	cc.credsOrErr = coe
	cc.closeGotFirstCredsOrErr()
}

func (cc *credentialsCache) refresh(ctx context.Context) (aws.Credentials, error) {
	oidcToken, err := cc.generateOIDCTokenFn(ctx, cc.integration)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}

	roleProvider := stscreds.NewWebIdentityRoleProvider(
		cc.stsClient,
		cc.roleARN,
		identityToken(oidcToken),
		func(wiro *stscreds.WebIdentityRoleOptions) {
			wiro.Duration = TokenLifetime
		},
	)

	ctx, cancel := context.WithTimeout(ctx, retrieveTimeout)
	defer cancel()

	creds, err := roleProvider.Retrieve(ctx)
	return creds, trace.Wrap(err)
}

func (cc *credentialsCache) waitForFirstCredsOrErr(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-cc.gotFirstCredsOrErr:
	}
}

// identityToken is an implementation of [stscreds.IdentityTokenRetriever] for returning a static token.
type identityToken string

// GetIdentityToken returns the token configured.
func (j identityToken) GetIdentityToken() ([]byte, error) {
	return []byte(j), nil
}

// v1Adapter wraps the credentialsCache to implement
// [credentials.ProviderWithContext] used by aws-sdk-go (v1).
type v1Adapter struct {
	cc *credentialsCache
}

var _ credentials.ProviderWithContext = (*v1Adapter)(nil)

// RetrieveWithContext returns cached credentials.
func (a *v1Adapter) RetrieveWithContext(ctx context.Context) (credentials.Value, error) {
	credsV2, err := a.cc.Retrieve(ctx)
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

// Retrieve returns cached credentials.
func (a *v1Adapter) Retrieve() (credentials.Value, error) {
	return a.RetrieveWithContext(context.Background())
}

// IsExpired always returns true in order to opt out of AWS SDK credential
// caching. Retrieve(WithContext) already returns cached credentials.
func (a *v1Adapter) IsExpired() bool {
	return true
}
