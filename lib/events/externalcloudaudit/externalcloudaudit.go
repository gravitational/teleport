package externalcloudaudit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
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
	if !modules.GetModules().Features().Cloud || modules.GetModules().Features().IsUsageBasedBilling {
		return &Configurator{isUsed: false}, nil
	}
	// TODO(tobiaszheller/nklaassen): consider adding some mechanism to disable it
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

	credentialsCache, err := newCredentialsCache(CacheConfig{
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

	// Clock is used to control time.
	Clock clockwork.Clock

	// IntegratioName used to generate AWS OIDC credentials.
	IntegratioName string

	// RefreshBeforeExpirationPeriod defines duration which is used to check
	// if existing credentials needs refreshing, based on existing expiration value.
	RefreshBeforeExpirationPeriod time.Duration

	// RefreshIntervalCheck defines how often cache will check if credentials
	// needs refreshing.
	RefreshIntervalCheck time.Duration

	// RetrieveTimeout defines timeout used when calling Retrieve credentials.
	RetrieveTimeout time.Duration
}

func (cfg *CacheConfig) CheckAndSetDefaults() error {
	if cfg.IntegratioName == "" {
		return trace.BadParameter("missing parameter IntegratioName")
	}
	if cfg.RefreshBeforeExpirationPeriod == 0 {
		cfg.RefreshBeforeExpirationPeriod = 15 * time.Minute
	}
	if cfg.RefreshIntervalCheck == 0 {
		cfg.RefreshIntervalCheck = 30 * time.Second
	}
	if cfg.RetrieveTimeout == 0 {
		cfg.RetrieveTimeout = 30 * time.Second
	}
	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
		cfg.Log.WithField("component", "external-cloud-audit-credentials-cache")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
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

	// retrieveFn is dynamically set after auth is initialized.
	retrieveFn RetrieveCredentialsFn

	// initialized is used to communicate (via closing channel) that cache is
	// initialzed, after retrieveFn is set.
	initialized chan struct{}

	credsOrErr   credsOrErr
	credsOrErrMu sync.RWMutex
}

type credsOrErr struct {
	creds aws.Credentials
	err   error
}

func newCredentialsCache(cfg CacheConfig) (*CredentialsCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &CredentialsCache{
		CacheConfig: cfg,
		initialized: make(chan struct{}),
		credsOrErr: credsOrErr{
			err: errors.New("cache not yet initialized"),
		},
	}, nil
}

type RetrieveCredentialsFn func(ctx context.Context, integration string) (aws.Credentials, error)

// SetRetrieveCredentialsFn sets RetrieveCredentialsFn. It should be called only once.
func (cc *CredentialsCache) SetRetrieveCredentialsFn(fn RetrieveCredentialsFn) {
	cc.retrieveFn = fn
	close(cc.initialized)
}

func (cc *CredentialsCache) retrieve(ctx context.Context) (aws.Credentials, error) {
	ctx, cancel := context.WithTimeout(ctx, cc.RetrieveTimeout)
	defer cancel()
	return cc.retrieveFn(ctx, cc.IntegratioName)
}

func (cc *CredentialsCache) getCredentialsFromCache(ctx context.Context) (aws.Credentials, error) {
	cc.credsOrErrMu.RLock()
	defer cc.credsOrErrMu.RUnlock()
	return cc.credsOrErr.creds, cc.credsOrErr.err
}

func (cc *CredentialsCache) retrieveIfNeeded(ctx context.Context) {
	credsFromCache, err := cc.getCredentialsFromCache(ctx)
	if err == nil && credsFromCache.HasKeys() && cc.Clock.Now().Add(cc.RefreshBeforeExpirationPeriod).Before(credsFromCache.Expires) {
		// No need to refresh, valid credentials in cache
		return
	}
	cc.Log.Debugf("BYOB Credentials need refreshing")

	creds, err := cc.retrieve(ctx)
	if err != nil {
		// if we were not able to retrive, check if existing credentials in cache are still valid.
		// if yes, just log debug, it will be retired on next interval check.
		if credsFromCache.HasKeys() && !credsFromCache.Expired() {
			cc.Log.WithError(err).Debugf("BYOB Failed to retrieve credentials, old ones will be still used")
			return
		}
		// if not, update cached error.
		cc.credsOrErrMu.Lock()
		cc.credsOrErr = credsOrErr{err: trace.Wrap(err)}
		cc.credsOrErrMu.Unlock()
		return
	}
	// refresh went well, update cached values.
	cc.credsOrErrMu.Lock()
	cc.credsOrErr = credsOrErr{creds: creds}
	cc.credsOrErrMu.Unlock()
}

func (cc *CredentialsCache) run(ctx context.Context) {
	// wait for initialized signal before running loop.
	select {
	case <-cc.initialized:
		cc.retrieveIfNeeded(ctx)
	case <-ctx.Done():
		cc.Log.WithError(ctx.Err()).Debugf("CredentialsCache received cancel signal before initialized")
		return
	}

	checkInterval := interval.New(interval.Config{
		Duration: cc.RefreshIntervalCheck,
		Jitter:   retryutils.NewSeventhJitter(),
	})
	defer checkInterval.Stop()
	for {
		select {
		case <-checkInterval.Next():
			cc.retrieveIfNeeded(ctx)
		case <-ctx.Done():
			cc.Log.WithError(ctx.Err()).Debugf("CredentialsCache received cancel signal, stopping refreshing loop")
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
