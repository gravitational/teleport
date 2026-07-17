/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// TestRoleCRDsPreserveUnknownAppResourceFields checks the generated role
// CRDs set x-kubernetes-preserve-unknown-fields on app_resources items, so
// the API server does not prune unknown fields before the operator sees them.
func TestRoleCRDsPreserveUnknownAppResourceFields(t *testing.T) {
	files := []string{
		"config/crd/bases/resources.teleport.dev_roles.yaml",
		"config/crd/bases/resources.teleport.dev_rolesv6.yaml",
		"config/crd/bases/resources.teleport.dev_rolesv7.yaml",
		"config/crd/bases/resources.teleport.dev_rolesv8.yaml",
		"config/crd/bases/resources.teleport.dev_rolesv9.yaml",
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err, file)
		var crd apiextv1.CustomResourceDefinition
		require.NoError(t, yaml.Unmarshal(data, &crd), file)
		require.NotEmpty(t, crd.Spec.Versions, file)
		for _, version := range crd.Spec.Versions {
			spec := version.Schema.OpenAPIV3Schema.Properties["spec"]
			allow := spec.Properties["allow"]
			appResources, ok := allow.Properties["app_resources"]
			require.True(t, ok, "%s version %s has no app_resources schema", file, version.Name)
			items := appResources.Items.Schema
			require.NotNil(t, items.XPreserveUnknownFields, "%s version %s app_resources items must preserve unknown fields", file, version.Name)
			require.True(t, *items.XPreserveUnknownFields, "%s version %s app_resources items must preserve unknown fields", file, version.Name)
		}
	}
}
