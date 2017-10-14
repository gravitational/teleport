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

var m = &sync.Mutex{}

var emptyRolesHandler = func() error {
	return nil
}

// SetEmptyRolesHandler sets the callback which is called when a new trusted
// cluster with empty roles is being created
func SetEmptyRolesHandler(fn func() error) {
	m.Lock()
	defer m.Unlock()
	emptyRolesHandler = fn
}

// EmptyRoles handler is called when a new trusted cluster with empty roles
// is being created
func EmptyRolesHandler() error {
	m.Lock()
	defer m.Unlock()
	return emptyRolesHandler()
}

var defaultAllowedLogins = func() []string {
	return []string{teleport.TraitInternalRoleVariable}
}

// SetDefaultAllowedLogins sets the function that returns default allowed
// logins for a new admin role
func SetDefaultAllowedLogins(fn func() []string) {
	m.Lock()
	defer m.Unlock()
	defaultAllowedLogins = fn
}

// DefaultAllowedLogins returns default allowed logins for a new admin role
func DefaultAllowedLogins() []string {
	m.Lock()
	defer m.Unlock()
	return defaultAllowedLogins()
}

var versionPrinter = func() {
	ver := fmt.Sprintf("Teleport v%s", teleport.Version)
	if teleport.Gitref != "" {
		ver = fmt.Sprintf("%s git:%s", ver, teleport.Gitref)
	}
	fmt.Println(ver)
}

// SetVersionPrinter sets the method that prints teleport version
func SetVersionPrinter(fn func()) {
	m.Lock()
	defer m.Unlock()
	versionPrinter = fn
}

// VersionPrinter prints teleport version
func VersionPrinter() {
	m.Lock()
	defer m.Unlock()
	versionPrinter()
}
