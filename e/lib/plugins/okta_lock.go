package plugins

import (
	"context"
	"encoding/base64"
	"log/slog"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/plugins/factory"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// oktaSemaphoreExpiration is the TTL for the Okta semaphore lease.
	oktaSemaphoreExpiration = 3 * time.Minute

	// oktaRetryInterval is the interval between attempts to acquire the
	// semaphore.
	oktaRetryInterval = time.Minute
)

// withOktaLock wraps a plugin factory to ensure that only one Okta plugin
// instance runs at a time across all Auth instances, using a Teleport semaphore
// for distributed locking.
//
// This uses the same semaphore kind ("okta-service") and name (base64 of the
// Okta org URL) as the old leader.Leader package, so old and new instances
// respect each other's leadership during rolling upgrades.
func withOktaLock(handler factory.Factory) factory.Factory {
	return func(ctx context.Context, plugin *types.PluginV1, deps factory.Dependencies) (factory.Delegate, error) {
		pluginFunc, err := handler(ctx, plugin, deps)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		authServer := deps.ParentProcess.GetAuthServer()
		return executeWithOktaLock(authServer, authServer.ServerID, plugin, pluginFunc, deps.Logger), nil
	}
}

func executeWithOktaLock(semaphores types.Semaphores, holder string, plugin *types.PluginV1, pluginFunc factory.Delegate, logger *slog.Logger) factory.Delegate {
	return func(ctx context.Context) error {
		orgURL := plugin.Spec.GetOkta().OrgUrl
		semaphoreName := oktaSemaphoreName(orgURL)

		log := logger.With(
			"semaphore_kind", okta.OktaServiceSemaphoreKind,
			"semaphore_name", semaphoreName,
			"plugin_name", plugin.GetName(),
		)

		for {
			log.DebugContext(ctx, "Attempting to acquire Okta semaphore")

			lock, err := services.AcquireSemaphoreLockWithRetry(ctx, services.SemaphoreLockConfigWithRetry{
				SemaphoreLockConfig: services.SemaphoreLockConfig{
					Service: semaphores,
					Expiry:  oktaSemaphoreExpiration,
					Params: types.AcquireSemaphoreRequest{
						SemaphoreKind: okta.OktaServiceSemaphoreKind,
						SemaphoreName: semaphoreName,
						MaxLeases:     1,
						Holder:        holder,
					},
				},
				Retry: retryutils.LinearConfig{
					Max:    oktaRetryInterval,
					Step:   oktaRetryInterval / 4,
					Jitter: retryutils.DefaultJitter,
				},
			})
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				log.DebugContext(ctx, "Okta Plugin instance is in standby mode trying to acquire a lock held by leader, will retry")
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(oktaRetryInterval):
					continue
				}
			}

			log.DebugContext(ctx, "Acquired Okta semaphore, starting plugin")

			// Run the plugin. The lock's context is canceled when the lease
			// is lost, so the plugin will be stopped automatically.
			pluginErr := pluginFunc(lock)

			// Plugin stopped — release the semaphore.
			lock.Stop()
			if err := lock.Wait(); err != nil && ctx.Err() == nil {
				log.DebugContext(ctx, "Semaphore lock released with error", "error", err)
			}

			if pluginErr != nil && ctx.Err() == nil {
				log.ErrorContext(ctx, "Plugin execution error, will retry", "error", pluginErr)
			}

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(oktaRetryInterval):
			}
		}
	}
}

// oktaSemaphoreName returns the semaphore name for the given Okta org URL.
// This encoding is shared between the old leader.Leader package and the new
// withOktaLock wrapper to ensure backward compatibility during rolling upgrades.
func oktaSemaphoreName(orgURL string) string {
	orgURL = strings.TrimSuffix(orgURL, "/")
	return base64.RawURLEncoding.EncodeToString([]byte(orgURL))
}
