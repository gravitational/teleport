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

package plugin

import (
	"context"
	"crypto/tls"

	"github.com/gravitational/trace"
)

type getCertFunc = func() (*tls.Certificate, error)

// Plugin describes interfaces of the teleport core plugin
type Plugin interface {
	// GetName returns plugin name
	GetName() string
	// RegisterProxyWebHandlers registers new methods with the ProxyWebHandler
	RegisterProxyWebHandlers(handler interface{}) error
	// RegisterAuthWebHandlers registers new methods with the Auth Web Handler
	RegisterAuthWebHandlers(service interface{}) error
	// RegisterAuthServices registers new services on the AuthServer
	RegisterAuthServices(ctx context.Context, server any, getClientCert getCertFunc) error
}

// Registry is the plugin registry
type Registry interface {
	// IsRegistered returns whether a plugin with the give name exists.
	IsRegistered(name string) bool
	// Add adds plugin to the registry
	Add(plugin Plugin) error
	// RegisterProxyWebHandlers registers Teleport Proxy web handlers
	RegisterProxyWebHandlers(handler interface{}) error
	// RegisterAuthWebHandlers registers Teleport Auth web handlers
	RegisterAuthWebHandlers(handler interface{}) error
	// RegisterAuthServices registers Teleport AuthServer services
	RegisterAuthServices(ctx context.Context, server any, getClientCert getCertFunc) error
}

// NewRegistry creates an instance of the Registry
func NewRegistry() Registry {
	return &registry{
		plugins: make(map[string]Plugin),
	}
}

type registry struct {
	plugins map[string]Plugin
}

// IsRegistered returns whether a plugin with the give name exists.
func (r *registry) IsRegistered(name string) bool {
	_, ok := r.plugins[name]
	return ok
}

// Add adds plugin to the plugin registry
func (r *registry) Add(p Plugin) error {
	if p == nil {
		return trace.BadParameter("missing plugin")
	}

	name := p.GetName()
	if name == "" {
		return trace.BadParameter("missing plugin name")
	}

	if r.IsRegistered(name) {
		return trace.AlreadyExists("plugin %v already exists", name)
	}

	r.plugins[name] = p

	return nil
}

// RegisterProxyWebHandlers registers Teleport Proxy web handlers
func (r *registry) RegisterProxyWebHandlers(handler interface{}) error {
	for _, p := range r.plugins {
		if err := p.RegisterProxyWebHandlers(handler); err != nil {
			return trace.Wrap(err, "plugin %v failed to register", p.GetName())
		}
	}

	return nil
}

// RegisterAuthWebHandlers registers Teleport Auth web handlers
func (r *registry) RegisterAuthWebHandlers(handler interface{}) error {
	for _, p := range r.plugins {
		if err := p.RegisterAuthWebHandlers(handler); err != nil {
			return trace.Wrap(err, "plugin %v failed to register", p.GetName())
		}
	}

	return nil
}

func (r *registry) RegisterAuthServices(ctx context.Context, server any, getClientCert getCertFunc) error {
	for _, p := range r.plugins {
		if err := p.RegisterAuthServices(ctx, server, getClientCert); err != nil {
			return trace.Wrap(err, "plugin %v failed to register", p.GetName())
		}
	}

	return nil
}
