// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
			spec: &dbobjectv1.DatabaseObjectSpec{
				Protocol:            "postgres",
				DatabaseServiceName: "test",
				ObjectKind:          types.KindDatabaseObject,
				Database:            "test",
				Schema:              "test",
				Name:                "test",
			},
			expectedError: nil,
			expectedObj: &dbobjectv1.DatabaseObject{
				Kind:    types.KindDatabaseObject,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:      "valid object",
					Namespace: defaults.Namespace,
				},
				Spec: &dbobjectv1.DatabaseObjectSpec{
					Protocol:            "postgres",
					DatabaseServiceName: "test",
					ObjectKind:          types.KindDatabaseObject,
					Database:            "test",
					Schema:              "test",
					Name:                "test",
				},
			},
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
			databaseObject: &dbobjectv1.DatabaseObject{
				Kind:     types.KindDatabaseObject,
				Metadata: &headerv1.Metadata{Name: "test", Namespace: defaults.Namespace},
				Spec:     &dbobjectv1.DatabaseObjectSpec{Name: "test", Protocol: "test"},
			},
			expectedError: nil,
		},
		{
			name:           "nil object",
			databaseObject: nil,
			expectedError:  trace.BadParameter("database object must be non-nil"),
		},
		{
			name:           "nil metadata",
			databaseObject: &dbobjectv1.DatabaseObject{Metadata: nil},
			expectedError:  trace.BadParameter("metadata: must be non-nil"),
		},
		{
			name: "invalid kind",
			databaseObject: &dbobjectv1.DatabaseObject{
				Kind:     "InvalidKind",
				Metadata: &headerv1.Metadata{Name: "test", Namespace: defaults.Namespace},
				Spec:     &dbobjectv1.DatabaseObjectSpec{Name: "test"},
			},
			expectedError: trace.BadParameter("invalid kind InvalidKind, expected db_object"),
		},
		{
			name: "missing spec",
			databaseObject: &dbobjectv1.DatabaseObject{
				Kind:     types.KindDatabaseObject,
				Metadata: &headerv1.Metadata{Name: "test", Namespace: defaults.Namespace},
				Spec:     nil,
			},
			expectedError: trace.BadParameter("spec: must be non-empty"),
		},
		{
			name: "missing object name",
			databaseObject: &dbobjectv1.DatabaseObject{
				Kind:     types.KindDatabaseObject,
				Metadata: &headerv1.Metadata{Name: "", Namespace: defaults.Namespace},
			},
			expectedError: trace.BadParameter("metadata.name: must be non-empty"),
		},
		{
			name: "missing name",
			databaseObject: &dbobjectv1.DatabaseObject{
				Kind:     types.KindDatabaseObject,
				Metadata: &headerv1.Metadata{Name: "test", Namespace: defaults.Namespace},
				Spec:     &dbobjectv1.DatabaseObjectSpec{Name: ""},
			},
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
