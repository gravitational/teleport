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

// package modules allows external packages override certain behavioral
// aspects of teleport
package modules

import (
	"fmt"
	"sync"

	"github.com/gravitational/teleport"
)

// Modules defines interface that external libraries can implement customizing
// default teleport behavior
type Modules interface {
	// EmptyRoles handler is called when a new trusted cluster with empty roles
	// is being created
	EmptyRolesHandler() error
	// DefaultAllowedLogins returns default allowed logins for a new admin role
	DefaultAllowedLogins() []string
	// PrintVersion prints teleport version
	PrintVersion()
	// RolesFromLogins returns roles for external user based on the logins
	// extracted from the connector
	RolesFromLogins([]string) []string
	// TraitsFromLogins returns traits for external user based on the logins
	// extracted from the connector
	TraitsFromLogins([]string) map[string][]string
}

// SetModules sets the modules interface
func SetModules(m Modules) {
	mutex.Lock()
	defer mutex.Unlock()
	modules = m
}

// GetModules returns the modules interface
func GetModules() Modules {
	mutex.Lock()
	defer mutex.Unlock()
	return modules
}

type defaultModules struct{}

// EmptyRolesHandler is called when a new trusted cluster with empty roles
// is created, no-op by default
func (p *defaultModules) EmptyRolesHandler() error {
	return nil
}

// DefaultAllowedLogins returns allowed logins for a new admin role
func (p *defaultModules) DefaultAllowedLogins() []string {
	return []string{teleport.TraitInternalRoleVariable}
}

// PrintVersion prints teleport version
func (p *defaultModules) PrintVersion() {
	ver := fmt.Sprintf("Teleport v%s", teleport.Version)
	if teleport.Gitref != "" {
		ver = fmt.Sprintf("%s git:%s", ver, teleport.Gitref)
	}
	fmt.Println(ver)
}

// RolesFromLogins returns roles for external user based on the logins
// extracted from the connector
//
// By default there is only one role, "admin", so logins are ignored and
// instead used as allowed logins (see TraitsFromLogins below).
func (p *defaultModules) RolesFromLogins(logins []string) []string {
	return []string{teleport.AdminRoleName}
}

// TraitsFromLogins returns traits for external user based on the logins
// extracted from the connector
//
// By default logins are treated as allowed logins user traits.
func (p *defaultModules) TraitsFromLogins(logins []string) map[string][]string {
	return map[string][]string{
		teleport.TraitLogins: logins,
	}
}

var (
	mutex           = &sync.Mutex{}
	modules Modules = &defaultModules{}
)
