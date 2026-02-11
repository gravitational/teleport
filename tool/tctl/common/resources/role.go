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

package resources

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

func NewRoleCollection(roles []types.Role) Collection {
	return &roleCollection{roles: roles}
}

type roleCollection struct {
	roles []types.Role
}

func (r *roleCollection) Resources() (res []types.Resource) {
	for _, resource := range r.roles {
		res = append(res, resource)
	}
	return res
}

func (r *roleCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, r := range r.roles {
		if r.GetName() == constants.DefaultImplicitRole {
			continue
		}
		rows = append(rows, []string{
			r.GetMetadata().Name,
			strings.Join(r.GetLogins(types.Allow), ","),
			printNodeLabels(r.GetNodeLabels(types.Allow)),
			printActions(r.GetRules(types.Allow)),
		})
	}

	headers := []string{"Role", "Allowed to login as", "Node Labels", "Access to resources"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Access to resources")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func printActions(rules []types.Rule) string {
	pairs := make([]string, 0, len(rules))
	for _, rule := range rules {
		pairs = append(pairs, fmt.Sprintf("%v:%v", strings.Join(rule.Resources, ","), strings.Join(rule.Verbs, ",")))
	}
	return strings.Join(pairs, ",")
}

func printNodeLabels(labels types.Labels) string {
	pairs := []string{}
	for key, values := range labels {
		if key == types.Wildcard {
			return "<all nodes>"
		}
		pairs = append(pairs, fmt.Sprintf("%v=%v", key, values))
	}
	return strings.Join(pairs, ",")
}

func roleHandler() Handler {
	return Handler{
		getHandler:    getRole,
		createHandler: createRole,
		updateHandler: updateRole,
		deleteHandler: deleteRole,
		singleton:     false,
		mfaRequired:   false,
		description:   "A set of permissions that can be granted to a user.",
	}
}

func getRole(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		roles, err := client.GetRoles(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &roleCollection{roles: roles}, nil
	}
	role, err := client.GetRole(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	warnAboutDynamicLabelsInDenyRule(ctx, slog.Default(), role)
	return &roleCollection{roles: []types.Role{role}}, nil

}

func createRole(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	role, err := services.UnmarshalRole(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := services.ValidateAccessPredicates(role); err != nil {
		// check for syntax errors in predicates
		return trace.Wrap(err)
	}
	err = services.CheckDynamicLabelsInDenyRules(role)
	if trace.IsBadParameter(err) {
		return trace.BadParameter("%s", dynamicLabelWarningMessage(role))
	} else if err != nil {
		return trace.Wrap(err)
	}

	warnAboutKubernetesResources(ctx, slog.Default(), role)

	roleName := role.GetName()
	_, err = client.GetRole(ctx, roleName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	roleExists := (err == nil)
	if roleExists && !opts.Force {
		return trace.AlreadyExists("role %q already exists", roleName)
	}
	if _, err := client.UpsertRole(ctx, role); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("role %q has been %s\n", roleName, upsertVerb(roleExists, opts.Force))
	return nil
}

func updateRole(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	role, err := services.UnmarshalRole(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := services.ValidateAccessPredicates(role); err != nil {
		// check for syntax errors in predicates
		return trace.Wrap(err)
	}

	warnAboutKubernetesResources(ctx, slog.Default(), role)
	warnAboutDynamicLabelsInDenyRule(ctx, slog.Default(), role)

	if _, err := client.UpdateRole(ctx, role); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("role %q has been updated\n", role.GetName())
	return nil
}

// warnAboutKubernetesResources warns about kubernetes resources
// if kubernetes_labels are set but kubernetes_resources are not.
func warnAboutKubernetesResources(ctx context.Context, logger *slog.Logger, r types.Role) {
	role, ok := r.(*types.RoleV6)
	// only warn about kubernetes resources for v6 roles
	if !ok || role.Version != types.V6 {
		return
	}
	if len(role.Spec.Allow.KubernetesLabels) > 0 && len(role.Spec.Allow.KubernetesResources) == 0 {
		logger.WarnContext(ctx, "role has allow.kubernetes_labels set but no allow.kubernetes_resources, this is probably a mistake - Teleport will restrict access to pods", "role", role.Metadata.Name)
	}
	if len(role.Spec.Allow.KubernetesLabels) == 0 && len(role.Spec.Allow.KubernetesResources) > 0 {
		logger.WarnContext(ctx, "role has allow.kubernetes_resources set but no allow.kubernetes_labels, this is probably a mistake - kubernetes_resources won't be effective", "role", role.Metadata.Name)
	}

	if len(role.Spec.Deny.KubernetesLabels) > 0 && len(role.Spec.Deny.KubernetesResources) > 0 {
		logger.WarnContext(ctx, "role has deny.kubernetes_labels set but also has deny.kubernetes_resources set, this is probably a mistake - deny.kubernetes_resources won't be effective", "role", role.Metadata.Name)
	}
}

func deleteRole(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteRole(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("role %q has been deleted\n", ref.Name)
	return nil
}

func dynamicLabelWarningMessage(r types.Role) string {
	return fmt.Sprintf("existing role %q has labels with the %q prefix in its deny rules. This is not recommended due to the volatility of %q labels and is not allowed for new roles",
		r.GetName(), types.TeleportDynamicLabelPrefix, types.TeleportDynamicLabelPrefix)
}

// warnAboutDynamicLabelsInDenyRule warns about using dynamic/ labels in deny
// rules. Only applies to existing roles as adding dynamic/ labels to deny
// rules in a new role is not allowed.
func warnAboutDynamicLabelsInDenyRule(ctx context.Context, logger *slog.Logger, r types.Role) {
	if err := services.CheckDynamicLabelsInDenyRules(r); err == nil {
		return
	} else if trace.IsBadParameter(err) {
		logger.WarnContext(ctx, "existing role has labels with the a dynamic prefix in its deny rules, this is not recommended due to the volatility of dynamic labels and is not allowed for new roles", "role", r.GetName())
	} else {
		logger.WarnContext(ctx, "error checking deny rules labels", "error", err)
	}
}
