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

package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// DatabaseServices defines an interface for managing DatabaseService resources.
type DatabaseServices interface {
	// UpsertDatabaseService updates an existing DatabaseService resource.
	UpsertDatabaseService(context.Context, types.DatabaseService) (*types.KeepAlive, error)

	// DeleteDatabaseService removes the specified DatabaseService resource.
	DeleteDatabaseService(ctx context.Context, name string) error

	// DeleteAllDatabaseServices removes all DatabaseService resources.
	DeleteAllDatabaseServices(context.Context) error
}

// MarshalDatabaseService marshals the DatabaseService resource to JSON.
func MarshalDatabaseService(databaseService types.DatabaseService, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch databaseService := databaseService.(type) {
	case *types.DatabaseServiceV1:
		if err := databaseService.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, databaseService))
	default:
		return nil, trace.BadParameter("unrecognized DatabaseService version %T", databaseService)
	}
}

// UnmarshalDatabaseService unmarshals the DatabaseService resource from JSON.
func UnmarshalDatabaseService(data []byte, opts ...MarshalOption) (types.DatabaseService, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing DatabaseService data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V1:
		var s types.DatabaseServiceV1
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter("%s", err)
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Revision != "" {
			s.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("database service resource version %q is not supported", h.Version)
}
