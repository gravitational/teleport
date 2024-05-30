/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package services

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// AccessMonitoringRules is the AccessMonitoringRule service
type AccessMonitoringRules interface {
	CreateAccessMonitoringRule(ctx context.Context, in *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error)
	UpdateAccessMonitoringRule(ctx context.Context, in *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error)
	UpsertAccessMonitoringRule(ctx context.Context, in *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error)
	GetAccessMonitoringRule(ctx context.Context, name string) (*accessmonitoringrulesv1.AccessMonitoringRule, error)
	DeleteAccessMonitoringRule(ctx context.Context, name string) error
	DeleteAllAccessMonitoringRules(ctx context.Context) error
	ListAccessMonitoringRules(ctx context.Context, limit int, startKey string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
	ListAccessMonitoringRulesWithFilter(ctx context.Context, pageSize int, nextToken string, subjects []string, notificationName string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
}

// NewAccessMonitoringRuleWithLabels creates a new AccessMonitoringRule  with the given spec and labels.
func NewAccessMonitoringRuleWithLabels(name string, labels map[string]string, spec *accessmonitoringrulesv1.AccessMonitoringRuleSpec) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	amr := &accessmonitoringrulesv1.AccessMonitoringRule{
		Kind:    types.KindAccessMonitoringRule,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
			Labels:    labels,
		},
		Spec: spec,
	}

	err := ValidateAccessMonitoringRule(amr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return amr, nil
}

// ValidateAccessMonitoringRule checks that the provided access monitoring rule is valid.
func ValidateAccessMonitoringRule(accessMonitoringRule *accessmonitoringrulesv1.AccessMonitoringRule) error {
	if accessMonitoringRule.Kind != types.KindAccessMonitoringRule {
		return trace.BadParameter("invalid kind for access monitoring rule: %q", accessMonitoringRule.Kind)
	}
	if accessMonitoringRule.Metadata == nil {
		return trace.BadParameter("accessMonitoringRule metadata is missing")
	}
	if accessMonitoringRule.Version != types.V1 {
		return trace.BadParameter("accessMonitoringRule %q is not supported", accessMonitoringRule.Version)
	}
	if accessMonitoringRule.Spec == nil {
		return trace.BadParameter("accessMonitoringRule spec is missing")
	}

	if len(accessMonitoringRule.Spec.Subjects) == 0 {
		return trace.BadParameter("accessMonitoringRule subject is missing")
	}

	if accessMonitoringRule.Spec.Condition == "" {
		return trace.BadParameter("accessMonitoringRule condition is missing")
	}

	if accessMonitoringRule.Spec.Notification != nil && accessMonitoringRule.Spec.Notification.Name == "" {
		return trace.BadParameter("accessMonitoringRule notification plugin name is missing")
	}
	if hasAccessRequestAsSubject := slices.ContainsFunc(accessMonitoringRule.Spec.Subjects, func(subject string) bool {
		return subject == types.KindAccessRequest
	}); hasAccessRequestAsSubject && accessMonitoringRule.Spec.Notification == nil {
		return trace.BadParameter("accessMonitoringRule notification configuration must be set if subjects contain %q",
			types.KindAccessRequest)
	}

	return nil
}

// MarshalAccessMonitoringRule marshals AccessMonitoringRule resource to JSON.
func MarshalAccessMonitoringRule(accessMonitoringRule *accessmonitoringrulesv1.AccessMonitoringRule, opts ...MarshalOption) ([]byte, error) {
	return FastMarshalProtoResourceDeprecated(accessMonitoringRule, opts...)
}

// UnmarshalAccessMonitoringRule unmarshals the AccessMonitoringRule resource.
func UnmarshalAccessMonitoringRule(data []byte, opts ...MarshalOption) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	return FastUnmarshalProtoResourceDeprecated[*accessmonitoringrulesv1.AccessMonitoringRule](data, opts...)
}
