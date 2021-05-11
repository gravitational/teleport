/*
Copyright 2021 Gravitational, Inc.

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

package workflows

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

// This file creates aliases and alternative structs for many
// Request related types in order to abstract over implementation
// details and provide a straightforward user experience.

// Request is an access request.
type Request struct {
	// ID is the unique identifier of the request.
	ID string
	// User is the user to whom the request applies.
	User string
	// Roles are the roles that the user will be granted
	// if the request is approved.
	Roles []string
	// State is the current state of the request.
	State State
	// Created is a creation time of the request.
	Created time.Time
	// RequestReason is an optional message explaining the reason for the request.
	RequestReason string
	// ResolveReason is an optional message explaining the reason for the resolution
	// (approval/denail) of the request.
	ResolveReason string
	// ResolveAnnotations is a set of arbitrary values sent by plugins or other
	// resolving parties during approval/denial.
	ResolveAnnotations map[string][]string
	// SystemAnnotations is a set of programmatically generated annotations attached
	// to pending access requests by teleport.
	SystemAnnotations map[string][]string
	// SuggestedReviewers is a set of usernames which are subjects to review the request.
	SuggestedReviewers []string
}

func requestFromAccessRequest(req types.AccessRequest) Request {
	return Request{
		ID:                 req.GetName(),
		User:               req.GetUser(),
		Roles:              req.GetRoles(),
		State:              req.GetState(),
		Created:            req.GetCreationTime(),
		RequestReason:      req.GetRequestReason(),
		ResolveReason:      req.GetResolveReason(),
		ResolveAnnotations: req.GetResolveAnnotations(),
		SystemAnnotations:  req.GetSystemAnnotations(),
		SuggestedReviewers: req.GetSuggestedReviewers(),
	}
}

// State represents the state of an access request.
type State = types.RequestState

const (
	// StatePending is the state of a pending request.
	StatePending State = types.RequestState_PENDING
	// StateApproved is the state of an approved request.
	StateApproved State = types.RequestState_APPROVED
	// StateDenied is the state of a denied request.
	StateDenied State = types.RequestState_DENIED
)

// Op describes the operation type of a RequestEvent.
type OpType = types.OpType

const (
	// OpInit is sent as the first sentinel value on the watch channel.
	OpInit = types.OpInit
	// OpPut inicates creation or update.
	OpPut = types.OpPut
	// OpDelete indicates deletion or expiry.
	OpDelete = types.OpDelete
)

// RequestUpdate describes request updating parameters.
type RequestUpdate types.AccessRequestUpdate

// Filter describes request filtering parameters.
type Filter = types.AccessRequestFilter

// PluginDataMap is custom user data associated with an access request.
type PluginDataMap map[string]string
