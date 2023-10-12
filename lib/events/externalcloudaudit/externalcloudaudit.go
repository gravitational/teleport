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

// TODO(tobiaszheller): looking for better name.
// Configurator allows to read external cloud audit spec and take care of
// refreshing AWS credentials used in OIDC integration.
type Configurator struct {
	// spec is set on initialization of Configurator. It won't change, because
	// every change of spec triggers service reload.
	spec   *externalcloudaudit.ExternalCloudAuditSpec
	isUsed bool

	credentialsCache *CredentialsCache
}

// NewConfigurator created new instance of Configurator.
// If external cloud audit is not used in cluster or auth is not running in Cloud,
// valid instance will be returned with 'isUsed' property set to false.
// Error is returned when it cannot check config in backend.
func NewConfigurator(ctx context.Context, bk backend.Backend) (*Configurator, error) {
	if !modules.GetModules().Features().Cloud {
		return &Configurator{isUsed: false}, nil
	}
	// TODO(tobiaszheller): consider adding some mechanism to disable it
	// via env flag or some other solution.

	svc := local.NewExternalCloudAuditService(bk)
	externalAudit, err := svc.GetClusterExternalCloudAudit(ctx)
	if err != nil {
		if trace.IsNotFound(err) {
			return &Configurator{isUsed: false}, nil
		}
		// TODO(tobiaszheller/nklaassen): if backend is not available, auth won't start
		// due to it, only on Cloud. Check if this is resonable.
		return nil, trace.Wrap(err)
	}

	credentialsCache, err := newExternalCloudAuditCredentialsCache(CacheConfig{
		IntegratioName: externalAudit.Spec.IntegrationName,
	})
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

func (c *Configurator) GetSpec() *externalcloudaudit.ExternalCloudAuditSpec {
	return c.spec
}

func (c *Configurator) IsUsed() bool {
	if c == nil {
		return false
	}
	return c.isUsed
}

func (c *Configurator) SetRetrieveCredentialsFn(fn RetrieveCredentialsFn) {
	c.credentialsCache.SetRetrieveCredentialsFn(fn)
}

type CacheConfig struct {
	// Log is a logger.
	Log logrus.FieldLogger

	// IntegratioName used to generate AWS OIDC credentials.
	IntegratioName string
}

func (cfg *CacheConfig) CheckAndSetDefaults() error {
	if cfg.IntegratioName == "" {
		return trace.BadParameter("missing parameter IntegratioName")
	}
	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
		cfg.Log.WithField("component", "external-cloud-audit-credentials-cache")
	}
	return nil
}

// CredentialsCache is used to store and refresh AWS credentials used with
// AWS OIDC integration.
// Credentials are valid 1h, but they cannot be refreshed if proxy is down.
// That's why we are trying to refresh it before they are about to expire.
//
// CredentialsCache is dependency to both s3 session uploader and athena audit
// logger. They are both initialized before auth. However AWS credentials using
// OIDC integration can be obtained only after auth is initialized.
// That's why retrieveFn is injected dynamically after auth is initialized.
// Before initialization, CredentialsCache will return error on Retrive call.
type CredentialsCache struct {
	CacheConfig
	// TODO(tobiaszheller): do we need that mutex? so far it's replaced with initialized chan.
	// mu protects retrieveFn.
	// mu sync.RWMutex
	// retrieveFn is dynamically set after auth is initialized.
	retrieveFn RetrieveCredentialsFn

	// initialized is used to communicate (via closing channel) that cache is
	// initialzed, after retrieveFn is set.
	initialized chan struct{}

	creds   aws.Credentials
	credsMu sync.RWMutex
}

func newExternalCloudAuditCredentialsCache(cfg CacheConfig) (*CredentialsCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &CredentialsCache{
		CacheConfig: cfg,
		initialized: make(chan struct{}),
	}, nil
}

type RetrieveCredentialsFn func(ctx context.Context, integration string) (aws.Credentials, error)

// SetRetrieveCredentialsFn sets RetrieveCredentialsFn. It should be called only once.
func (cc *CredentialsCache) SetRetrieveCredentialsFn(fn RetrieveCredentialsFn) {
	// cc.mu.Lock()
	// defer cc.mu.Unlock()
	cc.retrieveFn = fn
	close(cc.initialized)
}

