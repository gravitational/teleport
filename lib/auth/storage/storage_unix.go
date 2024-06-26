//go:build !windows
// +build !windows

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package storage

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend/kubernetes"
	"github.com/gravitational/teleport/lib/backend/lite"
)

// NewProcessStorage returns a new instance of the process storage.
func NewProcessStorage(ctx context.Context, path string) (*ProcessStorage, error) {
	if path == "" {
		return nil, trace.BadParameter("missing parameter path")
	}

	litebk, err := lite.NewWithConfig(ctx, lite.Config{
		Path:      path,
		EventsOff: true,
		Sync:      lite.SyncFull,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// identityStorage holds the storage backend for identity and state.
	// if the agent is running in Kubernetes it's replaced by kubernetes secret storage
	var identityStorage stateBackend = litebk

	// if running in a K8S cluster and required env vars are available
	// the agent will automatically switch state storage from local
	// sqlite into a Kubernetes Secret.
	if kubernetes.InKubeCluster() {
		kubeStorage, err := kubernetes.New()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		identityStorage = kubeStorage
	}

	return &ProcessStorage{BackendStorage: litebk, stateStorage: identityStorage}, nil
}
