/*
Copyright 2022 Gravitational, Inc.

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

	// DeleteDatabasService removes the specified DatabaseService resource.
	DeleteDatabaseService(ctx context.Context, name string) error

	// DeleteAllDatabaseServices removes all DatabaseService resources.
	DeleteAllDatabaseServices(context.Context) error
}

// MarshalDatabaseService marshals the DatabaseService resource to JSON.
func MarshalDatabaseService(databaseService types.DatabaseService, opts ...MarshalOption) ([]byte, error) {
	if err := databaseService.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch databaseService := databaseService.(type) {
	case *types.DatabaseServiceV1:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *databaseService
			copy.SetResourceID(0)
			databaseService = &copy
		}
		return utils.FastMarshal(databaseService)
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
			return nil, trace.BadParameter(err.Error())
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
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
