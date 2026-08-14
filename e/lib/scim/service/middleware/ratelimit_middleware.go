package middleware

import (
	"cmp"
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
)

// RateLimitMiddleware holds per-plugin rate limiters keyed by plugin name. It
// holds a server-level default and merges it with per-plugin overrides stored
// in the plugin's SCIM settings. Call [RateLimitMiddleware.ForPlugin] to obtain
// a [Middleware] bound to a specific plugin; entries are rebuilt when the
// effective config changes, evicted in LRU order past [rateLimitCacheSize],
// and removed when the plugin has no SCIM configuration.
type RateLimitMiddleware struct {
	defaultLimit *types.PluginSCIMRateLimit
	mu           sync.Mutex
	// limiter is the currently active limiter for SCIM Plugin.
	// Since multiple SCIM plugins are not supported there is no need to
	// track rate limiter per plugin. If multiple plugins are added in the future, this should be
	// changed to a map of plugin name to limiter, and the ForPlugin method should be updated accordingly.
	limiter *rateLimitMiddleware
}

// NewRateLimitMiddleware creates a [RateLimitMiddleware] from the supplied
// server-level default. cfg must not be nil.
func NewRateLimitMiddleware(cfg common.RateLimitConfig) (*RateLimitMiddleware, error) {
	return &RateLimitMiddleware{
		defaultLimit: &types.PluginSCIMRateLimit{
			Average:                 cfg.Average,
			Burst:                   cfg.Burst,
			PeriodSeconds:           cfg.PeriodSeconds,
			MaxConcurrentOperations: cfg.MaxConcurrentOperations,
		},
	}, nil
}

// ForPlugin returns a [Middleware] for the given plugin. The entry is rebuilt
// when the effective config changes and removed when the plugin has no SCIM
// configuration.
func (r *RateLimitMiddleware) ForPlugin(plugin types.Plugin) (Middleware, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg := r.rateLimitsForPlugin(plugin)
	if r.limiter != nil {
		if r.limiter.cfg.Equal(cfg) {
			return r.limiter, nil
		}
	}
	mw, err := newRateLimitMiddleware(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r.limiter = mw
	return mw, nil
}

// rateLimitsForPlugin merges the plugin's SCIM rate-limit settings with the server
// default, using cmp.Or so that zero plugin fields fall back to the default.
func (r *RateLimitMiddleware) rateLimitsForPlugin(plugin types.Plugin) *types.PluginSCIMRateLimit {
	rl := scimRateLimitFromPlugin(plugin)
	def := r.defaultLimit
	if rl == nil {
		return def
	}
	return &types.PluginSCIMRateLimit{
		Average:                 cmp.Or(rl.Average, def.Average),
		Burst:                   cmp.Or(rl.Burst, def.Burst),
		PeriodSeconds:           cmp.Or(rl.PeriodSeconds, def.PeriodSeconds),
		MaxConcurrentOperations: cmp.Or(rl.MaxConcurrentOperations, def.MaxConcurrentOperations),
	}
}

func scimRateLimitFromPlugin(plugin types.Plugin) *types.PluginSCIMRateLimit {
	v1, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil
	}
	scim := v1.Spec.GetScim()
	if scim == nil {
		return nil
	}
	return scim.RateLimit
}

// rateLimitMiddleware is the per-plugin rate limiter built from a specific config.
type rateLimitMiddleware struct {
	ForwardingMiddleware
	cfg *types.PluginSCIMRateLimit
	rl  *common.RateLimiter
}

func newRateLimitMiddleware(cfg *types.PluginSCIMRateLimit) (*rateLimitMiddleware, error) {
	rl, err := common.NewRateLimiter(common.RateLimitConfig{
		Average:                 cfg.Average,
		Burst:                   cfg.Burst,
		PeriodSeconds:           cfg.PeriodSeconds,
		MaxConcurrentOperations: cfg.MaxConcurrentOperations,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &rateLimitMiddleware{cfg: cfg, rl: rl}, nil
}

func (r *rateLimitMiddleware) checkLimits(ctx context.Context, token string, mutating bool) (release func(), err error) {
	if err := r.rl.CheckRateLimit(token); err != nil {
		var rlErr *common.RateLimitExceededError
		if errors.As(err, &rlErr) {
			retryAfter := roundUpSeconds(rlErr.Delay)
			grpc.SetTrailer(ctx, metadata.Pairs("retry-after", strconv.FormatInt(retryAfter, 10)))
			return nil, trace.LimitExceeded("rate limit exceeded, retry after %ds", retryAfter)
		}
		return nil, trace.Wrap(err)
	}
	if !mutating {
		return func() {}, nil
	}
	release, err = r.rl.AcquireMutation(token)
	if err != nil {
		grpc.SetTrailer(ctx, metadata.Pairs("retry-after", strconv.FormatInt(r.cfg.PeriodSeconds, 10)))
		return nil, trace.LimitExceeded("too many concurrent SCIM mutations, retry after %ds", r.cfg.PeriodSeconds)
	}
	return release, nil
}

func (r *rateLimitMiddleware) GetResourceMiddleware(ctx context.Context, req *pb.GetSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
	release, err := r.checkLimits(ctx, req.GetTarget().GetPluginId(), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()
	return next.GetResource(ctx, req)
}

func (r *rateLimitMiddleware) ListResourcesMiddleware(ctx context.Context, req *pb.ListSCIMResourcesRequest, next common.ResourceHandler) (*pb.ResourceList, error) {
	release, err := r.checkLimits(ctx, req.GetTarget().GetPluginId(), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()
	return next.ListResources(ctx, req)
}

func (r *rateLimitMiddleware) CreateResourceMiddleware(ctx context.Context, req *pb.CreateSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
	release, err := r.checkLimits(ctx, req.GetTarget().GetPluginId(), true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()
	return next.CreateResource(ctx, req)
}

func (r *rateLimitMiddleware) UpdateResourceMiddleware(ctx context.Context, req *pb.UpdateSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
	release, err := r.checkLimits(ctx, req.GetTarget().GetPluginId(), true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()
	return next.UpdateResource(ctx, req)
}

func (r *rateLimitMiddleware) DeleteResourceMiddleware(ctx context.Context, req *pb.DeleteSCIMResourceRequest, next common.ResourceHandler) error {
	release, err := r.checkLimits(ctx, req.GetTarget().GetPluginId(), true)
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()
	return next.DeleteResource(ctx, req)
}

func (r *rateLimitMiddleware) PatchResourceMiddleware(ctx context.Context, req *pb.PatchSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
	release, err := r.checkLimits(ctx, req.GetTarget().GetPluginId(), true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()
	return next.PatchResource(ctx, req)
}

// roundUpSeconds returns the delay in whole seconds, rounded up.
func roundUpSeconds(d time.Duration) int64 {
	return int64((d + time.Second - 1) / time.Second)
}
