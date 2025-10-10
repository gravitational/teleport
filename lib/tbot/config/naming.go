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

package config

import (
	"fmt"
	"regexp"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/set"
)

// reservedServiceNames are the service names reserved for internal use.
var reservedServiceNames = set.New(
	"ca-rotation",
	"crl-cache",
	"heartbeat",
	"identity",
	"spiffe-trust-bundle-cache",
)

var invalidServiceNameRegex = regexp.MustCompile(`[^a-z\d_\-+]`)

type serviceNamer struct {
	usedNames          set.Set[string]
	countByServiceType map[string]int
}

func newServiceNamer() *serviceNamer {
	return &serviceNamer{
		usedNames:          set.New[string](),
		countByServiceType: make(map[string]int),
	}
}

// pickName checks the user-chosen name is valid (e.g. not reserved for internal
// use or containing illegal characters) if one is given. If no name is given,
// it will generate a name based on the service type with a counter suffix.
func (n *serviceNamer) pickName(serviceType, name string) (string, error) {
	n.countByServiceType[serviceType]++

	if name == "" {
		name = fmt.Sprintf(
			"%s-%d",
			invalidServiceNameRegex.ReplaceAllString(serviceType, "-"),
			n.countByServiceType[serviceType],
		)
		if n.usedNames.Contains(name) {
			return "", trace.BadParameter("service name %q conflicts with an automatically generated service name", name)
		}
	} else {
		if n.usedNames.Contains(name) {
			return "", trace.BadParameter("service name %q used more than once", name)
		}
		if reservedServiceNames.Contains(name) {
			return "", trace.BadParameter("service name %q is reserved for internal use", name)
		}
		if invalidServiceNameRegex.MatchString(name) {
			return "", trace.BadParameter("invalid service name: %q, may only contain lowercase letters, numbers, hyphens, underscores, or plus symbols", name)
		}
	}

	n.usedNames.Add(name)
	return name, nil
}
