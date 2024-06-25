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

package local

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// VersionService manages backend version.
type VersionService struct {
	backend.Backend
	log *logrus.Entry
}

// NewVersionService returns new version service instance.
func NewVersionService(backend backend.Backend) *VersionService {
	return &VersionService{
		Backend: backend,
		log:     logrus.WithFields(logrus.Fields{teleport.ComponentKey: "VersionService"}),
	}
}

// UpsertTeleportVersion creates or overwrites an existing version.
func (s *VersionService) UpsertTeleportVersion(ctx context.Context, version types.Version) (types.Version, error) {
	rev := version.GetRevision()
	value, err := services.MarshalVersion(version)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.Key(versionPrefix),
		Value:    value,
		Expires:  version.Expiry(),
		Revision: rev,
	}

	lease, err := s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	version.SetRevision(lease.Revision)
	return version, nil
}

// GetTeleportVersion returns teleport version stored in backend.
func (s *VersionService) GetTeleportVersion(ctx context.Context) (types.Version, error) {
	item, err := s.Get(ctx, backend.Key(versionPrefix))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalVersion(item.Value, services.WithRevision(item.Revision))
}

const (
	versionPrefix = "version"
)
