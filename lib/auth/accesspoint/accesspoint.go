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

// Package accesspoint provides helpers for configuring caches in the context of
// setting up service-level auth access points. this logic has been moved out of
// lib/service in order to facilitate better testing practices.
package accesspoint

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/services"
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
	// EventsSystem is true if cache should have the events system enabled.
	EventsSystem bool
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

	// The following services are provided to the Cache to allow it to
	// populate its resource collections. They will either be the local service
	// directly or a client that can be used to fetch the resources from the
	// remote service.

	Access                  services.Access
	AccessLists             services.AccessLists
	AccessMonitoringRules   services.AccessMonitoringRules
	AppSession              services.AppSession
	Apps                    services.Apps
	ClusterConfig           services.ClusterConfiguration
	CrownJewels             services.CrownJewels
	DatabaseObjects         services.DatabaseObjects
	DatabaseServices        services.DatabaseServices
	Databases               services.Databases
	DiscoveryConfigs        services.DiscoveryConfigs
	DynamicAccess           services.DynamicAccessCore
	Events                  types.Events
	Integrations            services.Integrations
	KubeWaitingContainers   services.KubeWaitingContainer
	Kubernetes              services.Kubernetes
	Notifications           services.Notifications
	Okta                    services.Okta
	Presence                services.Presence
	Provisioner             services.Provisioner
	Restrictions            services.Restrictions
	SAMLIdPServiceProviders services.SAMLIdPServiceProviders
	SecReports              services.SecReports
	SnowflakeSession        services.SnowflakeSession
	SPIFFEFederations       services.SPIFFEFederations
	StaticHostUsers         services.StaticHostUser
	Trust                   services.Trust
	UserGroups              services.UserGroups
	UserTasks               services.UserTasks
	UserLoginStates         services.UserLoginStates
	Users                   services.UsersService
	WebSession              types.WebSessionInterface
	WebToken                types.WebTokenInterface
	WorkloadIdentity        services.WorkloadIdentities
	DynamicWindowsDesktops  services.DynamicWindowsDesktops
	WindowsDesktops         services.WindowsDesktops
	AutoUpdateService       services.AutoUpdateServiceGetter
	ProvisioningStates      services.ProvisioningStates
	IdentityCenter          services.IdentityCenter
	PluginStaticCredentials services.PluginStaticCredentials
	GitServers              services.GitServers
	HealthCheckConfig       services.HealthCheckConfigReader
	RecordingEncryption     services.RecordingEncryption
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

func NewCache(cfg Config) (*cache.Cache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	slog.DebugContext(cfg.Context, "Creating in-memory backend cache.", "cache_name", cfg.CacheName)
	mem, err := memory.New(memory.Config{
		Context:   cfg.Context,
		EventsOff: !cfg.EventsSystem,
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

	cacheCfg := cache.Config{
		Context:         cfg.Context,
		Backend:         reporter,
		Component:       teleport.Component(component...),
		MetricComponent: teleport.Component(metricComponent...),
		Tracer:          tracer,
		MaxRetryPeriod:  cfg.MaxRetryPeriod,
		Unstarted:       cfg.Unstarted,

		Access:                  cfg.Access,
		AccessLists:             cfg.AccessLists,
		AccessMonitoringRules:   cfg.AccessMonitoringRules,
		AppSession:              cfg.AppSession,
		Apps:                    cfg.Apps,
		ClusterConfig:           cfg.ClusterConfig,
		AutoUpdateService:       cfg.AutoUpdateService,
		CrownJewels:             cfg.CrownJewels,
		DatabaseObjects:         cfg.DatabaseObjects,
		DatabaseServices:        cfg.DatabaseServices,
		Databases:               cfg.Databases,
		DiscoveryConfigs:        cfg.DiscoveryConfigs,
		DynamicAccess:           cfg.DynamicAccess,
		Events:                  cfg.Events,
		Integrations:            cfg.Integrations,
		KubeWaitingContainers:   cfg.KubeWaitingContainers,
		Kubernetes:              cfg.Kubernetes,
		Notifications:           cfg.Notifications,
		Okta:                    cfg.Okta,
		Presence:                cfg.Presence,
		Provisioner:             cfg.Provisioner,
		Restrictions:            cfg.Restrictions,
		SAMLIdPServiceProviders: cfg.SAMLIdPServiceProviders,
		SecReports:              cfg.SecReports,
		SnowflakeSession:        cfg.SnowflakeSession,
		SPIFFEFederations:       cfg.SPIFFEFederations,
		StaticHostUsers:         cfg.StaticHostUsers,
		Trust:                   cfg.Trust,
		UserGroups:              cfg.UserGroups,
		UserLoginStates:         cfg.UserLoginStates,
		UserTasks:               cfg.UserTasks,
		Users:                   cfg.Users,
		WebSession:              cfg.WebSession,
		WebToken:                cfg.WebToken,
		WorkloadIdentity:        cfg.WorkloadIdentity,
		WindowsDesktops:         cfg.WindowsDesktops,
		DynamicWindowsDesktops:  cfg.DynamicWindowsDesktops,
		ProvisioningStates:      cfg.ProvisioningStates,
		IdentityCenter:          cfg.IdentityCenter,
		PluginStaticCredentials: cfg.PluginStaticCredentials,
		GitServers:              cfg.GitServers,
		HealthCheckConfig:       cfg.HealthCheckConfig,
		RecordingEncryption:     cfg.RecordingEncryption,
	}

	return cache.New(cfg.Setup(cacheCfg))
}
