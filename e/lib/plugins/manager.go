package plugins

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/lib/plugins/factory"
	"github.com/gravitational/teleport/e/lib/plugins/instance"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	teleclient "github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
)

// HeartbeatCreator is a function that will create heartbeats for a given component.
type HeartbeatCreator func(string) func(error)

// ManagerConfig contains parameters and dependencies for Manager
type ManagerConfig struct {
	Authorizers *AuthorizerSet
	// Plugins is the uncached plugin service.
	Plugins                 services.Plugins
	PluginStaticCredentials services.PluginStaticCredentials
	// Events can create an event watcher from the backend changefeed.
	// This MUST NOT be used with a cached client.
	Events    types.Events
	Factories map[types.PluginType]factory.Factory
	// TeleportClient is the Teleport API client passed to plugins.
	// This client will hit the cache when possible, else it will feed from the
	// backend directly.
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
		cfg.Factories = map[types.PluginType]factory.Factory{
			types.PluginTypeDiscord: factory.Discord,
			types.PluginTypeOkta:    factory.Okta,
			// We added the leader lock in slack to mitigate an issue in Cloud where several slack
			// instances might race when refreshing tokens.
			// TODO(hugoShaka): remove this once we have a better idea of what's going on
			types.PluginTypeSlack:             withLeaderLock(factory.Slack),
			types.PluginTypeOpsgenie:          factory.OpsGenie,
			types.PluginTypeServiceNow:        factory.ServiceNow,
			types.PluginTypePagerDuty:         factory.PagerDuty,
			types.PluginTypeJamf:              factory.Jamf,
			types.PluginTypeIntune:            factory.Intune,
			types.PluginTypeJira:              factory.Jira,
			types.PluginTypeMattermost:        factory.Mattermost,
			types.PluginTypeGitlab:            factory.GitLab,
			types.PluginTypeEntraID:           afterCacheReady(factory.EntraID),
			types.PluginTypeDatadog:           factory.Datadog,
			types.PluginTypeAWSIdentityCenter: afterCacheReady(withLeaderLock(factory.AWSIC)),
			types.PluginTypeGithub:            factory.GitHub,
			types.PluginTypeMSTeams:           factory.MSTeams,
			types.PluginTypeEmail:             factory.Email,
			types.PluginTypeNetIQ:             factory.NetIQ,
			types.PluginTypeSCIM:              afterCacheReady(factory.SCIM),
		}
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.RetryConfig == nil {
		cfg.RetryConfig = &retryutils.RetryV2Config{
			First:     retryutils.FullJitter(1 * time.Second),
			Driver:    retryutils.NewExponentialDriver(1 * time.Second),
			Max:       30 * time.Second,
			Jitter:    retryutils.HalfJitter,
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
	factories               map[types.PluginType]factory.Factory
	instancesByName         map[string]*instance.Instance
	instancesByCredential   map[string]*instance.Instance
	teleportClient          teleclient.Client
	watcher                 types.Watcher
	retryConfig             retryutils.RetryV2Config
	parentProcess           *service.TeleportProcess
	log                     *slog.Logger
	metrics                 *hostedPluginsRegistry
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
		instancesByName:         make(map[string]*instance.Instance),
		instancesByCredential:   make(map[string]*instance.Instance),
		teleportClient:          cfg.TeleportClient,
		retryConfig:             *cfg.RetryConfig,
		parentProcess:           cfg.ParentProcess,
		log:                     cfg.Logger,
		metrics:                 newHostedPluginsRegistry(),
	}

	cfg.ParentProcess.AddGatherer(m.metrics.registry)
	return m, nil
}

func (m *Manager) recordInstance(i *instance.Instance) {
	m.instancesByName[i.Plugin.GetName()] = i

	if credRef := i.Plugin.GetCredentials().GetStaticCredentialsRef(); credRef != nil {
		staticCredentialID := credRef.Labels[eteleport.PluginLabel]
		m.instancesByCredential[staticCredentialID] = i
	}
}

func (m *Manager) deleteInstance(instanceName string) {
	instance, ok := m.instancesByName[instanceName]
	if !ok {
		return
	}

	delete(m.instancesByName, instanceName)
	if credRef := instance.Plugin.GetCredentials().GetStaticCredentialsRef(); credRef != nil {
		delete(m.instancesByCredential, credRef.Labels[eteleport.PluginLabel])
	}
}

// Run runs the main loop of Manager until the Manager's context is canceled.
// It internally retries the event loop in case the watcher closes for any reason.
func (m *Manager) Run(ctx context.Context) error {
	retry, err := retryutils.NewRetryV2(m.retryConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		select {
		case <-ctx.Done():
			m.log.InfoContext(ctx, "Plugin manager is shutting down.")
			return nil
		default:
		}

		err := m.runInner(ctx)
		if err == nil {
			m.log.ErrorContext(ctx, "runInner should always return a non-nil error, but nil was returned")
		} else if errors.Is(err, context.Canceled) {
			m.log.InfoContext(ctx, "Plugin manager is shutting down per request")
			return err
		}

		retry.Inc()
		m.log.ErrorContext(ctx, "Error in the event loop, backing off", "backoff_duration", retry.Duration(), "error", err)
		select {
		case <-retry.After():
		case <-ctx.Done():
			m.log.InfoContext(ctx, "Plugin manager is shutting down.")
			return nil
		}
	}
}

// runInner watches for changes in Plugin resources,
// and applies them to concrete running plugin instances.
func (m *Manager) runInner(ctx context.Context) error {
	defer func() {
		for name := range m.instancesByName {
			m.shutdownInstance(name)
		}
	}()

	m.log.InfoContext(ctx, "Starting event loop")

	var err error
	m.watcher, err = m.events.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: types.KindPlugin},
			{Kind: types.KindPluginStaticCredentials},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// This watcher is uncached, by waiting its init, we are not waiting
	// for the cache to be filled, only for the services to be started.
	m.log.DebugContext(ctx, "Initial plugin startup complete, watching for new plugin events")
	select {
	case evt := <-m.watcher.Events():
		if evt.Type != types.OpInit {
			return trace.BadParameter("unexpected event type %q, was expecting OpInit", evt.Type)
		}
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case <-m.watcher.Done():
		return trace.Wrap(ctx.Err())
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

	// now we are up-to-date, we can process watcher events
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

	switch e.Resource.GetKind() {
	case types.KindPlugin:
		return m.dispatchPluginEvent(ctx, e)

	case types.KindPluginStaticCredentials:
		return m.dispatchPluginStaticCredentialsEvent(ctx, e)

	default:
		return trace.BadParameter(`unsupported resource: "%s/%s"`, e.Resource.GetKind(), e.Resource.GetName())
	}
}

func (m *Manager) dispatchPluginEvent(ctx context.Context, e types.Event) error {
	name := e.Resource.GetName()

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

		if m.instanceUpToDate(name, plugin) {
			return nil
		}
		m.shutdownInstance(name)
		if err := m.startInstance(ctx, plugin); err != nil {
			return trace.Wrap(err, "starting %v", name)
		}
	}
	return nil
}

