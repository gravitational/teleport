/*
Copyright 2015 Gravitational, Inc.

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
package srv

import (
	"strings"

	"github.com/gravitational/teleport/lib/auth"
)

// resolver is an interface implementing a query resolver,
// it get's a query and resolves it into a list of address strings
type resolver interface {
	resolve(query string) ([]string, error)
}

// backend resolver is a simple implementation of the resolver
// that uses servers presence information to find the servers
type backendResolver struct {
	authService auth.AccessPoint
}

// resolve provides a simple demo resolve functionality,such as globbing, expanding to all hosts
// or accepting a list of hosts:port pairs
func (b *backendResolver) resolve(query string) ([]string, error) {
	// simply expand the query to all known hosts
	if query == "*" {
		out := []string{}
		srvs, err := b.authService.GetServers()
		if err != nil {
			return nil, err
		}
		for _, s := range srvs {
			out = append(out, s.Addr)
		}
		return out, nil
	}
	// else treat it as a list of hosts
	return strings.Split(query, ","), nil
}
