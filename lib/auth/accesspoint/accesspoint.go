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

// package accesspoint provides helpers for configuring caches in the context of
// setting up service-level auth access points. this logic has been moved out of
// lib/service in order to facilitate better testing practices.
package accesspoint

import (
	"context"
	"slices"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/authclient"
	authservices "github.com/gravitational/teleport/lib/auth/services"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/observability/tracing"
)

// Config holds parameters used to configure a cache to
// serve as an auth access point for a teleport service.
type Config struct {
	// Context is the base context used to propagate closure to
	// cache components.
	Context context.Context
	// Setup is a function that takes cache configuration and
	// modifies it to support a specific teleport service.
	Setup cache.SetupConfigFn
	// CacheName identifies the cache in logs.
	CacheName []string
	// Events is true if cache should have the events system enabled.
	Events bool
	// Unstarted is true if the cache should not be started.
	Unstarted bool
	// MaxRetryPeriod is the max retry period between connection attempts
	// to auth.
	MaxRetryPeriod time.Duration
	// ProcessID is an optional identifier used to help disambiguate logs
	// when teleport performs in-memory reloads.
	ProcessID string
	// TracingProvider is the provider to be used for exporting
	// traces. No-op tracers will be used if no provider is set.
	TracingProvider *tracing.Provider
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Setup == nil {
		return trace.BadParameter("missing parameter Setup")
	}
	if len(c.CacheName) == 0 {
		return trace.BadParameter("missing parameter CacheName")
	}
	if c.Context == nil {
		c.Context = context.Background()
	}
	return nil
}

// NewCacheFromServices creates a new cache for a Teleport service that
// uses the provided services to populate its resource collections.
func NewCacheFromServices(services *authservices.Services, cfg Config) (*cache.Cache, error) {
	cacheCfg, err := baseConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cacheCfg.Events = services.Events
	cacheCfg.ClusterConfig = services.ClusterConfiguration
	cacheCfg.Provisioner = services.Provisioner
	cacheCfg.Trust = services.TrustInternal
	cacheCfg.Users = services.Identity
	cacheCfg.Access = services.Access
	cacheCfg.DynamicAccess = services.DynamicAccessExt
	cacheCfg.Presence = services.PresenceInternal
	cacheCfg.Restrictions = services.Restrictions
	cacheCfg.Apps = services.Apps
	cacheCfg.Kubernetes = services.Kubernetes
	cacheCfg.CrownJewels = services.CrownJewels
	cacheCfg.DatabaseServices = services.DatabaseServices
	cacheCfg.Databases = services.Databases
	cacheCfg.DatabaseObjects = services.DatabaseObjects
	cacheCfg.AppSession = services.Identity
	cacheCfg.SnowflakeSession = services.Identity
	cacheCfg.SAMLIdPSession = services.Identity
	cacheCfg.WindowsDesktops = services.WindowsDesktops
	cacheCfg.SAMLIdPServiceProviders = services.SAMLIdPServiceProviders
	cacheCfg.UserGroups = services.UserGroups
	cacheCfg.Notifications = services.Notifications
	cacheCfg.Okta = services.Okta
	cacheCfg.AccessLists = services.AccessLists
	cacheCfg.AccessMonitoringRules = services.AccessMonitoringRules
	cacheCfg.SecReports = services.SecReports
	cacheCfg.UserLoginStates = services.UserLoginStates
	cacheCfg.Integrations = services.Integrations
	cacheCfg.DiscoveryConfigs = services.DiscoveryConfigs
	cacheCfg.WebSession = services.Identity.WebSessions()
	cacheCfg.WebToken = services.Identity.WebTokens()
	cacheCfg.KubeWaitingContainers = services.KubeWaitingContainer

	return cache.New(cfg.Setup(*cacheCfg))
}

// NewCacheFromClient creates a new cache for a teleport service that
// uses an authclient.ClientI to populate its resource collections.
func NewCacheFromClient(client authclient.ClientI, cfg Config) (*cache.Cache, error) {
	cacheCfg, err := baseConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cacheCfg.Events = client
	cacheCfg.ClusterConfig = client
	cacheCfg.Provisioner = client
	cacheCfg.Trust = client
	cacheCfg.Users = client
	cacheCfg.Access = client
	cacheCfg.DynamicAccess = client
	cacheCfg.Presence = client
	cacheCfg.Restrictions = client
	cacheCfg.Apps = client
	cacheCfg.Kubernetes = client
	cacheCfg.CrownJewels = client.CrownJewelServiceClient()
	cacheCfg.DatabaseServices = client
	cacheCfg.Databases = client
	cacheCfg.DatabaseObjects = client.DatabaseObjectsClient()
	cacheCfg.AppSession = client
	cacheCfg.SnowflakeSession = client
	cacheCfg.SAMLIdPSession = client
	cacheCfg.WindowsDesktops = client
	cacheCfg.SAMLIdPServiceProviders = client
	cacheCfg.UserGroups = client
	cacheCfg.Notifications = client
	cacheCfg.Okta = client.OktaClient()
	cacheCfg.AccessLists = client.AccessListClient()
	cacheCfg.AccessMonitoringRules = client.AccessMonitoringRuleClient()
	cacheCfg.SecReports = client.SecReportsClient()
	cacheCfg.UserLoginStates = client.UserLoginStateClient()
	cacheCfg.Integrations = client
	cacheCfg.DiscoveryConfigs = client.DiscoveryConfigClient()
	cacheCfg.WebSession = client.WebSessions()
	cacheCfg.WebToken = client.WebTokens()
	cacheCfg.KubeWaitingContainers = client

	return cache.New(cfg.Setup(*cacheCfg))
}

func baseConfig(cfg Config) (*cache.Config, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Creating in-memory backend for %v.", cfg.CacheName)
	mem, err := memory.New(memory.Config{
		Context:   cfg.Context,
		EventsOff: !cfg.Events,
		Mirror:    true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var tracer oteltrace.Tracer
	if cfg.TracingProvider != nil {
		tracer = cfg.TracingProvider.Tracer(teleport.ComponentCache)
	}
	reporter, err := backend.NewReporter(backend.ReporterConfig{
		Component: teleport.ComponentCache,
		Backend:   mem,
		Tracer:    tracer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	component := slices.Clone(cfg.CacheName)
	if cfg.ProcessID != "" {
		component = append(component, cfg.ProcessID)
	}

	component = append(component, teleport.ComponentCache)
	metricComponent := append(slices.Clone(cfg.CacheName), teleport.ComponentCache)

	return &cache.Config{
		Context:         cfg.Context,
		Backend:         reporter,
		Component:       teleport.Component(component...),
		MetricComponent: teleport.Component(metricComponent...),
		Tracer:          tracer,
		MaxRetryPeriod:  cfg.MaxRetryPeriod,
		Unstarted:       cfg.Unstarted,
	}, nil
}
