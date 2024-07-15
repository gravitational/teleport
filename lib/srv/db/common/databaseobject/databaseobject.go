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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewDatabaseObject creates a new dbobjectv1.DatabaseObject.
func NewDatabaseObject(name string, spec *dbobjectv1.DatabaseObjectSpec) (*dbobjectv1.DatabaseObject, error) {
	return NewDatabaseObjectWithLabels(name, nil, spec)
}

// NewDatabaseObjectWithLabels creates a new dbobjectv1.DatabaseObject with specified labels.
func NewDatabaseObjectWithLabels(name string, labels map[string]string, spec *dbobjectv1.DatabaseObjectSpec) (*dbobjectv1.DatabaseObject, error) {
	databaseObject := &dbobjectv1.DatabaseObject{
		Kind:    types.KindDatabaseObject,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
			Labels:    labels,
		},
		Spec: spec,
	}

	err := ValidateDatabaseObject(databaseObject)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return databaseObject, nil
}

// ValidateDatabaseObject checks if *dbobjectv1.DatabaseObject is valid.
func ValidateDatabaseObject(obj *dbobjectv1.DatabaseObject) error {
	if obj == nil {
		return trace.BadParameter("database object must be non-nil")
	}
	if obj.Metadata == nil {
		return trace.BadParameter("metadata: must be non-nil")
	}
	if obj.Metadata.Name == "" {
		return trace.BadParameter("metadata.name: must be non-empty")
	}
	if obj.Kind != types.KindDatabaseObject {
		return trace.BadParameter("invalid kind %v, expected %v", obj.Kind, types.KindDatabaseObject)
	}
	if obj.Spec == nil {
		return trace.BadParameter("spec: must be non-empty")
	}
	if obj.Spec.Name == "" {
		return trace.BadParameter("spec.name: must be non-empty")
	}
	if obj.Spec.Protocol == "" {
		return trace.BadParameter("spec.protocol: must be non-empty")
	}
	return nil
}