func (cc *CredentialsCache) isInitialized() bool {
	select {
	case <-cc.initialized:
		return true
	default:
		return false
	}

	// cc.mu.RLock()
	// defer cc.mu.RUnlock()
	// return cc.retrieveFn != nil
}

func (cc *CredentialsCache) retrieve(ctx context.Context) (aws.Credentials, error) {
	if !cc.isInitialized() {
		logrus.Warn("BYOB v2 not ready yet for generating")
		return aws.Credentials{}, errors.New("e BYOB credentials not ready yet")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return cc.retrieveFn(ctx, cc.IntegratioName)
}

func (cc *CredentialsCache) getCredentialsFromCache(ctx context.Context) (aws.Credentials, error) {
	cc.credsMu.RLock()
	defer cc.credsMu.RUnlock()
	if !cc.creds.HasKeys() {
		return aws.Credentials{}, errors.New("credentials not available in cache yet")
	}
	return cc.creds, nil
}

func (cc *CredentialsCache) credentialsNeedsRefresh() bool {
	cc.credsMu.RLock()
	defer cc.credsMu.RUnlock()
	return !cc.creds.HasKeys() || (cc.creds.CanExpire && cc.creds.Expires.Before(time.Now().Add(15*time.Minute)))
}

func (cc *CredentialsCache) retrieveIfCloseToExpiration(ctx context.Context) {
	if !cc.credentialsNeedsRefresh() {
		cc.Log.Debugf("BYOB Credentials don't need refreshing yet")
		return
	}
	cc.Log.Debugf("BYOB Credentials need refreshing")
	cc.retrieveCredentialsAndUpdateCache(ctx)
}

func (cc *CredentialsCache) retrieveCredentialsAndUpdateCache(ctx context.Context) {
	creds, err := cc.retrieve(ctx)
	if err != nil {
		cc.Log.WithError(err).Debugf("BYOB Failed to retrieve")
		return
	}
	cc.credsMu.Lock()
	cc.creds = creds
	cc.credsMu.Unlock()
}

func (cc *CredentialsCache) run(ctx context.Context) {
	// wait for initialized signal before running loop.
	select {
	case <-cc.initialized:
		cc.retrieveCredentialsAndUpdateCache(ctx)
	case <-ctx.Done():
		cc.Log.WithError(ctx.Err()).Debugf("BYOB Context closed before initialzed")
		return
	}

	checkInterval := interval.New(interval.Config{
		// Check for expiration every 1m
		Duration: time.Minute,
		Jitter:   retryutils.NewSeventhJitter(),
	})
	defer checkInterval.Stop()
	for {
		select {
		case <-checkInterval.Next():
			cc.retrieveIfCloseToExpiration(ctx)
		case <-ctx.Done():
			cc.Log.WithError(ctx.Err()).Debugf("BYOB Context closed with err. Returning from refresh loop.")
			return
		}
	}
}

func (p *Configurator) CredentialsSDKV2() *CredentialsSDKv2Provider {
	return &CredentialsSDKv2Provider{retrieveFn: p.credentialsCache.getCredentialsFromCache}
}

type CredentialsSDKv2Provider struct {
	retrieveFn func(context.Context) (aws.Credentials, error)
}

func (p *CredentialsSDKv2Provider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	credsV2, err := p.retrieveFn(ctx)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}
	logrus.Warnf("BYOB retrieve new credentials v2, expires %v %v \n", credsV2.CanExpire, credsV2.Expires)
	return credsV2, nil
}

func (p *Configurator) CredentialsSDKV1() *credentials.Credentials {
	return credentials.NewCredentials(&v1Provider{retrieveFn: p.credentialsCache.getCredentialsFromCache})
}

type v1Provider struct {
	retrieveFn func(context.Context) (aws.Credentials, error)
}

func (p *v1Provider) Retrieve() (credentials.Value, error) {
	credsV2, err := p.retrieveFn(context.Background())
	if err != nil {
		return credentials.Value{}, trace.Wrap(err)
	}
	logrus.Warnf("BYOB retrieve new credentials v1, expires %v %v \n", credsV2.CanExpire, credsV2.Expires)

	return credentials.Value{
		AccessKeyID:     credsV2.AccessKeyID,
		SecretAccessKey: credsV2.SecretAccessKey,
		SessionToken:    credsV2.SessionToken,
		ProviderName:    credsV2.Source,
	}, nil
}

// TODO(tobiaszheller): rework.
func (c *v1Provider) IsExpired() bool {
	return false
}
