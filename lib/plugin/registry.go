/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugin

import "github.com/gravitational/trace"

// Plugin describes interfaces of the teleport core plugin
type Plugin interface {
	// GetName returns plugin name
	GetName() string
	// RegisterProxyWebHandlers registers new methods with the ProxyWebHandler
	RegisterProxyWebHandlers(handler interface{}) error
	// RegisterAuthWebHandlers registers new methods with the Auth Web Handler
	RegisterAuthWebHandlers(service interface{}) error
	// RegisterAuthServices registers new services on the AuthServer
	RegisterAuthServices(server interface{}) error
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
	RegisterAuthServices(server interface{}) error
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

// RegisterAuthServices registers Teleport AuthServer services
func (r *registry) RegisterAuthServices(server interface{}) error {
	for _, p := range r.plugins {
		if err := p.RegisterAuthServices(server); err != nil {
			return trace.Wrap(err, "plugin %v failed to register", p.GetName())
		}
	}

	return nil
}
