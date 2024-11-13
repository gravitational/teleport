// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

// DownstreamID holds the configured ID of a downstream identity "receiver". For
// the first iteration of our provisioning system a "downstream" is synonymous
// with a target SCIM server, but this may not always be the case.
type DownstreamID string

// ProvisioningStateID holds the ID of a given provisioning state. Provisioning
// state IDs are only unique within a given downstream, and must be
// disambiguated by a DownstreamID when queried or updated.
type ProvisioningStateID string

type DownstreamProvisioningStateGetter interface {
	// GetProvisioningState fetches a single provisioning state record for a given
	// downstream and principal,
	GetProvisioningState(context.Context, DownstreamID, ProvisioningStateID) (*provisioningv1.PrincipalState, error)
}

// DownstreamProvisioningStates defines an interface for managing principal
// provisioning state records scoped by a target downstream receiver.
type DownstreamProvisioningStates interface {
	DownstreamProvisioningStateGetter

	// ListProvisioningStates lists all provisioning state records for a given
	// downstream receiver.
	ListProvisioningStates(context.Context, DownstreamID, int, *pagination.PageRequestToken) ([]*provisioningv1.PrincipalState, pagination.NextPageToken, error)

	// Creates a new backend PrincipalState record. The target DownstreamID is
	// drawn from the supplied record.
	CreateProvisioningState(context.Context, *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error)

	// UpdateProvisioningState performs a conditional update of the supplied
	// PrincipalState, returning the updated resource. The target DownstreamID
	// is drawn from the supplied record.
	UpdateProvisioningState(context.Context, *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error)

	// UpsertProvisioningState performs an *unconditional* upsert of the supplied
	// PrincipalState, returning the updated resource. The target DownstreamID
	// is drawn from the supplied record. Beware of interactions that expect
	// protection from optimistic locks.
	UpsertProvisioningState(context.Context, *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error)

	// DeleteProvisioningState deletes a given principal's provisioning state
	// record
	DeleteProvisioningState(context.Context, DownstreamID, ProvisioningStateID) error

	// DeleteDownstreamProvisioningStates deletes *all* provisioning records for
	// a given downstream
	DeleteDownstreamProvisioningStates(context.Context, *provisioningv1.DeleteDownstreamProvisioningStatesRequest) (*emptypb.Empty, error)
}

// ProvisioningStates defines an interface for managing a Provisioning Principal
// State record database.
type ProvisioningStates interface {
	DownstreamProvisioningStates

	// ListProvisioningStatesForAllDownstreams lists all provisioning state
	// records for all downstream receivers. Note that the returned record names
	// may not be unique across all downstream receivers. Check the records'
	// `DownstreamID` field to disambiguate.
	ListProvisioningStatesForAllDownstreams(context.Context, int, *pagination.PageRequestToken) ([]*provisioningv1.PrincipalState, pagination.NextPageToken, error)

	// DeleteAllProvisioningStates deletes all provisioning state records for
	// all downstream receivers.
	DeleteAllProvisioningStates(context.Context) error
}
