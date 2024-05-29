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
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/services"
)

// AccessCacheConfig holds parameters used to confiure a cache to
// serve as an auth access point for a teleport service.
type AccessCacheConfig struct {
	// Context is the base context used to propagate closure to
	// cache components.
	Context context.Context
	// Services is a collection of upstream services from which
	// the access cache will derive its state.
	Services services.Services
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

func (c *AccessCacheConfig) CheckAndSetDefaults() error {
	if c.Services == nil {
		return trace.BadParameter("missing parameter Services")
	}
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

// NewAccessCache builds a cache.Cache instance for a teleport service. This logic has been
// broken out of lib/service in order to support easier unit testing of process components.
func NewAccessCache(cfg AccessCacheConfig) (*cache.Cache, error) {
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

	return cache.New(cfg.Setup(cache.Config{
		Context:                 cfg.Context,
		Backend:                 reporter,
		Events:                  cfg.Services,
		ClusterConfig:           cfg.Services,
		Provisioner:             cfg.Services,
		Trust:                   cfg.Services,
		Users:                   cfg.Services,
		Access:                  cfg.Services,
		DynamicAccess:           cfg.Services,
		Presence:                cfg.Services,
		Restrictions:            cfg.Services,
		Apps:                    cfg.Services,
		Kubernetes:              cfg.Services,
		DatabaseServices:        cfg.Services,
		Databases:               cfg.Services,
		AppSession:              cfg.Services,
		SnowflakeSession:        cfg.Services,
		SAMLIdPSession:          cfg.Services,
		WindowsDesktops:         cfg.Services,
		SAMLIdPServiceProviders: cfg.Services,
		UserGroups:              cfg.Services,
		Okta:                    cfg.Services.OktaClient(),
		AccessLists:             cfg.Services.AccessListClient(),
		AccessMonitoringRules:   cfg.Services.AccessMonitoringRuleClient(),
		SecReports:              cfg.Services.SecReportsClient(),
		UserLoginStates:         cfg.Services.UserLoginStateClient(),
		Integrations:            cfg.Services,
		DiscoveryConfigs:        cfg.Services.DiscoveryConfigClient(),
		WebSession:              cfg.Services.WebSessions(),
		WebToken:                cfg.Services.WebTokens(),
		KubeWaitingContainers:   cfg.Services,
		Component:               teleport.Component(component...),
		MetricComponent:         teleport.Component(metricComponent...),
		Tracer:                  tracer,
		MaxRetryPeriod:          cfg.MaxRetryPeriod,
		Unstarted:               cfg.Unstarted,
	}))
}
