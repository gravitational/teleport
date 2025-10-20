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
	// Clone returns a copy of the Okta import rule.
	Clone() OktaImportRule
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

// Clone returns a copy of the Okta import rule.
func (o *OktaImportRuleV1) Clone() OktaImportRule {
	return utils.CloneProtoMsg(o)
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
	// GetTargets will return the list of target applications and/or groups that will be
	// assigned as part of this assignment.
	GetTargets() []OktaAssignmentTarget
	// GetTargetsCnt returns the number of target applications and/or groups that will be
	// assigned as part of this assignment.
	GetTargetsCnt() int
	// GetCleanupTime will return the optional time that the assignment should be cleaned up.
	GetCleanupTime() time.Time
	// SetCleanupTime will set the cleanup time.
	SetCleanupTime(time.Time)
	// GetStatus gets the status (from the resource spec) of the assignment.
	GetStatus() string
	// SetStatus sets the status (from the resource spec) of the assignment. Only allows valid
	// transitions.
	SetStatus(status string) error
	// GetResourceStatus gets the actual status field of the resource.
	// TODO(kopiczko) replace GetStatus with OktaAssignmentStatus.Phase.
	GetResourceStatus() OktaAssignmentStatus
	// SetResourceStatus sets the actual status field of the resource.
	// TODO(kopiczko) replace SetStatus with OktaAssignmentStatus.Phase.
	SetResourceStatus(status OktaAssignmentStatus)
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

func ValidateOktaAssignment(a OktaAssignment) error {
	if a == nil {
		return nil
	}
	status := a.GetResourceStatus()
	err := status.validate(a.GetTargetsCnt())
	return trace.Wrap(err)
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

// GetTargetsCnt returns the number of target applications and/or groups that will be
// assigned as part of this assignment.
func (o *OktaAssignmentV1) GetTargetsCnt() int {
	return len(o.Spec.Targets)
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

func (o *OktaAssignmentV1) GetResourceStatus() OktaAssignmentStatus {
	return oktaAssignmentResourceStatusFromProto(o.Status)
}

func (o *OktaAssignmentV1) SetResourceStatus(status OktaAssignmentStatus) {
	o.Status = oktaAssignmentResourceStatusToProto(status)
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

type OktaAssignmentTargetType string
type OktaAssignmentPhase string
type OktaAssignmentTargetPhase string

const (
	// OktaAssignmentTargetTypeApplication is an application target of an Okta assignment.
	OktaAssignmentTargetTypeApplication OktaAssignmentTargetType = constants.OktaAssignmentTargetApplication
	// OktaAssignmentTargetTypeGroup is a group target of an Okta assignment.
	OktaAssignmentTargetTypeGroup OktaAssignmentTargetType = constants.OktaAssignmentTargetGroup

	// OktaAssignmentPhaseProcessed means that all the assignment targets are in either
	// "created", "imported", "cleared" or "referenced" phase.
	OktaAssignmentPhaseProcessed OktaAssignmentPhase = "processed"
	// OktaAssignmentPhaseFailed means that one or more of the assignment targets are in either
	// "error", "unauthorized" or "dangling" phase.
	OktaAssignmentPhaseFailed OktaAssignmentPhase = "failed"

	// OktaAssignmentTargetPhaseCreated means assignment for the target was successfully
	// created in Okta.
	OktaAssignmentTargetPhaseCreated OktaAssignmentTargetPhase = "created"
	// OktaAssignmentTargetPhaseImported means the assignment for the target is originated from
	// Okta (i.e. was created in Okta and not Teleport) and should not be provisioned by
	// Teleport to not overwrite Okta-side changes.
	OktaAssignmentTargetPhaseImported OktaAssignmentTargetPhase = "imported"
	// OktaAssignmentTargetPhaseCleared means assignment for the target was successfully
	// cleaned up in Okta.
	OktaAssignmentTargetPhaseCleared OktaAssignmentTargetPhase = "cleared"
	// OktaAssignmentTargetPhaseReferenced means assignment for the target was not cleared
	// because it's referenced by another okta_assignment resource.
	OktaAssignmentTargetPhaseReferenced OktaAssignmentTargetPhase = "referenced"
	// OktaAssignmentTargetPhaseError means there was an unexpected error during target
	// provisioning.
	OktaAssignmentTargetPhaseError OktaAssignmentTargetPhase = "error"
	// OktaAssignmentTargetPhaseUnauthorized means this target is not managed by Okta
	// integration or there was a failure establishing if it is managed by the Okta service.
	// This should not happen and probably indicates a bug.
	OktaAssignmentTargetPhaseUnauthorized OktaAssignmentTargetPhase = "unauthorized"
	// OktaAssignmentTargetPhaseDangling means that the corresponding Teleport resource
	// (app_server or user_group) was not found. The resource should be eventually synced back
	// by the App and Group sync.
	OktaAssignmentTargetPhaseDangling OktaAssignmentTargetPhase = "dangling"
)

// OktaAssignmentStatus is the status of the Okta assignment.
type OktaAssignmentStatus struct {
	// Phase of the assignment. It can be:
	// - "processed" - all the assignment targets are in either "created", "imported", "cleared" or
	//   "referenced" phase
	// - "failed" - one or more of the assignment targets are in either "error", "unauthorized" or
	//   "dangling" phase
	// TODO(kopiczko): replace spec.status and spec.finalized with this status.phase; this will most likely need a new "provisioning" phase
	Phase OktaAssignmentPhase `json:"phase,omitempty" yaml:"phase,omitempty"`
	// ProcessedAt is the time the resource was processed.
	ProcessedAt time.Time `json:"processed_at,omitempty" yaml:"processed_at,omitempty"`
	// Targets status information.
	Targets OktaAssignmentStatusTargets `json:"targets,omitempty" yaml:"targets,omitempty"`
}

func (s *OktaAssignmentStatus) validate(specTargetsCnt int) error {
	if s == nil {
		return nil
	}

	if s.Phase != "" {
		// Do not validate the phase itself so we can extend it in the future if needed.

		if s.ProcessedAt.IsZero() {
			return trace.BadParameter("status.processed_at: must be set when status.phase is set")
		}
	}

	if s.Targets.Stats.Total != 0 && s.Targets.Stats.Total != specTargetsCnt {
		return trace.BadParameter("status.targets.stats.total: [%d] not equal to len(spec.targets) [%d]",
			s.Targets.Stats.Total, specTargetsCnt)
	}

	// Leave to doors open to not store target statuses for successful targets in the future
	// (to optimize for space) so only compare rather than checking strict equality.
	if s.Targets.Stats.Total < len(s.Targets.Status) {
		return trace.BadParameter("status.targets.stats.total: [%d] smaller than len(status.targets.status) [%d]",
			s.Targets.Stats.Total, len(s.Targets.Status))
	}

	// Do not check strict equality to leave the doors open for introducing more stats.
	if s.Targets.Stats.Total < s.Targets.Stats.Provisioned+s.Targets.Stats.Failed {
		return trace.BadParameter("status.targets.stats: .total [%d] smaller than .provisioned [%d] + .failed [%d]",
			s.Targets.Stats.Total, s.Targets.Stats.Provisioned, s.Targets.Stats.Failed)
	}

	for i, ts := range s.Targets.Status {
		if ts.Type == "" {
			return trace.BadParameter("status.targets.status[%d].type: must be set", i)
		}

		if ts.ID == "" {
			return trace.BadParameter("status.targets.status[%d].id: must be set", i)
		}

		// No strict phase validation for the target so it can be extended.
		if ts.Phase == "" {
			return trace.BadParameter("status.targets.status[%d].phase: must be set", i)
		}

		if ts.ProcessedAt.IsZero() {
			return trace.BadParameter("status.targets.status[%d].processed_at: must be set", i)
		}

		if ts.ProcessedAt.After(s.ProcessedAt) {
			t1 := ts.ProcessedAt.UTC().Format(time.RFC3339Nano)
			t2 := s.ProcessedAt.UTC().Format(time.RFC3339Nano)
			return trace.BadParameter("status.targets.status[%d].processed_at: %q is after status.processed_at %q", i, t1, t2)
		}
	}

	return nil
}

type OktaAssignmentStatusTargets struct {
	// Stats of the targets.
	Stats OktaAssignmentStatusTargetsStats `json:"stats,omitempty" yaml:"stats,omitempty"`
	// Status is a list of individual targets' statuses.
	Status []OktaAssignmentStatusTargetStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type OktaAssignmentStatusTargetsStats struct {
	// Total is the number of all targets.
	Total int `json:"total,omitempty" yaml:"total,omitempty"`
	// Provisioned is the number of targets in "created" or "imported" phase.
	Provisioned int `json:"provisioned,omitempty" yaml:"provisioned,omitempty"`
	// Deprovisioned is the number of targets in "cleared" or "referenced" phase.
	Deprovisioned int `json:"deprovisioned,omitempty" yaml:"deprovisioned,omitempty"`
	// Failed is the number of targets in "error", "unauthorized" or "dangling" phase.
	Failed int `json:"failed,omitempty" yaml:"failed,omitempty"`
}

type OktaAssignmentStatusTargetStatus struct {
	// Type is the type of targeted Okta resource. Can be either "app" or "group".
	Type OktaAssignmentTargetType `json:"type,omitempty" yaml:"type,omitempty"`
	// ID is the ID of the targeted Okta resource.
	ID string `json:"id,omitempty" yaml:"id,omitempty"`
	// Phase of this assignment target. Can be either "provisioned" or "failed".
	Phase OktaAssignmentTargetPhase `json:"phase,omitempty" yaml:"phase,omitempty"`
	// ProcessedAt is the time the target was processed.
	ProcessedAt time.Time `json:"processed_at,omitempty" yaml:"processed_at,omitempty"`
	// FailedAttempts is only relevant if the target's phase is "failed" and then it is the
	// number of failed processing attempts made to provision this target.
	FailedAttempts int64 `json:"failed_attempts,omitempty" yaml:"failed_attempts,omitempty"`
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

func (o *PluginOktaSyncSettings) GetEnableUserSync() bool {
	if o == nil {
		return false
	}
	return o.SyncUsers
}

func (o *PluginOktaSyncSettings) GetEnableAppGroupSync() bool {
	if !o.GetEnableUserSync() {
		return false
	}
	return !o.DisableSyncAppGroups
}

func (o *PluginOktaSyncSettings) GetEnableAccessListSync() bool {
	if !o.GetEnableAppGroupSync() {
		return false
	}
	return o.SyncAccessLists
}

func (o *PluginOktaSyncSettings) GetEnableBidirectionalSync() bool {
	if !o.GetEnableAppGroupSync() {
		return false
	}
	return !o.DisableBidirectionalSync
}

func (o *PluginOktaSyncSettings) GetEnableSystemLogExport() bool {
	if o == nil {
		return false
	}
	return o.EnableSystemLogExport
}

func (o *PluginOktaSyncSettings) GetAssignDefaultRoles() bool {
	if o == nil {
		return false
	}
	return !o.DisableAssignDefaultRoles
}

type OktaUserSyncSource string

// IsUnknown returns true if user sync source is empty or explicitly set to "unknown".
func (s OktaUserSyncSource) IsUnknown() bool {
	switch s {
	case "", OktaUserSyncSourceUnknown:
		return true
	default:
		return false
	}
}

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
