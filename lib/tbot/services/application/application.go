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

package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot/services/application")

// validateAppName validates an application service's app_name field. In scope
// mode the name must be a strongly valid scope-qualified name
// ("<scope>::<name>"); otherwise it must be a plain, non-qualified app name.
func validateAppName(name string, scoped bool) error {
	if scoped {
		sqn, err := scopes.ParseQualifiedName(name)
		if err != nil {
			return trace.BadParameter("app_name: %v", err)
		}

		if err := sqn.StrongValidate(); err != nil {
			return trace.BadParameter("app_name: %v", err)
		}

		return nil
	}

	qn, err := scopes.ParseOptionallyQualifiedName(name)
	if err != nil || qn.Scope != "" {
		return trace.BadParameter("app_name: can not be a scope-qualified name when not in scope mode")
	}

	return nil
}

// getApp finds the first matching app for the scope-qualified name
//
// Includes a predicate filter on resource.scope which is only available in v19+.
func getApp(
	ctx context.Context,
	client apiclient.GetResourcesClient,
	qn scopes.QualifiedName,
) (types.Application, error) {
	ctx, span := tracer.Start(ctx, "getApp")
	defer span.End()

	apps, err := getMatchingApps(ctx, client, fmt.Sprintf(`name == %q && resource.scope == %q`, qn.Name, qn.Scope))
	if err != nil {
		return nil, trace.Wrap(err, "name: %q", qn.String())
	}

	return apps[0], nil
}

// getAppLegacy must be called by unscoped tbots in v18 instead of getApp.
//
// In v19+ getApp should always be used since the resource.scope field exists
func getAppLegacy(
	ctx context.Context,
	client apiclient.GetResourcesClient,
	name string,
) (types.Application, error) {
	ctx, span := tracer.Start(ctx, "getAppLegacy")
	defer span.End()

	// An unscoped tbot is authorized to retrieve resources inside scopes.
	// This means to retrieve the unscoped app we should filter with
	// `resource.scope == ""`. However, we cannot refer to resource.scope in the
	// predicate otherwise it will fail to evaluate on legacy auth servers.
	// Instead we query on just the name and then post filter out scoped apps.
	//
	// TODO (peterwillis): in v19+ we can filter by resource.scope
	// so this predicate can be include `resource.scope == ""` and only
	// unscoped apps will be matched.
	apps, err := getMatchingApps(ctx, client, fmt.Sprintf(`name == %q`, name))
	if err != nil {
		return nil, trace.Wrap(err, "name: %q", name)
	}

	apps = slices.DeleteFunc(apps, func(app types.Application) bool {
		return app.GetScope() != ""
	})
	if len(apps) == 0 {
		return nil, trace.Wrap(trace.NotFound("matching app not found"), "name: %q", name)
	}

	return apps[0], nil
}

func getMatchingApps(
	ctx context.Context,
	client apiclient.GetResourcesClient,
	predicate string,
) ([]types.Application, error) {
	ctx, span := tracer.Start(ctx, "getMatchingApps")
	defer span.End()

	servers, err := apiclient.GetAllResources[types.AppServer](ctx, client, &proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindAppServer,
		PredicateExpression: predicate,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	apps := make([]types.Application, 0, len(servers))
	for _, s := range servers {
		apps = append(apps, s.GetApp())
	}

	apps = types.DeduplicateApps(apps)

	if len(apps) == 0 {
		return nil, trace.NotFound("matching app not found")
	}

	return apps, nil
}
