package services

import (
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

func AppendIdentityCenterMatchers(matchers []RoleMatcher, resource types.ResourceWithLabels) ([]RoleMatcher, error) {
	log := slog.With("kind", resource.GetKind(), "name", resource.GetName())
	switch resource.GetKind() {
	case types.KindIdentityCenterAccount:
		asmt, ok := resource.(Resource153Adapter[IdentityCenterAccount])
		if !ok {
			log.Error("Unexpected underlying resource type",
				"type", fmt.Sprintf("%T", resource))
			return matchers, trace.BadParameter("Unexpected resource type %T", resource)
		}
		matchers = append(matchers, &identityCenterAccountMatcher{
			account: asmt.Inner.Spec.Id,
		})

	case types.KindIdentityCenterAccountAssignment:
		asmt, ok := resource.(Resource153Adapter[IdentityCenterAccountAssignment])
		if !ok {
			log.Error("Unexpected underlying resource type",
				"type", fmt.Sprintf("%T", resource))
			return matchers, trace.BadParameter("Unexpected resource type %T", resource)
		}
		matchers = append(matchers, &identityCenterAccountAssignmentMatcher{
			account:       asmt.Inner.Spec.AccountId,
			permissionSet: asmt.Inner.Spec.PermissionSet.Arn,
		})
	}

	return matchers, nil
}

type identityCenterAccountMatcher struct {
	account string
}

func (m *identityCenterAccountMatcher) Match(role types.Role, rct types.RoleConditionType) (bool, error) {
	log := slog.With("matcher", "account")

	log.Warn("Checking role access",
		slog.Group("target", "account", m.account),
		"role", role.GetName(),
		"condition", rct)

	roleAssignments := role.GetAccountAssignments(rct)
	for _, roleAssignment := range roleAssignments {
		log.Debug("role has", "account", roleAssignment.AccountID)
		if roleAssignment.AccountID == m.account {
			log.Info("matched")
			return true, nil

		}
	}
	return false, nil
}

type identityCenterAccountAssignmentMatcher struct {
	account       string
	permissionSet string
}

func (m *identityCenterAccountAssignmentMatcher) Match(role types.Role, rct types.RoleConditionType) (bool, error) {
	log := slog.With("matcher", "account-assignment")

	log.Warn("Checking role access",
		slog.Group("target", "account", m.account, "permission_set", m.permissionSet),
		"role", role.GetName(),
		"condition", rct)

	roleAssignments := role.GetAccountAssignments(rct)
	for _, roleAssignment := range roleAssignments {
		log.Debug("role has", "account", roleAssignment.AccountID, "permission_set", roleAssignment.PermissionSet)

		if roleAssignment.AccountID == m.account && roleAssignment.PermissionSet == m.permissionSet {
			log.Info("match")
			return true, nil
		}
	}
	return false, nil
}
