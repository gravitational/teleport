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
	"time"

	"github.com/gravitational/trace"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	apiusertasks "github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/usertasks"
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
	// Title is the issue title.
	Title string `json:"title,omitempty"`
	// Integration is the Integration Name this User Task refers to.
	Integration string `json:"integration,omitempty"`
	// LastStateChange indicates when the current's user task state was last changed.
	LastStateChange time.Time `json:"lastStateChange,omitempty"`
}

// UserTaskDetail contains all the details for a User Task.
type UserTaskDetail struct {
	// UserTask has the basic fields that all tasks include.
	UserTask
	// Description is a markdown document that explains the issue and how to fix it.
	Description string `json:"description,omitempty"`
	// DiscoverEC2 contains the task details for the DiscoverEC2 tasks.
	DiscoverEC2 *usertasks.UserTaskDiscoverEC2WithURLs `json:"discoverEc2,omitempty"`
	// DiscoverEKS contains the task details for the DiscoverEKS tasks.
	DiscoverEKS *usertasks.UserTaskDiscoverEKSWithURLs `json:"discoverEks,omitempty"`
	// DiscoverRDS contains the task details for the DiscoverRDS tasks.
	DiscoverRDS *usertasks.UserTaskDiscoverRDSWithURLs `json:"discoverRds,omitempty"`
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

// MakeUserTasks creates a UI list of UserTasks.
func MakeUserTasks(uts []*usertasksv1.UserTask) []UserTask {
	uiList := make([]UserTask, 0, len(uts))

	for _, ut := range uts {
		uiList = append(uiList, MakeUserTask(ut))
	}

	return uiList
}

// MakeDetailedUserTask creates a UI UserTask representation containing all the details.
func MakeDetailedUserTask(ut *usertasksv1.UserTask) UserTaskDetail {
	var discoverEC2 *usertasks.UserTaskDiscoverEC2WithURLs
	var discoverEKS *usertasks.UserTaskDiscoverEKSWithURLs
	var discoverRDS *usertasks.UserTaskDiscoverRDSWithURLs

	switch ut.GetSpec().GetTaskType() {
	case apiusertasks.TaskTypeDiscoverEC2:
		discoverEC2 = usertasks.EC2InstancesWithURLs(ut)

	case apiusertasks.TaskTypeDiscoverEKS:
		discoverEKS = usertasks.EKSClustersWithURLs(ut)

	case apiusertasks.TaskTypeDiscoverRDS:
		discoverRDS = usertasks.RDSDatabasesWithURLs(ut)
	}

	_, description := userTaskTitleAndDescription(ut)

	return UserTaskDetail{
		UserTask:    MakeUserTask(ut),
		Description: description,
		DiscoverEC2: discoverEC2,
		DiscoverEKS: discoverEKS,
		DiscoverRDS: discoverRDS,
	}
}

func userTaskTitleAndDescription(ut *usertasksv1.UserTask) (string, string) {
	switch ut.GetSpec().GetTaskType() {
	case apiusertasks.TaskTypeDiscoverEC2:
		return usertasks.DescriptionForDiscoverEC2Issue(ut.GetSpec().GetIssueType())

	case apiusertasks.TaskTypeDiscoverEKS:
		return usertasks.DescriptionForDiscoverEC2Issue(ut.GetSpec().GetIssueType())

	case apiusertasks.TaskTypeDiscoverRDS:
		return usertasks.DescriptionForDiscoverEC2Issue(ut.GetSpec().GetIssueType())
	}

	return "", ""
}

// MakeUserTask creates a UI UserTask representation.
func MakeUserTask(ut *usertasksv1.UserTask) UserTask {
	title, _ := userTaskTitleAndDescription(ut)
	return UserTask{
		Name:            ut.GetMetadata().GetName(),
		TaskType:        ut.GetSpec().GetTaskType(),
		State:           ut.GetSpec().GetState(),
		IssueType:       ut.GetSpec().GetIssueType(),
		Title:           title,
		Integration:     ut.GetSpec().GetIntegration(),
		LastStateChange: ut.GetStatus().GetLastStateChange().AsTime(),
	}
}
