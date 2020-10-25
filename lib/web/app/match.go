/*
Copyright 2020 Gravitational, Inc.

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

package app

import (
	"context"
	"math/rand"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// Match will match an application with the passed in matcher function. Matcher
// functions that can match on public address and name are available.
//
// Note that in the situation multiple applications match, a random selection
// is returned. This is done on purpose to support HA to allow multiple
// application proxy nodes to be run and if one is down, at least the
// application can be accessible on the other.
//
// In the future this function should be updated to keep state on application
// servers that are down and to not route requests to that server.
func Match(ctx context.Context, authClient appGetter, fn Matcher) (*services.App, services.Server, error) {
	servers, err := authClient.GetAppServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var am []*services.App
	var sm []services.Server

	for _, server := range servers {
		for _, app := range server.GetApps() {
			if fn(app) {
				am = append(am, app)
				sm = append(sm, server)
			}
		}
	}

	if len(am) == 0 {
		return nil, nil, trace.NotFound("failed to match application")
	}
	index := rand.Intn(len(am))
	return am[index], sm[index], nil
}

// Matcher allows matching on different properties of an application.
type Matcher func(*services.App) bool

// MatchPublicAddr matches on the public address of an application.
func MatchPublicAddr(publicAddr string) Matcher {
	return func(app *services.App) bool {
		return app.PublicAddr == publicAddr
	}
}

// MatchName matches on the name of an application.
func MatchName(name string) Matcher {
	return func(app *services.App) bool {
		return app.Name == name
	}
}

type appGetter interface {
	GetAppServers(context.Context, string, ...services.MarshalOption) ([]services.Server, error)
}
