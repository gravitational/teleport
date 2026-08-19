// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package state

import (
	"context"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport/lib/backend"
	kubebackend "github.com/gravitational/teleport/lib/backend/kubernetes"
)

const (
	maxTries      = 3
	operatorIDKey = "operator-id"
)

// State is used by the operator to store and retrieve information in Kubernetes.
// The State is shared across operator replicas.
type State struct {
	bk *kubebackend.Backend
}

// New creates a new State for the operator.
func New(ctx context.Context, client kubernetes.Interface) (*State, error) {
	bk, err := kubebackend.NewSharedWithClient(ctx, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &State{bk: bk}, nil
}

// OperatorID returns the unique ID of the operator.
// The ID is either retrieved from the kube state or created if it doesn't exist yet.
// If the function returns no error, the state contains the ID.
func (s *State) OperatorID(ctx context.Context) (uuid.UUID, error) {
	key := backend.KeyFromString(operatorIDKey)
	for tries := 0; ; tries++ {
		item, err := s.bk.Get(ctx, key)
		if err == nil {
			// ID exists and we can read it.
			return uuid.ParseBytes(item.Value)
		}
		if !trace.IsNotFound(err) {
			// Unexpected error, return it.
			return uuid.UUID{}, trace.Wrap(err, "getting operator ID from kube state")
		}

		// ID does not exist yet, try to create it.
		newID := uuid.New()
		_, err = s.bk.Create(ctx, backend.Item{
			Key:   key,
			Value: []byte(newID.String()),
		})
		if err == nil {
			// Successfully created the ID, return it.
			return newID, nil
		}

		if !trace.IsAlreadyExists(err) {
			// Unexpected error.
			return uuid.UUID{}, trace.Wrap(err, "creating operator ID in kube state")
		}
		// Something created the ID in the meantime, we must try again.

		if tries >= maxTries {
			// Avoid infinite retry loop.
			return uuid.UUID{}, trace.Wrap(err, "failed to create operator ID in kube state a after too many attempts")
		}
	}
}
