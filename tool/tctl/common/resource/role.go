package resource

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getRole(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		roles, err := client.GetRoles(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewRoleCollection(roles), nil
	}
	role, err := client.GetRole(ctx, rc.ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	warnAboutDynamicLabelsInDenyRule(ctx, rc.config.Logger, role)
	return collections.NewRoleCollection([]types.Role{role}), nil
}

// createRole implements `tctl create role.yaml` command.
func (rc *ResourceCommand) createRole(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
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

	warnAboutKubernetesResources(ctx, rc.config.Logger, role)

	roleName := role.GetName()
	_, err = client.GetRole(ctx, roleName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	roleExists := (err == nil)
	if roleExists && !rc.IsForced() {
		return trace.AlreadyExists("role %q already exists", roleName)
	}
	if _, err := client.UpsertRole(ctx, role); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("role %q has been %s\n", roleName, UpsertVerb(roleExists, rc.IsForced()))
	return nil
}

func (rc *ResourceCommand) updateRole(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	role, err := services.UnmarshalRole(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := services.ValidateAccessPredicates(role); err != nil {
		// check for syntax errors in predicates
		return trace.Wrap(err)
	}

	warnAboutKubernetesResources(ctx, rc.config.Logger, role)
	warnAboutDynamicLabelsInDenyRule(ctx, rc.config.Logger, role)

	if _, err := client.UpdateRole(ctx, role); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("role %q has been updated\n", role.GetName())
	return nil
}

func (rc *ResourceCommand) deleteRole(ctx context.Context, client *authclient.Client) error {
	if err := client.DeleteRole(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("role %q has been deleted\n", rc.ref.Name)
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
