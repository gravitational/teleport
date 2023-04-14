package plugins

import (
	"context"
	"errors"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// ManagerConfig contains parameters and dependencies for Manager
type ManagerConfig struct {
	Authorizers *AuthorizerSet
	Backend     services.Plugins
	Events      types.Events
	Factories   map[types.PluginType]instanceFactory
	// TeleportClient is the Teleport API client passed to plugins
	TeleportClient teleport.Client
	// RetryConfig defines the backoff settings for retrying the inner event loop
	RetryConfig *retryutils.RetryV2Config

	Clock clockwork.Clock
	Log   *logrus.Entry
}

// checkAndSetDefaults validates the configuration and sets default values
func (cfg *ManagerConfig) checkAndSetDefaults() error {
	if cfg.Authorizers == nil {
		return trace.BadParameter("authorizers must be set")
	}
	if cfg.Backend == nil {
		return trace.BadParameter("backend must be set")
	}
	if cfg.Events == nil {
		return trace.BadParameter("events must be set")
	}
	if cfg.Factories == nil {
		cfg.Factories = map[types.PluginType]instanceFactory{
			types.PluginTypeSlack: slackInstanceFactory,
		}
	}
	if cfg.TeleportClient == nil {
		return trace.BadParameter("teleportClient must be set")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.RetryConfig == nil {
		cfg.RetryConfig = &retryutils.RetryV2Config{
			First:     utils.FullJitter(1 * time.Second),
			Driver:    retryutils.NewExponentialDriver(1 * time.Second),
			Max:       30 * time.Second,
			Jitter:    retryutils.NewHalfJitter(),
			AutoReset: 2,
			Clock:     cfg.Clock,
		}
	}

	if cfg.Log == nil {
		cfg.Log = logrus.NewEntry(logrus.StandardLogger())
	}

	return nil
}

// Manager listens for changes in plugin resources,
// and starts, stops, or reconfigures the plugin instances accordingly.
//
// Manager's event loop runs as a single goroutine,
// as such no synchronization to its fields is implemented.
type Manager struct {
	authorizers    *AuthorizerSet
	backend        services.Plugins
	events         types.Events
	factories      map[types.PluginType]instanceFactory
	instances      map[string]*instance
	teleportClient teleport.Client
	watcher        types.Watcher
	retryConfig    retryutils.RetryV2Config

	log *logrus.Entry
}

// NewManager constructs a new Manager from the given config
func NewManager(cfg ManagerConfig) (*Manager, error) {
	metrics.RegisterPrometheusCollectors(statuses)

	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	m := &Manager{
		authorizers:    cfg.Authorizers,
		backend:        cfg.Backend,
		events:         cfg.Events,
		factories:      cfg.Factories,
		instances:      make(map[string]*instance),
		teleportClient: cfg.TeleportClient,
		retryConfig:    *cfg.RetryConfig,

		log: cfg.Log,
	}
	return m, nil
}

// Run runs the main loop of Manager until the Manager's context is canceled.
// It internally retries the event loop in case the watcher closes for any reason.
func (m *Manager) Run(ctx context.Context) error {
	retry, err := retryutils.NewRetryV2(m.retryConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		err := m.runInner(ctx)
		if err == nil {
			m.log.Error("runInner should always return a non-nil error, but nil was returned")
		} else if errors.Is(err, context.Canceled) {
			m.log.Info("Plugin manager is shutting down per request")
			return err
		}

		retry.Inc()
		m.log.WithError(err).Errorf("Error in the event loop, will retry in %v", retry.Duration())
		<-retry.After()
	}
}

// runInner watches for changes in Plugin resources,
// and applies them to concrete running plugin instances.
func (m *Manager) runInner(ctx context.Context) error {
	defer func() {
		for name := range m.instances {
			m.shutdownInstance(name)
		}
	}()

	m.log.Info("Starting event loop")

	var err error
	m.watcher, err = m.events.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: types.KindPlugin},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	resources, err := m.backend.GetPlugins(ctx, true)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, resource := range resources {
		event := types.Event{
			Type:     types.OpPut,
			Resource: resource,
		}
		if err := m.dispatchEvent(event); err != nil {
			m.log.WithError(err).Errorf("failed to dispatch %v", event)
		}
	}

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			m.log.Infof("Event loop stopping: %v", err)
			return trace.Wrap(err)
		case <-m.watcher.Done():
			return trace.Wrap(m.watcher.Error())
		case event := <-m.watcher.Events():

			if err := m.dispatchEvent(event); err != nil {
				m.log.WithError(err).Errorf("failed to dispatch %v", event)
			}
		}
	}
}

