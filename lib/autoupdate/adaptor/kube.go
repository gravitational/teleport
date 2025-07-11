/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package adaptor

import (
	"context"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/kubernetes"
)

func CreateKubeUpdaterIDIfEmpty(ctx context.Context, kubeBackend *kubernetes.Backend) error {
	var lookupErr, createErr error
	for range 5 {
		_, lookupErr = LookupKubeUpdaterID(ctx, kubeBackend)
		if lookupErr == nil {
			return nil
		}
		if !trace.IsNotFound(lookupErr) {
			return trace.Wrap(lookupErr, "unexpected error looking up updater ID")
		}
		createErr = CreateKubeUpdaterID(ctx, kubeBackend)
		if createErr == nil {
			return nil
		}
		if !trace.IsRetryError(createErr) {
			return trace.Wrap(createErr, "unexpected error creating updater ID")
		}
	}
	return trace.Errorf("failed to create ID, lookup failed with %q and create failed with %q", lookupErr, createErr)
}

func CreateKubeUpdaterID(ctx context.Context, kubeBackend *kubernetes.Backend) error {
	updaterID := uuid.NewString()
	item := backend.Item{
		Key:   backend.NewKey(teleport.UpdaterIDKubeBackendKey),
		Value: []byte(updaterID),
	}
	_, err := kubeBackend.Create(ctx, item)
	if err != nil {
		if errors.IsConflict(err) {
			return trace.Retry(err, "conflict when writing updater ID")
		}
		return trace.Wrap(err)
	}
	return nil
}

func LookupKubeUpdaterID(ctx context.Context, kubeBackend *kubernetes.Backend) (uuid.UUID, error) {
	item, err := kubeBackend.Get(ctx, backend.NewKey(teleport.UpdaterIDKubeBackendKey))
	if err != nil {
		return uuid.Nil, trace.Wrap(err)
	}

	return uuid.Parse(string(item.Value))
}
