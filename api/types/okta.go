/*
Copyright 2023 Gravitational, Inc.

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

package types

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types/compare"
	"github.com/gravitational/teleport/api/utils"
)

var _ compare.IsEqual[OktaAssignment] = (*OktaAssignmentV1)(nil)

// OktaImportRule specifies a rule for importing and labeling Okta applications and groups.
type OktaImportRule interface {
	ResourceWithLabels

	// GetPriority will return the priority of the Okta import rule.
	GetPriority() int32

	// GetMappings will return the list of mappings for the Okta import rule.
	GetMappings() []OktaImportRuleMapping
}

// NewOktaImportRule returns a new OktaImportRule.
func NewOktaImportRule(metadata Metadata, spec OktaImportRuleSpecV1) (OktaImportRule, error) {
	o := &OktaImportRuleV1{
		ResourceHeader: ResourceHeader{
			Metadata: metadata,
		},
		Spec: spec,
	}
	if err := o.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return o, nil
}

// GetPriority will return the priority of the Okta import rule.
func (o *OktaImportRuleV1) GetPriority() int32 {
	return o.Spec.Priority
}

// GetMappings will return the list of mappings for the Okta import rule.
func (o *OktaImportRuleV1) GetMappings() []OktaImportRuleMapping {
	matches := make([]OktaImportRuleMapping, len(o.Spec.Mappings))

	for i, match := range o.Spec.Mappings {
		matches[i] = match
	}

	return matches
}

// String returns the Okta import rule string representation.
func (o *OktaImportRuleV1) String() string {
	return fmt.Sprintf("OktaImportRuleV1(Name=%v, Labels=%v)",
		o.GetName(), o.GetAllLabels())
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (o *OktaImportRuleV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(o.GetAllLabels()), o.GetName())
	return MatchSearch(fieldVals, values, nil)
}

// setStaticFields sets static resource header and metadata fields.
func (o *OktaImportRuleV1) setStaticFields() {
	o.Kind = KindOktaImportRule
	o.Version = V1
}

// CheckAndSetDefaults checks and sets default values
func (o *OktaImportRuleV1) CheckAndSetDefaults() error {
	o.setStaticFields()
	if err := o.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if o.Spec.Priority < 0 {
		return trace.BadParameter("priority must be a positive number")
	}

	if len(o.Spec.Mappings) == 0 {
		return trace.BadParameter("mappings is empty")
	}

	for _, mapping := range o.Spec.Mappings {
		if err := mapping.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// OktaImportRuleMapping is a list of matches that map match rules to labels.
type OktaImportRuleMapping interface {
	// GetMatches returns all matches for a mapping.
	GetMatches() []OktaImportRuleMatch
	// GetAddLabels returns the labels that will be added for a mapping.
	GetAddLabels() map[string]string
}

// GetMatches returns all matches for a mapping.
func (o *OktaImportRuleMappingV1) GetMatches() []OktaImportRuleMatch {
	matches := make([]OktaImportRuleMatch, len(o.Match))

	for i, match := range o.Match {
		matches[i] = match
	}

	return matches
}

// GetAddLabels returns the labels that will be added for a mapping.
func (o *OktaImportRuleMappingV1) GetAddLabels() map[string]string {
	return o.AddLabels
}

// CheckAndSetDefaults checks and sets default values
func (o *OktaImportRuleMappingV1) CheckAndSetDefaults() error {
	for _, match := range o.Match {
		if err := match.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// OktaImportRuleMatch creates a new Okta import rule match.
type OktaImportRuleMatch interface {
	// GetAppIDs returns whether or not this match contains an App ID match and, if so, the list of app IDs.
	GetAppIDs() (bool, []string)
	// GetGroupIDs returns whether or not this match contains a Group ID match and, if so, the list of app IDs.
	GetGroupIDs() (bool, []string)
	// GetAppNameRegexes returns whether or not this match contains app name regexes and, if so, the regexes.
	GetAppNameRegexes() (bool, []string)
	// GetGroupNameRegexes returns whether or not this match contains group name regexes and, if so, the regexes.
	GetGroupNameRegexes() (bool, []string)
}

// GetAppIDs returns whether or not this match contains an App ID match and, if so, the list of app IDs.
func (o *OktaImportRuleMatchV1) GetAppIDs() (bool, []string) {
	return len(o.AppIDs) > 0, o.AppIDs
}

// GetGroupIDs returns whether or not this match contains a Group ID match and, if so, the list of app IDs.
func (o *OktaImportRuleMatchV1) GetGroupIDs() (bool, []string) {
	return len(o.GroupIDs) > 0, o.GroupIDs
}

// GetAppNameRegexes returns whether or not this match contains app name regexes and, if so, the regexes.
func (o *OktaImportRuleMatchV1) GetAppNameRegexes() (bool, []string) {
	return len(o.AppNameRegexes) > 0, o.AppNameRegexes
}

// GetGroupNameRegexes returns whether or not this match contains group name regexes and, if so, the regexes.
func (o *OktaImportRuleMatchV1) GetGroupNameRegexes() (bool, []string) {
	return len(o.GroupNameRegexes) > 0, o.GroupNameRegexes
}

// CheckAndSetDefaults checks and sets default values
func (o *OktaImportRuleMatchV1) CheckAndSetDefaults() error {
	if len(o.AppIDs) > 0 && len(o.GroupIDs) > 0 {
		return trace.BadParameter("only one of App IDs or Group IDs can be set")
	}

	return nil
}

// OktaAssignment is a representation of an action or set of actions taken by Teleport to assign Okta users
// to applications or groups. When modifying this object, please make sure to update
// tool/tctl/common/oktaassignment to reflect any new fields that were added.
type OktaAssignment interface {
	ResourceWithLabels

	// SetMetadata will set the metadata for the Okta assignment.
	SetMetadata(metadata Metadata)
	// GetUser will return the user that the Okta assignment actions applies to.
	GetUser() string
	// GetTargets will return the list of targets that will be assigned as part of this assignment.
	GetTargets() []OktaAssignmentTarget
	// GetCleanupTime will return the optional time that the assignment should be cleaned up.
	GetCleanupTime() time.Time
	// SetCleanupTime will set the cleanup time.
	SetCleanupTime(time.Time)
	// GetStatus gets the status of the assignment.
	GetStatus() string
	// SetStatus sets the status of the eassignment. Only allows valid transitions.
	SetStatus(status string) error
	// SetLastTransition sets the last transition time.
	SetLastTransition(time.Time)
	// GetLastTransition returns the time that the action last transitioned.
	GetLastTransition() time.Time
	// IsFinalized returns the finalized state.
	IsFinalized() bool
	// SetFinalized sets the finalized state
	SetFinalized(bool)
	// Copy returns a copy of this Okta assignment resource.
	Copy() OktaAssignment
}

// NewOktaAssignment creates a new Okta assignment object.
func NewOktaAssignment(metadata Metadata, spec OktaAssignmentSpecV1) (OktaAssignment, error) {
	o := &OktaAssignmentV1{
		ResourceHeader: ResourceHeader{
			Metadata: metadata,
		},
		Spec: spec,
	}
	if err := o.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return o, nil
}

// SetMetadata will set the metadata for the Okta assignment.
func (o *OktaAssignmentV1) SetMetadata(metadata Metadata) {
	o.Metadata = metadata
}

// GetUser returns the user that the actions will be applied to.
func (o *OktaAssignmentV1) GetUser() string {
	return o.Spec.User
}

// GetTargets returns the targets associated with the Okta assignment.
func (o *OktaAssignmentV1) GetTargets() []OktaAssignmentTarget {
	targets := make([]OktaAssignmentTarget, len(o.Spec.Targets))

	for i, target := range o.Spec.Targets {
		targets[i] = target
	}

	return targets
}

// GetCleanupTime will return the optional time that the assignment should be cleaned up.
func (o *OktaAssignmentV1) GetCleanupTime() time.Time {
	return o.Spec.CleanupTime
}

// SetCleanupTime will set the cleanup time.
func (o *OktaAssignmentV1) SetCleanupTime(cleanupTime time.Time) {
	o.Spec.CleanupTime = cleanupTime.UTC()
}

// GetStatus gets the status of the assignment.
func (o *OktaAssignmentV1) GetStatus() string {
	switch o.Spec.Status {
	case OktaAssignmentSpecV1_PENDING:
		return constants.OktaAssignmentStatusPending
	case OktaAssignmentSpecV1_PROCESSING:
		return constants.OktaAssignmentStatusProcessing
	case OktaAssignmentSpecV1_SUCCESSFUL:
		return constants.OktaAssignmentStatusSuccessful
	case OktaAssignmentSpecV1_FAILED:
		return constants.OktaAssignmentStatusFailed
	default:
		return constants.OktaAssignmentStatusUnknown
	}
}

// SetStatus sets the status of the eassignment. Only allows valid transitions.
//
// Valid transitions are:
// * PENDING -> (PROCESSING)
// * PROCESSING -> (SUCCESSFUL, FAILED, PROCESSING)
// * SUCCESSFUL -> (PROCESSING)
// * FAILED -> (PROCESSING)
func (o *OktaAssignmentV1) SetStatus(status string) error {
	invalidTransition := false
	switch o.Spec.Status {
	case OktaAssignmentSpecV1_PENDING:
		switch status {
		case constants.OktaAssignmentStatusProcessing:
		default:
			invalidTransition = true
		}
	case OktaAssignmentSpecV1_PROCESSING:
		switch status {
		case constants.OktaAssignmentStatusProcessing:
		case constants.OktaAssignmentStatusSuccessful:
		case constants.OktaAssignmentStatusFailed:
		default:
			invalidTransition = true
		}
	case OktaAssignmentSpecV1_SUCCESSFUL:
		switch status {
		case constants.OktaAssignmentStatusProcessing:
		default:
			invalidTransition = true
		}
	case OktaAssignmentSpecV1_FAILED:
		switch status {
		case constants.OktaAssignmentStatusProcessing:
		default:
			invalidTransition = true
		}
	case OktaAssignmentSpecV1_UNKNOWN:
		// All transitions are allowed from UNKNOWN.
	default:
		invalidTransition = true
	}

	if invalidTransition {
		return trace.BadParameter("invalid transition: %s -> %s", o.GetStatus(), status)
	}

	o.Spec.Status = OktaAssignmentStatusToProto(status)

	return nil
}

// SetLastTransition sets the last transition time.
func (o *OktaAssignmentV1) SetLastTransition(time time.Time) {
	o.Spec.LastTransition = time.UTC()
}

// GetLastTransition returns the optional time that the action last transitioned.
func (o *OktaAssignmentV1) GetLastTransition() time.Time {
	return o.Spec.LastTransition
}

// IsFinalized returns the finalized state.
func (o *OktaAssignmentV1) IsFinalized() bool {
	return o.Spec.Finalized
}

// SetFinalized sets the finalized state
func (o *OktaAssignmentV1) SetFinalized(finalized bool) {
	o.Spec.Finalized = finalized
}

// Copy returns a copy of this Okta assignment resource.
func (o *OktaAssignmentV1) Copy() OktaAssignment {
	return utils.CloneProtoMsg(o)
}

// String returns the Okta assignment rule string representation.
func (o *OktaAssignmentV1) String() string {
	return fmt.Sprintf("OktaAssignmentV1(Name=%v, Labels=%v)",
		o.GetName(), o.GetAllLabels())
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (o *OktaAssignmentV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(o.GetAllLabels()), o.GetName())
	return MatchSearch(fieldVals, values, nil)
}

// setStaticFields sets static resource header and metadata fields.
func (o *OktaAssignmentV1) setStaticFields() {
	o.Kind = KindOktaAssignment
	o.Version = V1
}

// CheckAndSetDefaults checks and sets default values
func (o *OktaAssignmentV1) CheckAndSetDefaults() error {
	o.setStaticFields()
	if err := o.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if o.Spec.User == "" {
		return trace.BadParameter("user must not be empty")
	}

	// Make sure the times are UTC so that Copy() works properly.
	o.Spec.CleanupTime = o.Spec.CleanupTime.UTC()
	o.Spec.LastTransition = o.Spec.LastTransition.UTC()

	return nil
}

// IsEqual determines if two okta assignment resources are equivalent to one another.
func (o *OktaAssignmentV1) IsEqual(i OktaAssignment) bool {
	if other, ok := i.(*OktaAssignmentV1); ok {
		return deriveTeleportEqualOktaAssignmentV1(o, other)
	}
	return false
}

// OktaAssignmentTarget is an target for an Okta assignment.
type OktaAssignmentTarget interface {
	// GetTargetType returns the target type.
	GetTargetType() string
	// GetID returns the ID of the target.
	GetID() string
}

// GetTargetType returns the target type.
func (o *OktaAssignmentTargetV1) GetTargetType() string {
	switch o.Type {
	case OktaAssignmentTargetV1_APPLICATION:
		return constants.OktaAssignmentTargetApplication
	case OktaAssignmentTargetV1_GROUP:
		return constants.OktaAssignmentTargetGroup
	default:
		return constants.OktaAssignmentTargetUnknown
	}
}

// GetID returns the ID of the action target.
func (o *OktaAssignmentTargetV1) GetID() string {
	return o.Id
}

// OktaAssignments is a list of OktaAssignment resources.
type OktaAssignments []OktaAssignment

// ToMap returns these Okta assignments as a map keyed by Okta assignment name.
func (o OktaAssignments) ToMap() map[string]OktaAssignment {
	m := make(map[string]OktaAssignment, len(o))
	for _, oktaAssignment := range o {
		m[oktaAssignment.GetName()] = oktaAssignment
	}
	return m
}

// AsResources returns these Okta assignments as resources with labels.
func (o OktaAssignments) AsResources() ResourcesWithLabels {
	resources := make(ResourcesWithLabels, 0, len(o))
	for _, oktaAssignment := range o {
		resources = append(resources, oktaAssignment)
	}
	return resources
}

// Len returns the slice length.
func (o OktaAssignments) Len() int { return len(o) }

// Less compares Okta assignments by name.
func (o OktaAssignments) Less(i, j int) bool { return o[i].GetName() < o[j].GetName() }

// Swap swaps two Okta assignments.
func (o OktaAssignments) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

// OktaAssignmentStatusToProto will convert the internal notion of an Okta status into the Okta status
// message understood by protobuf.
func OktaAssignmentStatusToProto(status string) OktaAssignmentSpecV1_OktaAssignmentStatus {
	switch status {
	case constants.OktaAssignmentStatusPending:
		return OktaAssignmentSpecV1_PENDING
	case constants.OktaAssignmentStatusProcessing:
		return OktaAssignmentSpecV1_PROCESSING
	case constants.OktaAssignmentStatusSuccessful:
		return OktaAssignmentSpecV1_SUCCESSFUL
	case constants.OktaAssignmentStatusFailed:
		return OktaAssignmentSpecV1_FAILED
	default:
		return OktaAssignmentSpecV1_UNKNOWN
	}
}

// OktaAssignmentStatusProtoToString will convert the Okta status known to protobuf into the internal notion
// of an Okta status.
func OktaAssignmentStatusProtoToString(status OktaAssignmentSpecV1_OktaAssignmentStatus) string {
	switch status {
	case OktaAssignmentSpecV1_PENDING:
		return constants.OktaAssignmentStatusPending
	case OktaAssignmentSpecV1_PROCESSING:
		return constants.OktaAssignmentStatusProcessing
	case OktaAssignmentSpecV1_SUCCESSFUL:
		return constants.OktaAssignmentStatusSuccessful
	case OktaAssignmentSpecV1_FAILED:
		return constants.OktaAssignmentStatusFailed
	default:
		return constants.OktaAssignmentStatusUnknown
	}
}

func (o *PluginOktaSettings) GetCredentialsInfo() *PluginOktaCredentialsInfo {
	if o == nil {
		return nil
	}
	return o.CredentialsInfo
}

func (o *PluginOktaSettings) GetSyncSettings() *PluginOktaSyncSettings {
	if o == nil {
		return nil
	}
	return o.SyncSettings
}

type OktaUserSyncSource string

const (
	// OktaUserSyncSourceUnknown indicates the user sync source is not set.
	OktaUserSyncSourceUnknown OktaUserSyncSource = "unknown"

	// OktaUserSyncSourceSamlApp indicates users are synchronized from Okta SAML app for the connector assignments.
	OktaUserSyncSourceSamlApp OktaUserSyncSource = "saml_app"

	// OktaUserSyncSourceSamlOrg indicates users are synchronized  Okta organization (legacy).
	OktaUserSyncSourceOrg OktaUserSyncSource = "org"
)

func (o *PluginOktaSyncSettings) GetUserSyncSource() OktaUserSyncSource {
	if o == nil {
		return OktaUserSyncSourceUnknown
	}
	switch v := OktaUserSyncSource(o.UserSyncSource); v {
	case "":
		return OktaUserSyncSourceUnknown
	case OktaUserSyncSourceUnknown, OktaUserSyncSourceSamlApp, OktaUserSyncSourceOrg:
		return v
	default:
		slog.ErrorContext(context.Background(), "Unhandled PluginOktaSyncSettings_UserSyncSource, returning OktaUserSyncSourceUnknown", "value", o.UserSyncSource)
		return OktaUserSyncSourceUnknown
	}
}

func (o *PluginOktaSyncSettings) SetUserSyncSource(source OktaUserSyncSource) {
	if o == nil {
		panic("calling (*PluginOktaSyncSettings).SetUserSyncSource on nil pointer")
	}
	switch source {
	case OktaUserSyncSourceUnknown, OktaUserSyncSourceSamlApp, OktaUserSyncSourceOrg:
		o.UserSyncSource = string(source)
	default:
		slog.ErrorContext(context.Background(), "Unhandled OktaUserSyncSource, not doing anything", "value", source)
	}
}
