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
	databaseObject := dbobjectv1.DatabaseObject_builder{
		Kind:    types.KindDatabaseObject,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:      name,
			Namespace: defaults.Namespace,
			Labels:    labels,
		}.Build(),
		Spec: spec,
	}.Build()

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
	if !obj.HasMetadata() {
		return trace.BadParameter("metadata: must be non-nil")
	}
	if obj.GetMetadata().GetName() == "" {
		return trace.BadParameter("metadata.name: must be non-empty")
	}
	if obj.GetKind() != types.KindDatabaseObject {
		return trace.BadParameter("invalid kind %v, expected %v", obj.GetKind(), types.KindDatabaseObject)
	}
	if !obj.HasSpec() {
		return trace.BadParameter("spec: must be non-empty")
	}
	if obj.GetSpec().GetName() == "" {
		return trace.BadParameter("spec.name: must be non-empty")
	}
	if obj.GetSpec().GetProtocol() == "" {
		return trace.BadParameter("spec.protocol: must be non-empty")
	}
	return nil
}
