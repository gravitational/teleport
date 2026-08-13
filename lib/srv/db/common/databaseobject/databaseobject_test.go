/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package databaseobject

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestNewDatabaseObject(t *testing.T) {
	tests := []struct {
		name          string
		spec          *dbobjectv1.DatabaseObjectSpec
		expectedObj   *dbobjectv1.DatabaseObject
		expectedError error
	}{
		{
			name: "valid object",
			spec: dbobjectv1.DatabaseObjectSpec_builder{
				Protocol:            "postgres",
				DatabaseServiceName: "test",
				ObjectKind:          types.KindDatabaseObject,
				Database:            "test",
				Schema:              "test",
				Name:                "test",
			}.Build(),
			expectedError: nil,
			expectedObj: dbobjectv1.DatabaseObject_builder{
				Kind:    types.KindDatabaseObject,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name:      "valid object",
					Namespace: defaults.Namespace,
				}.Build(),
				Spec: dbobjectv1.DatabaseObjectSpec_builder{
					Protocol:            "postgres",
					DatabaseServiceName: "test",
					ObjectKind:          types.KindDatabaseObject,
					Database:            "test",
					Schema:              "test",
					Name:                "test",
				}.Build(),
			}.Build(),
		},
		{
			name:          "missing name",
			spec:          &dbobjectv1.DatabaseObjectSpec{},
			expectedError: trace.BadParameter("spec.name: must be non-empty"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := NewDatabaseObject(tt.name, tt.spec)
			require.ErrorIs(t, err, tt.expectedError)
			require.Equal(t, tt.expectedObj, obj)
		})
	}
}

func TestValidateDatabaseObject(t *testing.T) {
	tests := []struct {
		name           string
		databaseObject *dbobjectv1.DatabaseObject
		expectedError  error
	}{
		{
			name: "valid object",
			databaseObject: dbobjectv1.DatabaseObject_builder{
				Kind:     types.KindDatabaseObject,
				Metadata: headerv1.Metadata_builder{Name: "test", Namespace: defaults.Namespace}.Build(),
				Spec:     dbobjectv1.DatabaseObjectSpec_builder{Name: "test", Protocol: "test"}.Build(),
			}.Build(),
			expectedError: nil,
		},
		{
			name:           "nil object",
			databaseObject: nil,
			expectedError:  trace.BadParameter("database object must be non-nil"),
		},
		{
			name:           "nil metadata",
			databaseObject: dbobjectv1.DatabaseObject_builder{Metadata: nil}.Build(),
			expectedError:  trace.BadParameter("metadata: must be non-nil"),
		},
		{
			name: "invalid kind",
			databaseObject: dbobjectv1.DatabaseObject_builder{
				Kind:     "InvalidKind",
				Metadata: headerv1.Metadata_builder{Name: "test", Namespace: defaults.Namespace}.Build(),
				Spec:     dbobjectv1.DatabaseObjectSpec_builder{Name: "test"}.Build(),
			}.Build(),
			expectedError: trace.BadParameter("invalid kind InvalidKind, expected db_object"),
		},
		{
			name: "missing spec",
			databaseObject: dbobjectv1.DatabaseObject_builder{
				Kind:     types.KindDatabaseObject,
				Metadata: headerv1.Metadata_builder{Name: "test", Namespace: defaults.Namespace}.Build(),
				Spec:     nil,
			}.Build(),
			expectedError: trace.BadParameter("spec: must be non-empty"),
		},
		{
			name: "missing object name",
			databaseObject: dbobjectv1.DatabaseObject_builder{
				Kind:     types.KindDatabaseObject,
				Metadata: headerv1.Metadata_builder{Name: "", Namespace: defaults.Namespace}.Build(),
			}.Build(),
			expectedError: trace.BadParameter("metadata.name: must be non-empty"),
		},
		{
			name: "missing name",
			databaseObject: dbobjectv1.DatabaseObject_builder{
				Kind:     types.KindDatabaseObject,
				Metadata: headerv1.Metadata_builder{Name: "test", Namespace: defaults.Namespace}.Build(),
				Spec:     dbobjectv1.DatabaseObjectSpec_builder{Name: ""}.Build(),
			}.Build(),
			expectedError: trace.BadParameter("spec.name: must be non-empty"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDatabaseObject(tt.databaseObject)
			require.ErrorIs(t, err, tt.expectedError)
		})
	}
}
