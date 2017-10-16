/*
Copyright 2017 Gravitational, Inc.

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

// package plugins allows external packages override certain behavioral
// aspects of teleport
package plugins

import (
	"fmt"
	"sync"

	"github.com/gravitational/teleport"
)

// Plugins defines interface that external libraries can implement customizing
// default teleport behavior
type Plugins interface {
	// EmptyRoles handler is called when a new trusted cluster with empty roles
	// is being created
	EmptyRolesHandler() error
	// DefaultAllowedLogins returns default allowed logins for a new admin role
	DefaultAllowedLogins() []string
	// PrintVersion prints teleport version
	PrintVersion()
}

// SetPlugins sets the plugins interface
func SetPlugins(p Plugins) {
	m.Lock()
	defer m.Unlock()
	plugins = p
}

// GetPlugins returns the plugins interface
func GetPlugins() Plugins {
	m.Lock()
	defer m.Unlock()
	return plugins
}

type defaultPlugins struct{}

// EmptyRolesHandler is called when a new trusted cluster with empty roles
// is created, no-op by default
func (p *defaultPlugins) EmptyRolesHandler() error {
	return nil
}

// DefaultAllowedLogins returns allowed logins for a new admin role
func (p *defaultPlugins) DefaultAllowedLogins() []string {
	return []string{teleport.TraitInternalRoleVariable}
}

// PrintVersion prints teleport version
func (p *defaultPlugins) PrintVersion() {
	ver := fmt.Sprintf("Teleport v%s", teleport.Version)
	if teleport.Gitref != "" {
		ver = fmt.Sprintf("%s git:%s", ver, teleport.Gitref)
	}
	fmt.Println(ver)
}

var (
	m               = &sync.Mutex{}
	plugins Plugins = &defaultPlugins{}
)
