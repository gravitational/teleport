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

func NewDatabaseObject(name string, spec *dbobjectv1.DatabaseObjectSpec) (*dbobjectv1.DatabaseObject, error) {
	return NewDatabaseObjectWithLabels(name, nil, spec)
}

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

	err := validateDatabaseObject(databaseObject)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return databaseObject, nil
}

func validateDatabaseObject(databaseObject *dbobjectv1.DatabaseObject) error {
	if databaseObject.Kind != types.KindDatabaseObject {
		return trace.BadParameter("wrong kind %v", databaseObject.Kind)
	}
	if databaseObject.Spec == nil {
		return trace.BadParameter("missing spec")
	}
	if databaseObject.Spec.Name == "" {
		return trace.BadParameter("missing name")
	}
	return nil
}
