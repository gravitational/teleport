//go:build !windows
// +build !windows

/*
Copyright 2019 Gravitational, Inc.

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

package auth

import (
	"context"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/kubernetes"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/trace"
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

	// identityStorage is the
	// if in Kubernetes it's replaced by kubernetes secret storage
	var identityStorage stateBackend = litebk

	// if running in a K8S cluster and required env vars are available
	// the agent will automatically switch state storage from local
	// sqlite into a Kubernetes Secret.
	if kubernetes.InKubeCluster() {
		kubeStorage, err := kubernetes.New()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// if secret does not exist but Statefulset had local storage with identities,
		// the agent reads them from SQLite and dumps into Kubernetes Secret.
		// TODO(tigrato): remove this once the compatibility layer between local
		// storage and Kube secret storage is no longer required!
		if !kubeStorage.Exists(ctx) {
			if err := copyLocalStorageIntoKubernetes(ctx, kubeStorage, litebk); err != nil {
				return nil, trace.Wrap(err)
			}
		}

		identityStorage = kubeStorage
	}

	return &ProcessStorage{BackendStorage: litebk, stateStorage: identityStorage}, nil
}

// copyLocalStorageIntoKubernetes reads every `identity` and `state` keys from local storage
// and copies them into Kubernetes Secret. This code is executed only when the agent starts and the
// secret was not yet created in K8S. Subsequent restarts of the agent won't execute this code since the
// secret already exists in K8S
// TODO(tigrato): remove this once the compatibility layer between local storage and
// Kube secret storage is no longer required!
func copyLocalStorageIntoKubernetes(ctx context.Context, k8sStorage *kubernetes.Backend, litebk *lite.Backend) error {
	// read keys starting with `/ids`, e.g. `/ids/{role}/{current,replacement}`
	idsStorage := readPrefixedKeysFromLocalStorage(ctx, litebk, idsPrefix)

	// read keys starting with `/states` `/states/{role}/state`
	stateStorage := readPrefixedKeysFromLocalStorage(ctx, litebk, statesPrefix)

	// if no keys where found, this is a fresh start.
	if len(idsStorage) == 0 && len(stateStorage) == 0 {
		return nil
	}

	// store keys in K8S Secret
	return trace.Wrap(k8sStorage.PutItems(ctx, append(idsStorage, stateStorage...)...))
}

// readPrefixedKeysFromLocalStorage reads every key from local storage whose key starts with `prefix`.
// If no values were found or an error is returned from SQLite backend, it ignores any error and returns empty items.
// TODO(tigrato): remove this once the compatibility layer between local storage and Kube secret storage is no longer required!
func readPrefixedKeysFromLocalStorage(ctx context.Context, litebk *lite.Backend, prefix string) (items []backend.Item) {
	results, err := litebk.GetRange(
		ctx,
		backend.Key(prefix),
		backend.RangeEnd(
			backend.Key(prefix),
		),
		backend.NoLimit,
	)
	if err != nil {
		return
	}

	return results.Items
}
