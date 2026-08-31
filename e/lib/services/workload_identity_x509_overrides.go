package services

import (
	"context"
	"crypto/x509"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type workloadIdentityIssuerOverride struct {
	issuer *x509.Certificate
	chain  [][]byte
}

type ParsedWorkloadIdentityX509IssuerOverride struct {
	// overrides is a map of DER-encoded SPKI to (parsed) issuer and (encoded)
	// certificate chain.
	overrides map[string]workloadIdentityIssuerOverride
}

func (p *ParsedWorkloadIdentityX509IssuerOverride) GetCAOverride(ca *tlsca.CertAuthority) (*tlsca.CertAuthority, [][]byte, bool) {
	override, ok := p.overrides[string(ca.Cert.RawSubjectPublicKeyInfo)]
	if !ok {
		return nil, nil, false
	}

	// this is ok because the same SPKI results in the same public key, so
	// override.issuer.PublicKey is equal to ca.Signer.Public()
	return &tlsca.CertAuthority{
		Cert:   override.issuer,
		Signer: ca.Signer,
	}, override.chain, true
}

func ParseWorkloadIdentityX509IssuerOverride(resource *workloadidentityv1pb.X509IssuerOverride) (*ParsedWorkloadIdentityX509IssuerOverride, error) {
	if expected, actual := apitypes.KindWorkloadIdentityX509IssuerOverride, resource.GetKind(); expected != actual {
		return nil, trace.BadParameter("expected kind %v, got %q", expected, actual)
	}
	if expected, actual := apitypes.V1, resource.GetVersion(); expected != actual {
		return nil, trace.BadParameter("expected version %v, got %q", expected, actual)
	}
	if expected, actual := "", resource.GetSubKind(); expected != actual {
		return nil, trace.BadParameter("expected sub_kind %v, got %q", expected, actual)
	}
	if name := resource.GetMetadata().GetName(); name == "" {
		return nil, trace.BadParameter("missing name")
	}
	if name := resource.GetMetadata().GetName(); name == "none" {
		return nil, trace.BadParameter("got reserved name \"none\"")
	}

	// TODO(espadolini): get rid of this limitation once the story around
	// multiple independent overrides and trust domains is more defined
	if name := resource.GetMetadata().GetName(); name != "default" {
		return nil, trace.BadParameter("expected name \"default\", got %q", name)
	}

	parsed := &ParsedWorkloadIdentityX509IssuerOverride{
		overrides: make(map[string]workloadIdentityIssuerOverride, len(resource.GetSpec().GetOverrides())),
	}
	for _, override := range resource.GetSpec().GetOverrides() {
		issuer, err := x509.ParseCertificate(override.GetIssuer())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if _, alreadyExists := parsed.overrides[string(issuer.RawSubjectPublicKeyInfo)]; alreadyExists {
			return nil, trace.BadParameter("different overrides with the same public key are not allowed")
		}
		for _, certDER := range override.GetChain() {
			if _, err := x509.ParseCertificate(certDER); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		parsed.overrides[string(issuer.RawSubjectPublicKeyInfo)] = workloadIdentityIssuerOverride{
			issuer: issuer,
			chain:  override.GetChain(),
		}
	}

	return parsed, nil
}

// NewWorkloadIdentityX509IssuerOverrideCache returns a
// [services.WorkloadIdentityX509CAOverrideGetter] that fetches overrides from a
// storage service as needed, parsing them and keeping the resulting value for a
// while. It can optionally make use of a backend watcher to invalidate the
// cache as changes happen to the stored override configuration, to follow the
// changes more promptly.
func NewWorkloadIdentityX509IssuerOverrideCache(
	storage services.WorkloadIdentityX509Overrides,
	events backend.Backend,
	log *slog.Logger,
	cacheCtx context.Context,
) (*WorkloadIdentityX509IssuerOverrideCache, error) {
	return newWorkloadIdentityX509IssuerOverrideCache(
		storage,
		events,
		log,
		cacheCtx,
		clockwork.NewRealClock(),
	)
}

func newWorkloadIdentityX509IssuerOverrideCache(
	storage services.WorkloadIdentityX509Overrides,
	events backend.Backend,
	log *slog.Logger,
	cacheCtx context.Context,
	cacheClock clockwork.Clock,
) (*WorkloadIdentityX509IssuerOverrideCache, error) {
	const cacheTTL = 60 * time.Second
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     cacheTTL,
		Context: cacheCtx,
		Clock:   cacheClock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	watcherRetry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:     0,
		Driver:    retryutils.NewExponentialDriver(150 * time.Millisecond),
		Max:       3 * time.Minute,
		Jitter:    retryutils.HalfJitter,
		AutoReset: 2,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &WorkloadIdentityX509IssuerOverrideCache{
		storage: storage,
		events:  events,
		log:     log,

		watcherRetry: watcherRetry,
		cache:        cache,
	}, nil
}

type WorkloadIdentityX509IssuerOverrideCache struct {
	storage services.WorkloadIdentityX509Overrides
	events  backend.Backend
	log     *slog.Logger

	watcherRetry *retryutils.RetryV2
	cache        *utils.FnCache
}

var _ services.WorkloadIdentityX509CAOverrideGetter = (*WorkloadIdentityX509IssuerOverrideCache)(nil)

// RunWatcher will run backend watchers (with a backoff in case of errors) to
// invalidate the cache as changes happen to the underlying storage. The watcher
// loop will exit when the context terminates.
func (c *WorkloadIdentityX509IssuerOverrideCache) RunWatcher(ctx context.Context) {
	for {
		err := c.runOnce(ctx)
		if ctx.Err() != nil {
			return
		}

		backoff := c.watcherRetry.Duration()
		c.log.WarnContext(ctx, "Watcher exited, retrying after backoff", "backoff", backoff, "error", err)

		select {
		case <-ctx.Done():
			return
		case <-c.watcherRetry.Clock.After(backoff):
			c.watcherRetry.Inc()
		}
	}
}

// workloadIdentityX509IssuerOverridePrefix is the backend key prefix used for
// issuer overrides, the same as its namesake in lib/services/local.
const workloadIdentityX509IssuerOverridePrefix = "workload_identity_x509_issuer_override"

func (c *WorkloadIdentityX509IssuerOverrideCache) runOnce(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// TODO(espadolini): watch the whole prefix if we end up supporting more
	// than a single key
	prefix := backend.NewKey(workloadIdentityX509IssuerOverridePrefix, "default")
	w, err := c.events.NewWatcher(ctx, backend.Watch{
		Name:      apitypes.KindWorkloadIdentityX509IssuerOverride,
		Prefixes:  []backend.Key{prefix},
		QueueSize: 128,
	})
	if err != nil {
		return err
	}
	defer w.Close()

	c.log.DebugContext(ctx, "Watcher created, waiting for init")

	const initTimeout = 30 * time.Second
	select {
	case <-w.Done():
		return trace.Errorf("watcher closed while waiting for init")
	case <-ctx.Done():
		return trace.Wrap(context.Cause(ctx), "context canceled while waiting for init")
	case <-time.After(initTimeout):
		return trace.Errorf("timeout while waiting for init")
	case e := <-w.Events():
		if e.Type != apitypes.OpInit {
			return trace.BadParameter("expected init event, got type %q", e.Type)
		}
	}

	c.log.DebugContext(ctx, "Watcher initialized")

	// TODO(espadolini): clear the whole cache if we end up supporting more than
	// a single key
	c.cache.Remove("default")
	defer c.cache.Remove("default")

	for {
		var e backend.Event
		select {
		case <-w.Done():
			return trace.Errorf("watcher closed")
		case <-ctx.Done():
			return trace.Wrap(context.Cause(ctx), "context canceled")
		case e = <-w.Events():
		}

		switch e.Type {
		default:
			// kill the stream and reset it because we don't know what could've
			// possibly happened to the data (imagine this was a new OpTruncate
			// event, for example)
			return trace.BadParameter("unexpected event type %q", e.Type)
		case apitypes.OpPut, apitypes.OpDelete:
			// TODO(espadolini): invalidate the correct item if we end up
			// supporting more than a single key
			c.cache.Remove("default")
			c.log.DebugContext(ctx, "Received change, invalidating cache", "name", "default", "event_type", e.Type)
		}
	}
}

type workloadIdentityX509IssuerOverrideCacheItem struct {
	parsed *ParsedWorkloadIdentityX509IssuerOverride
	err    error
}

// GetWorkloadIdentityX509CAOverride implements [services.WorkloadIdentityX509CAOverrideGetter].
func (c *WorkloadIdentityX509IssuerOverrideCache) GetWorkloadIdentityX509CAOverride(ctx context.Context, name string, ca *tlsca.CertAuthority) (*tlsca.CertAuthority, [][]byte, error) {
	switch name {
	case "none":
		return ca, nil, nil
	case "", "default":
	default:
		return nil, nil, trace.NotFound(apitypes.KindWorkloadIdentityX509IssuerOverride+" %q doesn't exist", name)
	}

	r, err := c.cache.Get(ctx, "default", func(ctx context.Context) (workloadIdentityX509IssuerOverrideCacheItem, error) {
		resource, err := c.storage.GetX509IssuerOverride(ctx, "default")
		if err != nil {
			if trace.IsNotFound(err) {
				return workloadIdentityX509IssuerOverrideCacheItem{err: err}, nil
			}
			return workloadIdentityX509IssuerOverrideCacheItem{}, err
		}

		parsed, err := ParseWorkloadIdentityX509IssuerOverride(resource)
		if err != nil {
			return workloadIdentityX509IssuerOverrideCacheItem{err: err}, nil
		}
		return workloadIdentityX509IssuerOverrideCacheItem{parsed: parsed}, nil
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	parsed, err := r.parsed, r.err
	if err != nil {
		if name == "" && trace.IsNotFound(err) {
			return ca, nil, nil
		}
		return nil, nil, trace.Wrap(err)
	}

	newCA, chain, found := parsed.GetCAOverride(ca)
	if !found {
		if name == "" {
			return nil, nil, trace.BadParameter(apitypes.KindWorkloadIdentityX509IssuerOverride + " \"default\" exists but is missing issuers")
		}
		return nil, nil, trace.BadParameter("missing issuer override in "+apitypes.KindWorkloadIdentityX509IssuerOverride+" %q", name)
	}

	return newCA, chain, nil
}
