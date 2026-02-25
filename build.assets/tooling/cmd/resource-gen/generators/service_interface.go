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

package generators

import (
	"fmt"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type serviceInterfaceData struct {
	resourceBase
	GetParams    string
	DeleteParams string
	ListParams   string
	HasGetter    bool
	HasWrite     bool
}

// GenerateServiceInterface renders the service interface file contents.
func GenerateServiceInterface(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	d := serviceInterfaceData{
		resourceBase: newResourceBase(rs, module),
		GetParams:    getParams(rs.Storage),
		DeleteParams: deleteParams(rs.Storage),
		ListParams:   listParams(rs.Storage),
		HasGetter:    rs.Operations.Get || rs.Operations.List,
		HasWrite:     rs.Operations.Create || rs.Operations.Update || rs.Operations.Upsert || rs.Operations.Delete,
	}
	return render("service-interface", serviceInterfaceTmpl, d)
}

var serviceInterfaceTmpl = mustReadTemplate("service_interface.go.tmpl")

// GenerateValidation renders the validation scaffold file (lib/services/foo.go)
// with a ValidateFoo stub that the developer fills in.
func GenerateValidation(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	d := serviceInterfaceData{
		resourceBase: newResourceBase(rs, module),
	}
	return render("validation", validationTmpl, d)
}

var validationTmpl = mustReadTemplate("validation.go.tmpl")

// GenerateValidationTest renders the validation test scaffold file (lib/services/foo_test.go).
func GenerateValidationTest(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	d := serviceInterfaceData{
		resourceBase: newResourceBase(rs, module),
	}
	return render("validation-test", validationTestTmpl, d)
}

var validationTestTmpl = mustReadTemplate("validation_test.go.tmpl")

func getParams(storage spec.StorageConfig) string {
	switch storage.Pattern {
	case spec.StoragePatternScoped:
		return fmt.Sprintf(", %s string, name string", storage.ScopeBy)
	case spec.StoragePatternSingleton:
		return ""
	default:
		return ", name string"
	}
}

func deleteParams(storage spec.StorageConfig) string {
	switch storage.Pattern {
	case spec.StoragePatternScoped:
		return fmt.Sprintf(", %s string, name string", storage.ScopeBy)
	case spec.StoragePatternSingleton:
		return ""
	default:
		return ", name string"
	}
}

func listParams(storage spec.StorageConfig) string {
	switch storage.Pattern {
	case spec.StoragePatternScoped:
		return fmt.Sprintf(", %s string, pageSize int64, pageToken string", storage.ScopeBy)
	default:
		return ", pageSize int64, pageToken string"
	}
}
