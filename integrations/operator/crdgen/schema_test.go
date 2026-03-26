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

package crdgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// TestCRDSchemaValidation will read all crds in the operator/config/crd/bases directory,
// all k8s custom resource definitions in the operator/crdgen/test/fixtures directory
// and will validate the resource definitions.
// We manually validate here in order to not to rely on sigs.k8s.io/controller-runtime/pkg/envtest.
func TestCRDSchemaValidation(t *testing.T) {
	t.Parallel()
	crdDir := filepath.Join("..", "config", "crd", "bases")
	fixtureDir := filepath.Join("test/fixtures")

	validators := buildValidators(t, crdDir)

	fixtures, err := filepath.Glob(filepath.Join(fixtureDir, "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, fixtures)

	for _, fixturePath := range fixtures {
		t.Run(filepath.Base(fixturePath), func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(fixturePath)
			require.NoError(t, err)

			obj := &unstructured.Unstructured{}
			require.NoError(t, yaml.Unmarshal(data, obj))

			validator := validators[obj.GroupVersionKind()]
			require.NotNil(t, validator)

			result := validator.Validate(obj.Object)
			require.Empty(t, result.Errors)
		})
	}
}

// buildValidators parses all CRDs in the specified directory and returns a map of GVK and SchemaCreateValidator.
func buildValidators(t *testing.T, crdDir string) map[schema.GroupVersionKind]validation.SchemaCreateValidator {
	t.Helper()

	crdFiles, err := filepath.Glob(filepath.Join(crdDir, "*.yaml"))
	require.NoError(t, err)

	validators := make(map[schema.GroupVersionKind]validation.SchemaCreateValidator, len(crdFiles))
	for _, crdFile := range crdFiles {
		data, err := os.ReadFile(crdFile)
		require.NoError(t, err)

		crd := &v1.CustomResourceDefinition{}
		require.NoError(t, yaml.UnmarshalStrict(data, crd))

		require.NotEmpty(t, crd.Spec.Versions)
		ver := crd.Spec.Versions[0]
		require.NotNil(t, ver.Schema)
		openApiV3Schema := ver.Schema.OpenAPIV3Schema
		require.NotNil(t, openApiV3Schema)

		// NewSchemaValidator needs the JSONSchemaProps from apiextensions,
		// luckily the v1 package provides a way to convert
		// between v1 and the internal type
		internalSchema := &apiextensions.JSONSchemaProps{}
		require.NoError(t, v1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(openApiV3Schema, internalSchema, nil))

		validator, _, err := validation.NewSchemaValidator(internalSchema)
		require.NoError(t, err)

		gvk := schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: ver.Name,
			Kind:    crd.Spec.Names.Kind,
		}

		validators[gvk] = validator
	}
	return validators
}
