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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/accessmonitoring"
	"github.com/gravitational/teleport/lib/utils/typical"
)

var (
	// accessRequestConditionParser is a parser for the access request condition.
	// It is used to validate access monitoring rules before write operations.
	accessRequestConditionParser = mustNewAccessRequestConditionParser()
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
	ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
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
		return trace.BadParameter("accessMonitoringRule version %q is not supported", accessMonitoringRule.Version)
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

	if err := validateSchedules(accessMonitoringRule.GetSpec().GetSchedules()); err != nil {
		return trace.Wrap(err, "validating spec.schedules")
	}

	if accessMonitoringRule.Spec.Notification != nil && accessMonitoringRule.Spec.Notification.Name == "" {
		return trace.BadParameter("accessMonitoringRule notification plugin name is missing")
	}

	if automaticReview := accessMonitoringRule.GetSpec().GetAutomaticReview(); automaticReview != nil {
		if automaticReview.GetIntegration() == "" {
			return trace.BadParameter("accessMonitoringRule automatic_review integration is missing")
		}

		switch automaticReview.GetDecision() {
		case types.RequestState_APPROVED.String(), types.RequestState_DENIED.String():
		case "":
			return trace.BadParameter("accessMonitoringRule automatic_review decision is missing")
		default:
			return trace.BadParameter("accessMonitoringRule automatic_review decision %q is not supported", automaticReview.GetDecision())
		}
	}

	if slices.Contains(accessMonitoringRule.GetSpec().GetSubjects(), types.KindAccessRequest) {
		_, err := accessRequestConditionParser.Parse(accessMonitoringRule.GetSpec().GetCondition())
		if err != nil {
			return trace.BadParameter("accessMonitoringRule condition is invalid: %s", err.Error())
		}

		desiredState := accessMonitoringRule.GetSpec().GetDesiredState()
		switch desiredState {
		case "", types.AccessMonitoringRuleStateReviewed:
		default:
			return trace.BadParameter("accessMonitoringRule desired_state %q is not supported", desiredState)
		}

		if accessMonitoringRule.GetSpec().GetNotification() != nil {
			return nil
		}
		if accessMonitoringRule.GetSpec().GetAutomaticReview() != nil {
			return nil
		}
		return trace.BadParameter("one of notification or automatic_review must be configured if subjects contain %q",
			types.KindAccessRequest)
	}

	return nil
}

func validateSchedules(schedules map[string]*accessmonitoringrulesv1.Schedule) error {
	for name, schedule := range schedules {
		if schedule.GetTime() == nil {
			return trace.BadParameter("spec.schedules[%s].time is required", name)
		}

		if err := validateTimeSchedule(schedule.GetTime()); err != nil {
			return trace.Wrap(err, "validating spec.schedules[%s].time", name)
		}
	}
	return nil
}

func validateTimeSchedule(schedule *accessmonitoringrulesv1.TimeSchedule) error {
	if len(schedule.GetShifts()) == 0 {
		return trace.BadParameter("at least one shift is required")
	}

	for _, shift := range schedule.GetShifts() {
		if err := validateShift(shift); err != nil {
			return trace.Wrap(err, "shift is invalid")
		}
	}
	return nil
}

func validateShift(shift *accessmonitoringrulesv1.TimeSchedule_Shift) error {
	if _, ok := types.ParseWeekday(shift.GetWeekday()); !ok {
		return trace.BadParameter("failed to parse weekday: %v", shift.GetWeekday())
	}

	start, err := clockTime(time.Time{}, shift.GetStart())
	if err != nil {
		return trace.Wrap(err, "invalid start time")
	}

	end, err := clockTime(time.Time{}, shift.GetEnd())
	if err != nil {
		return trace.Wrap(err, "invalid end time")
	}

	if !start.Before(end) {
		return trace.BadParameter("start time must be before end time")
	}
	return nil
}

func clockTime(timestamp time.Time, hourMinute string) (time.Time, error) {
	const hourMinuteFormat = "15:04" // 24-hour HH:MM format

	parsed, err := time.ParseInLocation(hourMinuteFormat, hourMinute, timestamp.Location())
	if err != nil {
		return time.Time{}, trace.Wrap(err)
	}

	return time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(),
		parsed.Hour(), parsed.Minute(), 0, 0, timestamp.Location()), nil
}

// MarshalAccessMonitoringRule marshals AccessMonitoringRule resource to JSON.
func MarshalAccessMonitoringRule(accessMonitoringRule *accessmonitoringrulesv1.AccessMonitoringRule, opts ...MarshalOption) ([]byte, error) {
	return FastMarshalProtoResourceDeprecated(accessMonitoringRule, opts...)
}

// UnmarshalAccessMonitoringRule unmarshals the AccessMonitoringRule resource.
func UnmarshalAccessMonitoringRule(data []byte, opts ...MarshalOption) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	return FastUnmarshalProtoResourceDeprecated[*accessmonitoringrulesv1.AccessMonitoringRule](data, opts...)
}

// MatchAccessMonitoringRule returns true if the provided rule matches the provided match fields.
// The match fields are optional. If a match field is not provided, then the
// rule matches any value for that field.
func MatchAccessMonitoringRule(rule *accessmonitoringrulesv1.AccessMonitoringRule, subjects []string, notificationIntegration, automaticReviewIntegration string) bool {
	if notificationIntegration != "" {
		if rule.GetSpec().GetNotification().GetName() != notificationIntegration {
			return false
		}
	}
	if automaticReviewIntegration != "" {
		if rule.GetSpec().GetAutomaticReview().GetIntegration() != automaticReviewIntegration {
			return false
		}
	}
	for _, subject := range subjects {
		if ok := slices.ContainsFunc(rule.Spec.Subjects, func(s string) bool {
			return s == subject
		}); !ok {
			return false
		}
	}
	return true
}

func mustNewAccessRequestConditionParser() *typical.Parser[accessmonitoring.AccessRequestExpressionEnv, any] {
	parser, err := accessmonitoring.NewAccessRequestConditionParser()
	if err != nil {
		panic(err)
	}
	return parser
}