func (m *Manager) dispatchPluginStaticCredentialsEvent(ctx context.Context, e types.Event) error {
	if e.Type != types.OpPut {
		return nil
	}

	log := m.log.With(slog.String("credential_resource_name", e.Resource.GetName()))
	log.InfoContext(ctx, "Handling credential update for plugin")

	updatedCredential, ok := e.Resource.(*types.PluginStaticCredentialsV1)
	if !ok {
		return trace.BadParameter("unexpected resource type %T received for plugin static credential", updatedCredential)
	}

	pluginCredentialID, ok := updatedCredential.GetLabel(eteleport.PluginLabel)
	if !ok {
		return trace.BadParameter("credential missing plugin label")
	}

	log = log.With(slog.String("plugin_unique_id", pluginCredentialID))
	log.InfoContext(ctx, "Looking up plugin instance")

	instance, ok := m.instancesByCredential[pluginCredentialID]
	if !ok {
		log.InfoContext(ctx, "No such plugin instance")
		return nil
	}

	log = log.With(slog.String("plugin_name", instance.Plugin.GetName()))
	log.InfoContext(ctx, "Looking up in-use credential by name")

	liveCred := instance.FindCredentialByName(updatedCredential.GetName())
	if liveCred == nil {
		log.WarnContext(ctx, "No in-use credentials found for plugin")
		return nil
	}

	log.InfoContext(ctx, "Checking changes in credential")
	if m.credentialUpToDate(liveCred, updatedCredential) {
		log.InfoContext(ctx, "Credential unchanged. Do not restart.")
		return nil
	}

	// If we get to here, we know that a credential has changed, AND that the
	// credential belongs to a running plugin. Restart it so that it can pick up
	// the new, updated credentials
	log.InfoContext(ctx, "Detected credential update. Restarting plugin")
	return trace.Wrap(m.restartInstance(ctx, instance.GetName()))
}

func (m *Manager) shutdownInstance(name string) {
	instance, ok := m.instancesByName[name]
	if !ok {
		return
	}

	m.log.InfoContext(context.Background(), "Stopping plugin", "plugin_name", name)
	instance.Cancel()
	if plugin := instance.Plugin; plugin != nil {
		m.metrics.remove(plugin)
	}
	m.deleteInstance(name)
}

