// Teleport
// Copyright (C) 2026  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/services"
)

// ScopedHandlers returns a map of ScopedHandler per kind for resource types that
// require scope-qualified names. Resource types that also support classic (unscoped)
// access can register in both Handlers() and ScopedHandlers().
func ScopedHandlers() map[string]ScopedHandler {
	return map[string]ScopedHandler{
		types.KindNode:                        serverScopedHandler(),
		types.KindWorkloadIdentity:            workloadIdentityScopedHandler(),
		scopedaccess.KindScopedRole:           scopedRoleScopedHandler(),
		types.KindScopedToken:                 scopedTokenScopedHandler(),
		scopedaccess.KindScopedRoleAssignment: scopedRoleAssignmentScopedHandler(),
	}
}

// ScopedHandler represents a scoped resource supported by tctl resource commands.
// Get and delete operations for scoped handlers require an explicit scope-qualified
// name to disambiguate types in different scopes.
//
// Create and update retain the classic [services.UnknownResource] signature because
// the scope travels in the resource itself, not in the CLI args.
type ScopedHandler struct {
	// getHandler powers "tctl get <kind>[/<subkind>] <scope>::<name>" and "tctl get <kind>"
	// (list all). A nil qn lists all resources; a non-nil qn fetches a single resource.
	getHandler func(ctx context.Context, client *authclient.Client, subKind string, sqn *scopes.QualifiedName, opts GetOpts) (Collection, error)
	// deleteHandler powers "tctl rm <kind>[/<subkind>] <scope>::<name>". The SQN is always
	// non-nil at call time. subKind is empty when the user omits the sub-kind segment.
	deleteHandler func(ctx context.Context, client *authclient.Client, subKind string, sqn scopes.QualifiedName) error
	// createHandler powers "tctl create". Scope comes from the resource.
	createHandler func(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error
	// updateHandler powers "tctl edit". Scope comes from the resource.
	updateHandler func(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error
	// description is a resource description used by "tctl list-kinds".
	description string
	// mfaRequired informs "tctl get" whether the resource is read sensitive.
	mfaRequired bool
}

// Get queries the cluster for a scoped resource collection.
// A nil sqn lists all resources of this kind.
// A non-nil sqn returns only the resource matching that scope and name.
func (h *ScopedHandler) Get(ctx context.Context, clt *authclient.Client, subKind string, sqn *scopes.QualifiedName, opts GetOpts) (Collection, error) {
	if h.getHandler == nil {
		return nil, trace.NotImplemented("resource does not support 'tctl get'")
	}
	return h.getHandler(ctx, clt, subKind, sqn, opts)
}

// Create takes a raw resource manifest and creates the resource.
func (h *ScopedHandler) Create(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	if h.createHandler == nil {
		return trace.NotImplemented("resource does not support 'tctl create'")
	}
	return h.createHandler(ctx, clt, raw, opts)
}

// Update takes a raw resource manifest and updates the resource.
func (h *ScopedHandler) Update(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	if h.updateHandler == nil {
		return trace.NotImplemented("resource does not have an update handler")
	}
	return h.updateHandler(ctx, clt, raw, opts)
}

// Delete deletes a scoped resource by its qualified name.
// subKind is required for resource types that have sub-kinds; pass "" otherwise.
func (h *ScopedHandler) Delete(ctx context.Context, clt *authclient.Client, subKind string, sqn scopes.QualifiedName) error {
	if h.deleteHandler == nil {
		return trace.NotImplemented("resource does not support 'tctl rm'")
	}
	return h.deleteHandler(ctx, clt, subKind, sqn)
}

// MFARequired indicates that this resource requires MFA to Get.
func (h *ScopedHandler) MFARequired() bool {
	return h.mfaRequired
}

// Description returns the description of the handler's resource.
func (h *ScopedHandler) Description() string {
	return h.description
}

// SupportedCommands returns the list of tctl commands this handler supports.
func (h *ScopedHandler) SupportedCommands() []string {
	var verbs []string
	if h.getHandler != nil {
		verbs = append(verbs, "get")
	}
	if h.createHandler != nil {
		verbs = append(verbs, "create")
	}
	if h.deleteHandler != nil {
		verbs = append(verbs, "rm")
	}
	// No check on the update handler for the "update" command because it is not
	// doing anything useful today: https://github.com/gravitational/teleport/issues/61381

	return verbs
}

// rejectSubKind returns an error for resource types that don't support sub-kinds.
// The error message also covers the case where the user may have intended the segment
// as a resource name, directing them toward the scope-qualified name form.
func rejectSubKind(kind, subKind string) error {
	return trace.BadParameter(
		"resource type %q does not support sub-kinds (got %q)\n"+
			"hint: if %q was intended as a resource name, provide it as a scope-qualified name:\n"+
			"  tctl get %s <scope>::%s",
		kind, subKind, subKind, kind, subKind,
	)
}

// scopeMismatchNotFound builds a NotFound error for the pre-namespacing case where a
// resource was found by name but lives in a different scope than requested. This mirrors
// the behavior that namespacing will make automatic: from the perspective of the requested
// scope, the resource does not exist.
//
// Note: this check is client-side and inherently racy as the resource's scope could change
// between the lookup and the caller's subsequent action. It is a temporary measure to
// prevent obviously misleading results until the backend APIs are updated to accept
// scope-qualified identifiers natively and enforce the scope constraint server-side.
func scopeMismatchNotFound(kind string, requested scopes.QualifiedName, actualScope string) error {
	var hint string
	if actualScope == "" {
		// Resource is unscoped; the SQN form is not valid for unscoped resources.
		hint = fmt.Sprintf("tctl get %s/%s", kind, requested.Name)
	} else {
		hint = fmt.Sprintf("tctl get %s %s::%s", kind, actualScope, requested.Name)
	}
	return trace.NotFound(
		"%s %q not found in scope %q (a %s with that name exists in scope %q, try: %s)",
		kind, requested.Name, requested.Scope,
		kind, actualScope,
		hint,
	)
}
