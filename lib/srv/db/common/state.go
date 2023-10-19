// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

// Teleport-related SQL states.
//
// SQLSTATE reference:
// https://en.wikipedia.org/wiki/SQLSTATE
const (
	// SQLStateActiveUser is the SQLSTATE raised by deactivation procedure when
	// user has active connections.
	SQLStateActiveUser = "TP000"
	// SQLStateUsernameDoesNotMatch is the SQLSTATE raised by activation
	// procedure when the Teleport username does not match user's attributes.
	//
	// Possibly there is a hash collision, or someone manually updated the user
	// attributes.
	SQLStateUsernameDoesNotMatch = "TP001"
	// SQLStateRolesChanged is the SQLSTATE raised by activation procedure when
	// the user has active connections but roles has changed.
	SQLStateRolesChanged = "TP002"
	// SQLStateUserDropped is the SQLSTATE returned by the delete procedure
	// indicating the user was dropped.
	SQLStateUserDropped = "TP003"
	// SQLStateUserDeactivated is the SQLSTATE returned by the delete procedure
	// indicating was deactivated.
	SQLStateUserDeactivated = "TP004"
)