func (m *Manager) dispatchEvent(e types.Event) error {
	if e.Resource == nil {
		return nil
	}
	name := e.Resource.GetName()

	if e.Resource.GetKind() != types.KindPlugin {
		return trace.BadParameter(`unsupported resource: "%s/%s"`, e.Resource.GetKind(), name)
	}

	switch e.Type {
	case types.OpDelete:
		m.shutdownInstance(name)
	case types.OpPut:
		if e.Resource.GetMetadata().Labels[HostedPluginLabel] != "true" {
			// Shut down instance in case it was previously hosted
			m.shutdownInstance(name)
			m.log.Infof("Plugin %q not marked as hosted, skipping", name)
			return nil
		}

		plugin, ok := e.Resource.(*types.PluginV1)
		if !ok {
			return trace.BadParameter("unsupported plugin type: %T", e.Resource)
		}

		if m.instanceUpToDate(name, &plugin.Spec) {
			break
		}
		m.shutdownInstance(name)
		if err := m.startInstance(plugin); err != nil {
			return trace.Wrap(err, "starting %v", name)
		}
	}

	return nil
}

func (m *Manager) shutdownInstance(name string) {
	instance, ok := m.instances[name]
	if !ok {
		return
	}

	m.log.Infof("Stopping plugin %s", name)
	instance.cancel()
	delete(m.instances, name)
}

// startInstance configures a plugin instance per the given spec,
// and starts it as a separate goroutine
func (m *Manager) startInstance(plugin *types.PluginV1) error {
	m.log.Infof("Starting plugin %s", plugin.GetName())

	factory, ok := m.factories[plugin.GetType()]
	if !ok {
		return trace.BadParameter("unsupported plugin type %q", plugin.GetType())
	}

	authorizer, err := m.authorizers.Get(plugin.GetType())
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.Wrap(err, "unsupported plugin type %q", plugin.GetType())
		}
		return trace.Wrap(err)
	}
	store := newPluginStore(m.backend, plugin.GetName())
	statusSink := newStatusSink(m.backend, plugin.GetName(), string(plugin.GetType()))

	log := m.log.WithFields(logrus.Fields{
		"plugin_name": plugin.GetName(),
		"plugin_type": plugin.GetType(),
	})
	deps := instanceDependencies{
		authorizer: authorizer,
		client:     m.teleportClient,
		store:      store,
		statusSink: statusSink,
		log:        log,
	}

	// Use Background() here for now, no connection to event loop's context.
	// We rely on cancel() being called correctly in all codepaths.
	// TODO(justinas): reconsider
	ctx, cancel := context.WithCancel(context.Background())
	delegate, err := factory(ctx, plugin, deps)
	if err != nil {
		cancel()
		return trace.Wrap(err)
	}

	// Plugin instances run in independent goroutines,
	// as they are long-running ("indefinitely") jobs.
	go func() {
		if err := delegate(); err != nil {
			// Stopping by request is not an error,
			// watcher already logs an error once when stopped.
			if !errors.Is(err, context.Canceled) && err.Error() != "watcher closed" {
				log.WithError(err).Error("plugin instance delegate failed")
				statusSink.Emit(ctx, types.PluginStatusV1{Code: types.PluginStatusCode_OTHER_ERROR})
			}
		}
	}()

	m.instances[plugin.GetName()] = &instance{
		cancel: cancel,
		spec:   &plugin.Spec,
	}
	return nil
}

func (m *Manager) instanceUpToDate(name string, spec *types.PluginSpecV1) bool {
	instance, ok := m.instances[name]
	if !ok {
		return false
	}
	return instance.spec.Equal(spec)
}