// startInstance configures a plugin instance per the given spec,
// and starts it as a separate goroutine
func (m *Manager) startInstance(ctx context.Context, plugin *types.PluginV1) error {
	log := m.log.With(teleport.ComponentKey, plugin.GetName(),
		"plugin_name", plugin.GetName(),
		"plugin_type", plugin.GetType(),
	)

	log.InfoContext(ctx, "Starting plugin")

	factoryFn, ok := m.factories[plugin.GetType()]
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

	store := newPluginStore(m.plugins, plugin.GetName(), log)
	statusSink := newStatusSink(m.plugins, plugin.GetName(), string(plugin.GetType()))

	staticCreds, err := m.getStaticCredentials(ctx, plugin)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	credPointers, err := cloneCredentials(staticCreds)
	if err != nil {
		return trace.Wrap(err)
	}

	pluginMetricsRegistry := prometheus.NewRegistry()

	if err := m.metrics.add(plugin, pluginMetricsRegistry); err != nil {
		m.log.ErrorContext(ctx, "Failed to register plugin metrics", "error", err)
		// Failure to expose metrics is not bad enough for us to refuse starting the plugin.
	}
	// We wrap the metrics registry to prefix every metric reported by the plugin
	// with the plugin type and name. This avoids conflicts and properly indicates
	// who registered the metric.
	reg, err := metrics.NewRegistry(pluginMetricsRegistry, "teleport_plugin", strings.ReplaceAll(string(plugin.GetType()), "-", "_"))
	if err != nil {
		return trace.Wrap(err, "building plugin metrics registry")
	}

	// Use Background() here for now, no connection to event loop's context.
	// We rely on cancel() being called correctly in all codepaths.
	// TODO(justinas): reconsider
	pluginCtx, cancel := context.WithCancel(context.Background())
	deps := factory.Dependencies{
		Authorizer:        authorizer,
		Client:            m.teleportClient,
		Store:             store,
		StatusSink:        statusSink,
		ParentProcess:     m.parentProcess,
		StaticCredentials: staticCreds,
		Logger:            log,
		PluginsService:    m.plugins,
		MetricsRegistry:   reg,
	}

	// Note that we give a copy of the plugin resource to the plugin factory. If
	// we shared the same resource instance with the plugin process, and that
	// process were plugin to modify it, then the Plugin Update monitor would
	// treat that as an update and immediately attempt to restart the plugin,
	// starting off an infinite sequence of modifications and restarts.

	delegate, err := factoryFn(ctx, apiutils.CloneProtoMsg(plugin), deps)
	if err != nil {
		cancel()
		return trace.Wrap(err)
	}

	// Plugin instances run in independent goroutines,
	// as they are long-running ("indefinitely") jobs.
	go func() {
		if err := delegate(pluginCtx); err != nil {
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
				if err := statusSink.Emit(ctx, &types.PluginStatusV1{
					Code:         types.PluginStatusCode_UNAUTHORIZED,
					ErrorMessage: err.Error(),
				}); err != nil {
					log.ErrorContext(ctx, "Failed to emit plugin unauthorized status", "error", err)
				}

			default:
				log.ErrorContext(ctx, "Plugin instance delegate failed", "error", err)
				if err := statusSink.Emit(pluginCtx, &types.PluginStatusV1{
					Code:         types.PluginStatusCode_OTHER_ERROR,
					ErrorMessage: err.Error(),
				}); err != nil {
					log.ErrorContext(ctx, "Failed to emit plugin error status", "error", err)
				}
			}
		}
	}()

	m.recordInstance(instance.New(cancel, plugin, credPointers))

	return nil
}

func (m *Manager) instanceUpToDate(name string, updated *types.PluginV1) bool {
	instance, ok := m.instancesByName[name]
	if !ok {
		return false
	}
	return instance.IsUpToDate(updated)
}

// cloneCredentials clones a slice of [types.PluginStaticCredentials]s, returning
// them as a slice of concrete [*types.PluginStaticCredentialsV1] values
func cloneCredentials(s []types.PluginStaticCredentials) ([]*types.PluginStaticCredentialsV1, error) {
	result := make([]*types.PluginStaticCredentialsV1, len(s))
	for i, src := range s {
		dst, ok := src.(*types.PluginStaticCredentialsV1)
		if !ok {
			return nil, trace.BadParameter("unexpected credential type %T", src)
		}
		result[i] = apiutils.CloneProtoMsg(dst)
	}
	return result, nil
}

// credentialUpToDate checks if the live credential has been updated
func (m *Manager) credentialUpToDate(liveCred, updatedCred *types.PluginStaticCredentialsV1) bool {
	return liveCred.Spec.Equal(updatedCred.Spec)
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

func (m *Manager) restartInstance(ctx context.Context, name string) error {
	m.log.InfoContext(ctx, "Restarting instance", "instance_name", name)

	m.shutdownInstance(name)

	m.log.InfoContext(ctx, "Looking up plugin resource", "instance_name", name)
	plugin, err := m.plugins.GetPlugin(ctx, name, true /* with secrets */)
	if err != nil {
		m.log.InfoContext(ctx, "Failed plugin resource lookup", "error", err)
		return trace.Wrap(err)
	}
	pluginPtr, ok := plugin.(*types.PluginV1)
	if !ok {
		return trace.BadParameter("unexpected plugin type %T", plugin)
	}
	return trace.Wrap(m.startInstance(ctx, pluginPtr))
}
