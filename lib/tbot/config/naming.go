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
)

// reservedServiceNames are the service names reserved for internal use.
var reservedServiceNames = []string{
	"ca-rotation",
	"crl-cache",
	"heartbeat",
	"identity",
	"spiffe-trust-bundle-cache",
}

var reservedServiceNamesMap = func() map[string]struct{} {
	m := make(map[string]struct{}, len(reservedServiceNames))
	for _, k := range reservedServiceNames {
		m[k] = struct{}{}
	}
	return m
}()

var invalidServiceNameRegex = regexp.MustCompile(`[^a-z\d_\-+]`)

type serviceNamer struct {
	usedNames          map[string]struct{}
	countByServiceType map[string]int
}

func newServiceNamer() *serviceNamer {
	return &serviceNamer{
		usedNames:          make(map[string]struct{}),
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
		if _, ok := n.usedNames[name]; ok {
			return "", trace.BadParameter("service name %q conflicts with an automatically generated service name", name)
		}
	} else {
		if _, ok := n.usedNames[name]; ok {
			return "", trace.BadParameter("service name %q used more than once", name)
		}
		if _, ok := reservedServiceNamesMap[name]; ok {
			return "", trace.BadParameter("service name %q is reserved for internal use", name)
		}
		if invalidServiceNameRegex.MatchString(name) {
			return "", trace.BadParameter("invalid service name: %q, may only contain lowercase letters, numbers, hyphens, underscores, or plus symbols", name)
		}
	}

	n.usedNames[name] = struct{}{}
	return name, nil
}
