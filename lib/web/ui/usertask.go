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

package ui

import (
	"github.com/gravitational/trace"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
)

// UserTask describes UserTask fields.
// Used for listing User Tasks without receiving all the details.
type UserTask struct {
	// Name is the UserTask name.
	Name string `json:"name"`
	// TaskType identifies this task's type.
	TaskType string `json:"taskType"`
	// State is the state for the User Task.
	State string `json:"state,omitempty"`
	// IssueType identifies this task's issue type.
	IssueType string `json:"issueType,omitempty"`
	// Integration is the Integration Name this User Task refers to.
	Integration string `json:"integration,omitempty"`
}

// UserTaskDetail contains all the details for a User Task.
type UserTaskDetail struct {
	// UserTask has the basic fields that all taks include.
	UserTask
	// DiscoverEC2 contains the task details for the DiscoverEC2 tasks.
	DiscoverEC2 *usertasksv1.DiscoverEC2 `json:"discoverEc2,omitempty"`
	AccessGraph *AccessGraph             `json:"accessGraph,omitempty"`
}

// AccessGraph contains the instances that failed to auto-enroll into the cluster.
type AccessGraph struct {
	Severity    string        `json:"severity,omitempty"`
	RiskFactors []*RiskFactor `json:"risk_factors,omitempty"`
	AccountId   string        `json:"account_id,omitempty"`
}

type RiskFactor struct {
	Name             string            `json:"name,omitempty"`
	Severity         string            `json:"severity,omitempty"`
	Reason           string            `json:"reason,omitempty"`
	UnusedRole       *string           `json:"unused_role,omitempty"`
	PolicyRiskFactor *PolicyRiskFactor `json:"policy_risk_factor,omitempty"`
}

// UpdateUserTaskStateRequest is a request to update a UserTask
type UpdateUserTaskStateRequest struct {
	// State is the new state for the User Task.
	State string `json:"state,omitempty"`
}

// CheckAndSetDefaults checks if the provided values are valid.
func (r *UpdateUserTaskStateRequest) CheckAndSetDefaults() error {
	if r.State == "" {
		return trace.BadParameter("missing state")
	}

	return nil
}

// UserTasksListResponse contains a list of UserTasks.
// In case of exceeding the pagination limit (either via query param `limit` or the default 1000)
// a `nextToken` is provided and should be used to obtain the next page (as a query param `startKey`)
type UserTasksListResponse struct {
	// Items is a list of resources retrieved.
	Items []UserTask `json:"items"`
	// NextKey is the position to resume listing events.
	NextKey string `json:"nextKey"`
}

// UserTasksListDetailedResponse contains a list of UserTaskDetail.
// In case of exceeding the pagination limit (either via query param `limit` or the default 1000)
// a `nextToken` is provided and should be used to obtain the next page (as a query param `startKey`)
type UserTasksListDetailedResponse struct {
	// Items is a list of resources retrieved.
	Items []UserTaskDetail `json:"items"`
	// NextKey is the position to resume listing events.
	NextKey string `json:"nextKey"`
}

// MakeUserTasks creates a UI list of UserTasks.
func MakeUserTasks(uts []*usertasksv1.UserTask) []UserTask {
	uiList := make([]UserTask, 0, len(uts))

	for _, ut := range uts {
		uiList = append(uiList, MakeUserTask(ut))
	}

	return uiList
}

func severityToString(severity usertasksv1.Severity) string {
	switch severity {
	case usertasksv1.Severity_SEVERITY_LOW:
		return "low"
	case usertasksv1.Severity_SEVERITY_MEDIUM:
		return "medium"
	case usertasksv1.Severity_SEVERITY_HIGH:
		return "high"
	case usertasksv1.Severity_SEVERITY_CRITICAL:
		return "critical"
	default:
		return "unknown"
	}
}

type PolicyUpdate struct {
	PolicyName     string `json:"policy_name,omitempty"`
	PreviousPolicy string `json:"previous_policy,omitempty"`
	NewPolicy      string `json:"new_policy,omitempty"`
	Detach         bool   `json:"detach,omitempty"`
}
type PolicyRiskFactor struct {
	Updates []*PolicyUpdate `json:"updates,omitempty"`
}

// MakeDetailedUserTask creates a UI UserTask representation containing all the details.
func MakeDetailedUserTask(ut *usertasksv1.UserTask) UserTaskDetail {
	var accessGraph *AccessGraph
	if ut.GetSpec().GetAccessGraph() != nil {
		accessGraph = &AccessGraph{
			Severity:    severityToString(ut.GetSpec().GetAccessGraph().GetSeverity()),
			RiskFactors: make([]*RiskFactor, 0, len(ut.GetSpec().GetAccessGraph().GetRiskFactors())),
			AccountId:   ut.GetSpec().GetAccessGraph().GetAccountId(),
		}
		for _, rf := range ut.GetSpec().GetAccessGraph().GetRiskFactors() {
			var unusedRole *string
			var policyRiskFactor *PolicyRiskFactor
			if uRole := rf.GetUnusedRole(); unusedRole != nil {
				unusedRole = &(uRole.LastUsed)
			} else if policy := rf.GetPolicy(); policy != nil {
				updates := make([]*PolicyUpdate, 0, len(policy.GetUpdates()))
				for _, update := range policy.GetUpdates() {
					updates = append(updates, &PolicyUpdate{
						PolicyName:     update.GetPolicyName(),
						PreviousPolicy: update.GetPreviousPolicy(),
						NewPolicy:      update.GetNewPolicy(),
						Detach:         update.GetDetach(),
					})
				}
				policyRiskFactor = &PolicyRiskFactor{
					Updates: updates,
				}
			}
			accessGraph.RiskFactors = append(accessGraph.RiskFactors, &RiskFactor{
				Name:     rf.GetName(),
				Severity: severityToString(rf.GetSeverity()),

				UnusedRole:       unusedRole,
				PolicyRiskFactor: policyRiskFactor,
			})
		}
	}
	return UserTaskDetail{
		UserTask:    MakeUserTask(ut),
		DiscoverEC2: ut.GetSpec().GetDiscoverEc2(),
		AccessGraph: accessGraph,
	}
}

// MakeUserTask creates a UI UserTask representation.
func MakeUserTask(ut *usertasksv1.UserTask) UserTask {
	return UserTask{
		Name:        ut.GetMetadata().GetName(),
		TaskType:    ut.GetSpec().GetTaskType(),
		State:       ut.GetSpec().GetState(),
		IssueType:   ut.GetSpec().GetIssueType(),
		Integration: ut.GetSpec().GetIntegration(),
	}
}
