package web

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/teleport/lib/web"
)

// listRootScopedRolesHandler is the handler for GET /enterprise/rootscopedroles.
// It lists scoped roles defined in the root scope, which are precisely the
// scoped roles that can be granted by access lists.
//
// Query params:
// - limit: max number of results
// - startKey: pagination cursor
// - filter: case-insensitive substring match on role name
func (p *Plugin) listRootScopedRolesHandler(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *web.SessionContext) (any, error) {
	if err := p.ScopesFeatures.AssertEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	values := r.URL.Query()
	return listRootScopedRoles(r.Context(), clt.ScopedAccessServiceClient(), values)
}

type scopedRoleLister interface {
	ListScopedRoles(context.Context, *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error)
}

func listRootScopedRoles(ctx context.Context, clt scopedRoleLister, values url.Values) (*ui.ListScopedRolesResponse, error) {
	limit, err := web.QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.ListScopedRoles(ctx, scopedaccessv1.ListScopedRolesRequest_builder{
		PageSize:  limit,
		PageToken: values.Get("startKey"),
		ResourceScope: scopesv1.Filter_builder{
			Scope: scopes.Root,
			Mode:  scopesv1.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
		}.Build(),
		NameFilter: values.Get("filter"),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newUIListScopedRolesResponse(resp), nil
}

func newUIListScopedRolesResponse(resp *scopedaccessv1.ListScopedRolesResponse) *ui.ListScopedRolesResponse {
	return &ui.ListScopedRolesResponse{
		Roles:    slices.Map(resp.GetRoles(), newUIScopedRoleListItem),
		StartKey: resp.GetNextPageToken(),
	}
}

func newUIScopedRoleListItem(scopedRole *scopedaccessv1.ScopedRole) ui.ScopedRoleListItem {
	return ui.ScopedRoleListItem{
		Name:             scopedRole.GetMetadata().GetName(),
		Scope:            scopedRole.GetScope(),
		AssignableScopes: scopedRole.GetSpec().GetAssignableScopes(),
	}
}
