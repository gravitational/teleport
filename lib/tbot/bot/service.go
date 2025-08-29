/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package bot

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

// Service is a long-running sub-component of tbot.
type Service interface {
	// String returns a human-readable name for the service that can be used
	// in logging. It should identify the type of the service and any top
	// level configuration that could distinguish it from a same-type service.
	String() string
	// Run starts the service and blocks until the service exits. It should
	// return a nil error if the service exits successfully and an error
	// if it is unable to proceed. It should exit gracefully if the context
	// is canceled.
	Run(ctx context.Context) error
}

// OneShotService is a [Service] that offers a mode in which it runs a single
// time and then exits. This aligns with the `--oneshot` mode of tbot.
type OneShotService interface {
	Service
	// OneShot runs the service once and then exits. It should return a nil
	// error if the service exits successfully and an error if it is unable
	// to proceed. It should exit gracefully if the context is canceled.
	OneShot(ctx context.Context) error
}

// ServiceDependencies will be constructed by the bot and passed to each service
// constructor.
type ServiceDependencies struct {
	// Client that can be used to interact with the auth server.
	Client *apiclient.Client

	// Resolver that can be used to look up proxy addresses.
	Resolver reversetunnelclient.Resolver

	// Logger to which errors and messages can be written. It's the service's
	// responsibility to tag the logger with a component.
	Logger *slog.Logger

	// ProxyPinger can be used to ping the proxy or auth server to discover
	// connection information (e.g. whether TLS routing is enabled).
	ProxyPinger connection.ProxyPinger

	// IdentityGenerator can be used to generate "impersonated" identities.
	IdentityGenerator *identity.Generator

	// ClientBuilder can be used to build new API clients from impersonated
	// identities.
	ClientBuilder *client.Builder

	// BotIdentity is a function that can be called to get the bot's internal
	// identity.
	BotIdentity func() *identity.Identity

	// BotIdentityReadyCh is a channel on which the service can receive to block
	// until the bot's internal identity has been renewed for the first time.
	BotIdentityReadyCh <-chan struct{}

	// ReloadCh is a channel on which the service can receive notifications that
	// it's time to reload their certificates (e.g. following a CA rotation).
	ReloadCh <-chan struct{}

	// StatusRegistry is the registry the service can register itself with to
	// report service health.
	StatusRegistry *readyz.Registry
}

// LoggerForService returns a logger with the service's name as its component.
func (deps ServiceDependencies) LoggerForService(svc Service) *slog.Logger {
	return deps.Logger.With(
		teleport.ComponentKey,
		teleport.Component(teleport.ComponentTBot, "svc", svc.String()),
	)
}

// ServiceBuilder will be called by the bot to create a service.
type ServiceBuilder func(ServiceDependencies) (Service, error)

// ServicePair combines two related Services.
type ServicePair struct{ primary, secondary Service }

// NewServicePair combines two related Services, the "primary" and its supporting
// "secondary" service.
func NewServicePair(primary, secondary Service) *ServicePair {
	return &ServicePair{
		primary:   primary,
		secondary: secondary,
	}
}

// String calls the primary service's String method.
func (s *ServicePair) String() string {
	return s.primary.String()
}

// Run the services in-parallel.
func (s *ServicePair) Run(ctx context.Context) error {
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return s.primary.Run(groupCtx)
	})
	group.Go(func() error {
		return s.secondary.Run(groupCtx)
	})
	return group.Wait()
}

// OneShotServicePair combines two related OneShotServices.
type OneShotServicePair struct {
	*ServicePair

	primary, secondary OneShotService
}

// NewOneShotServicePair combines two related OneShotServices, the "primary" and
// its supporting "secondary" service.
func NewOneShotServicePair(primary, secondary OneShotService) *OneShotServicePair {
	return &OneShotServicePair{
		ServicePair: NewServicePair(primary, secondary),
		primary:     primary,
		secondary:   secondary,
	}
}

// OneShot runs the services' OneShot methods in-parallel.
func (s *OneShotServicePair) OneShot(ctx context.Context) error {
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return s.primary.OneShot(groupCtx)
	})
	group.Go(func() error {
		return s.secondary.OneShot(groupCtx)
	})
	return group.Wait()
}

// LiteralService create a ServiceBuilder that returns the service as-is.
func LiteralService(service Service) ServiceBuilder {
	return func(ServiceDependencies) (Service, error) {
		return service, nil
	}
}

// NewNopService returns a service with the given name that does nothing at all.
func NewNopService(name string) NopService {
	return NopService{name: name}
}

// NopService is a service that does nothing at all.
type NopService struct{ name string }

func (n NopService) String() string { return n.name }

// Run blocks until the given context is canceled.
func (NopService) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// OneShot returns nil immediately.
func (NopService) OneShot(context.Context) error { return nil }
