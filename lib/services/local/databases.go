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
	startKey := backend.Key(databasesPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	databases := make([]types.Database, len(result.Items))
	for i, item := range result.Items {
		database, err := services.UnmarshalDatabase(item.Value,
			services.WithResourceID(item.ID), services.WithExpires(item.Expires))
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
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return database, nil
}

// CreateDatabase creates a new database resource.
func (s *DatabaseService) CreateDatabase(ctx context.Context, database types.Database) error {
	if err := database.CheckAndSetDefaults(); err != nil {
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
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateDatabase updates an existing database resource.
func (s *DatabaseService) UpdateDatabase(ctx context.Context, database types.Database) error {
	if err := database.CheckAndSetDefaults(); err != nil {
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
	_, err = s.Update(ctx, item)
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
	startKey := backend.Key(databasesPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	databasesPrefix = "db"
)
