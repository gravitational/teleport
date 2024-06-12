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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// DatabaseService manages database resources in the backend.
type DatabaseService struct {
	backend.Backend
}

// NewDatabasesService creates a new DatabasesService.
func NewDatabasesService(backend backend.Backend) *DatabaseService {
	return &DatabaseService{Backend: backend}
}

// GetDatabases returns all database resources.
func (s *DatabaseService) GetDatabases(ctx context.Context) ([]types.Database, error) {
	startKey := backend.ExactKey(databasesPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	databases := make([]types.Database, len(result.Items))
	for i, item := range result.Items {
		database, err := services.UnmarshalDatabase(item.Value,
			services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		databases[i] = database
	}
	return databases, nil
}

// GetDatabase returns the specified database resource.
func (s *DatabaseService) GetDatabase(ctx context.Context, name string) (types.Database, error) {
	item, err := s.Get(ctx, backend.Key(databasesPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("database %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}
	database, err := services.UnmarshalDatabase(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return database, nil
}

// CreateDatabase creates a new database resource.
func (s *DatabaseService) CreateDatabase(ctx context.Context, database types.Database) error {
	if err := services.CheckAndSetDefaults(database); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalDatabase(database)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(databasesPrefix, database.GetName()),
		Value:   value,
		Expires: database.Expiry(),
		ID:      database.GetResourceID(),
	}
	_, err = s.Create(ctx, item)
	if trace.IsAlreadyExists(err) {
		return trace.AlreadyExists("database %q already exists", database.GetName())
	}

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateDatabase updates an existing database resource.
func (s *DatabaseService) UpdateDatabase(ctx context.Context, database types.Database) error {
	if err := services.CheckAndSetDefaults(database); err != nil {
		return trace.Wrap(err)
	}
	rev := database.GetRevision()
	value, err := services.MarshalDatabase(database)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.Key(databasesPrefix, database.GetName()),
		Value:    value,
		Expires:  database.Expiry(),
		ID:       database.GetResourceID(),
		Revision: rev,
	}
	_, err = s.Update(ctx, item)
	if trace.IsNotFound(err) {
		return trace.NotFound("database %q doesn't exist", database.GetName())
	}

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteDatabase removes the specified database resource.
func (s *DatabaseService) DeleteDatabase(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.Key(databasesPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("database %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllDatabases removes all database resources.
func (s *DatabaseService) DeleteAllDatabases(ctx context.Context) error {
	startKey := backend.ExactKey(databasesPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	databasesPrefix = "db"
)
