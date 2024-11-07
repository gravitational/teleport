package plugins

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	teleclient "github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// HeartbeatCreator is a function that will create heartbeats for a given component.
type HeartbeatCreator func(string) func(error)

// ManagerConfig contains parameters and dependencies for Manager
type ManagerConfig struct {
	Authorizers             *AuthorizerSet
	Plugins                 services.Plugins
	PluginStaticCredentials services.PluginStaticCredentials
	Events                  types.Events
	Factories               map[types.PluginType]instanceFactory
	// TeleportClient is the Teleport API client passed to plugins
	TeleportClient teleclient.Client
	// RetryConfig defines the backoff settings for retrying the inner event loop
	RetryConfig *retryutils.RetryV2Config
	// ParentProcess is the process that is running this plugin manager. This is needed
	// for plugins that do things like start services.
	ParentProcess *service.TeleportProcess
	Clock         clockwork.Clock
	Logger        *slog.Logger
}

// checkAndSetDefaults validates the configuration and sets default values
func (cfg *ManagerConfig) checkAndSetDefaults() error {
	if cfg.Authorizers == nil {
		return trace.BadParameter("authorizers must be set")
	}
	if cfg.Plugins == nil {
		return trace.BadParameter("plugins must be set")
	}
	if cfg.PluginStaticCredentials == nil {
		return trace.BadParameter("plugin static credentials must be set")
	}
	if cfg.Events == nil {
		return trace.BadParameter("events must be set")
	}
	if cfg.TeleportClient == nil {
		return trace.BadParameter("teleportClient must be set")
	}
	if cfg.ParentProcess == nil {
		return trace.BadParameter("parent process must be set")
	}

	if cfg.Factories == nil {
		cfg.Factories = map[types.PluginType]instanceFactory{
			types.PluginTypeDiscord:           discordInstanceFactory,
			types.PluginTypeOkta:              oktaInstanceFactory,
			types.PluginTypeSlack:             slackInstanceFactory,
			types.PluginTypeOpsgenie:          opsgenieInstanceFactory,
			types.PluginTypeServiceNow:        serviceNowInstanceFactory,
			types.PluginTypePagerDuty:         pagerDutyInstanceFactory,
			types.PluginTypeJamf:              jamfInstanceFactory,
			types.PluginTypeJira:              jiraInstanceFactory,
			types.PluginTypeMattermost:        mattermostInstanceFactory,
			types.PluginTypeGitlab:            gitlabInstanceFactory,
			types.PluginTypeEntraID:           entraIDInstanceFactory,
			types.PluginTypeDatadog:           datadogInstanceFactory,
			types.PluginTypeAWSIdentityCenter: awsIdentityCenterInstanceFactory,
			types.PluginTypeMSTeams:           msTeamsInstanceFactory,
		}
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
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return nil
}

// Manager listens for changes in plugin resources,
// and starts, stops, or reconfigures the plugin instances accordingly.
//
// Manager's event loop runs as a single goroutine,
// as such no synchronization to its fields is implemented.
type Manager struct {
	authorizers             *AuthorizerSet
	plugins                 services.Plugins
	pluginStaticCredentials services.PluginStaticCredentials
	events                  types.Events
	factories               map[types.PluginType]instanceFactory
	instances               map[string]*instance
	teleportClient          teleclient.Client
	watcher                 types.Watcher
	retryConfig             retryutils.RetryV2Config
	parentProcess           *service.TeleportProcess
	log                     *slog.Logger
}

// NewManager constructs a new Manager from the given config
func NewManager(cfg ManagerConfig) (*Manager, error) {
	metrics.RegisterPrometheusCollectors(statuses)

	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	m := &Manager{
		authorizers:             cfg.Authorizers,
		plugins:                 cfg.Plugins,
		pluginStaticCredentials: cfg.PluginStaticCredentials,
		events:                  cfg.Events,
		factories:               cfg.Factories,
		instances:               make(map[string]*instance),
		teleportClient:          cfg.TeleportClient,
		retryConfig:             *cfg.RetryConfig,
		parentProcess:           cfg.ParentProcess,
		log:                     cfg.Logger,
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
			m.log.ErrorContext(ctx, "runInner should always return a non-nil error, but nil was returned")
		} else if errors.Is(err, context.Canceled) {
			m.log.InfoContext(ctx, "Plugin manager is shutting down per request")
			return err
		}

		retry.Inc()
		m.log.ErrorContext(ctx, "Error in the event loop, backing off", "backoff_duration", retry.Duration(), "error", err)
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

	m.log.InfoContext(ctx, "Starting event loop")

	var err error
	m.watcher, err = m.events.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: types.KindPlugin},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	resources, err := m.plugins.GetPlugins(ctx, true)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, resource := range resources {
		event := types.Event{
			Type:     types.OpPut,
			Resource: resource,
		}
		if err := m.dispatchEvent(ctx, event); err != nil {
			m.log.ErrorContext(ctx, "failed to dispatch event", "event", event, "error", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			m.log.InfoContext(ctx, "Event loop stopping", "error", err)
			return trace.Wrap(err)
		case <-m.watcher.Done():
			return trace.Wrap(m.watcher.Error())
		case event := <-m.watcher.Events():

			if err := m.dispatchEvent(ctx, event); err != nil {
				m.log.ErrorContext(ctx, "failed to dispatch event", "event", event, "error", err)
			}
		}
	}
}

func (m *Manager) dispatchEvent(ctx context.Context, e types.Event) error {
	// Ignore watch status events because they are sent on watch start
	// to indicate the current state of the watched resources. We don't
	// need to do anything with them and they just serve to inform the program that
	// the watch has started successfully and report which resources are currently
	// being watched - this behavior was introduced when we introduced the
	// partial watch feature.
	if e.Resource == nil || e.Resource.GetKind() == types.KindWatchStatus {
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
			m.log.InfoContext(ctx, "Plugin not marked as hosted, skipping", "plugin_name", name)
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
		if err := m.startInstance(ctx, plugin); err != nil {
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

	m.log.InfoContext(context.Background(), "Stopping plugin", "plugin_name", name)
	instance.cancel()
	delete(m.instances, name)
}

// startInstance configures a plugin instance per the given spec,
// and starts it as a separate goroutine
func (m *Manager) startInstance(ctx context.Context, plugin *types.PluginV1) error {
	log := m.log.With(teleport.ComponentKey, plugin.GetName(),
		"plugin_name", plugin.GetName(),
		"plugin_type", plugin.GetType(),
	)

	log.InfoContext(ctx, "Starting plugin")

	factory, ok := m.factories[plugin.GetType()]
	if !ok {
		return trace.BadParameter("unsupported plugin type %q", plugin.GetType())
	}

	var authorizer *Authorizer
	if NeedsOAuth(plugin) {
		var err error
		authorizer, err = m.authorizers.Get(plugin.GetType())
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.Wrap(err, "unsupported plugin type %q", plugin.GetType())
			}
			return trace.Wrap(err)
		}
	}

	store := newPluginStore(m.plugins, plugin.GetName())
	statusSink := newStatusSink(m.plugins, plugin.GetName(), string(plugin.GetType()))

	staticCreds, err := m.getStaticCredentials(ctx, plugin)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	// Use Background() here for now, no connection to event loop's context.
	// We rely on cancel() being called correctly in all codepaths.
	// TODO(justinas): reconsider
	pluginCtx, cancel := context.WithCancel(context.Background())
	deps := instanceDependencies{
		lifetime:          pluginCtx,
		authorizer:        authorizer,
		client:            m.teleportClient,
		store:             store,
		statusSink:        statusSink,
		parentProcess:     m.parentProcess,
		staticCredentials: staticCreds,
		logger:            log,
		pluginsService:    m.plugins,
	}

	// Note that we give a copy of the plugin resource to the plugin factory. If
	// we shared the same resource instance with the plugin process, and that
	// process were plugin to modify it, then the Plugin Update monitor would
	// treat that as an update and immediately attempt to restart the plugin,
	// starting off an infinite sequence of modifications and restarts.

	delegate, err := factory(ctx, apiutils.CloneProtoMsg(plugin), deps)
	if err != nil {
		cancel()
		return trace.Wrap(err)
	}

	// Plugin instances run in independent goroutines,
	// as they are long-running ("indefinitely") jobs.
	go func() {
		if err := delegate(); err != nil {
			var accessDenied *trace.AccessDeniedError

			switch {
			case errors.Is(err, context.Canceled) || err.Error() == "watcher closed":
				// Stopping by request is not an error, and watcher already logs an
				// error once when stopped.
				break

			case errors.As(err, &accessDenied):
				// Authentication failed for some reason. Let's at least hint to
				// the user what the problem might be.
				log.ErrorContext(ctx, "Plugin instance delegate failed due to authentication error")
				statusSink.Emit(ctx, &types.PluginStatusV1{
					Code:         types.PluginStatusCode_UNAUTHORIZED,
					ErrorMessage: err.Error(),
				})

			default:
				log.ErrorContext(ctx, "Plugin instance delegate failed", "error", err)
				statusSink.Emit(pluginCtx, &types.PluginStatusV1{
					Code:         types.PluginStatusCode_OTHER_ERROR,
					ErrorMessage: err.Error(),
				})
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

// getStaticCredentials will return static credentials for a plugin if they are needed.
func (m *Manager) getStaticCredentials(ctx context.Context, plugin types.Plugin) ([]types.PluginStaticCredentials, error) {
	// The credentials field is a oneof, so it can have multiple values. The static credentials
	// ref may not be present, so if it isn't present, we'll just assume that this object
	// doesn't have any static credentials set. Otherwise, we'll get the static credentials.
	staticCredsRef := plugin.GetCredentials().GetStaticCredentialsRef()
	if staticCredsRef == nil {
		return nil, trace.NotFound("no static credentials found")
	}

	staticCreds, err := m.pluginStaticCredentials.GetPluginStaticCredentialsByLabels(ctx, staticCredsRef.Labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return staticCreds, nil
}

// NeedsOAuth returns true if the plugin needs OAuth.
func NeedsOAuth(plugin types.Plugin) bool {
	switch plugin.GetType() {
	case types.PluginTypeSlack:
		return true
	}
	return false
}
