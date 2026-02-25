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
	"path/filepath"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
)

// Generator describes a single code generator with its output path and conditions.
type Generator struct {
	Name         string
	PathFunc     func(kind string, rs spec.ResourceSpec) string
	Generate     func(rs spec.ResourceSpec, module string) (string, error)
	SkipIfExists bool
	Condition    func(rs spec.ResourceSpec) bool // nil means always run
}

// Generators returns the ordered list of all registered generators.
func Generators() []Generator {
	return []Generator{
		{
			Name:     "service-interface",
			PathFunc: func(kind string, _ spec.ResourceSpec) string { return filepath.Join("lib", "services", kind+".gen.go") },
			Generate: GenerateServiceInterface,
		},
		{
			Name:     "backend-impl",
			PathFunc: func(kind string, _ spec.ResourceSpec) string { return filepath.Join("lib", "services", "local", kind+".gen.go") },
			Generate: GenerateBackendImplementation,
		},
		{
			Name: "grpc-service",
			PathFunc: func(_ string, rs spec.ResourceSpec) string {
				resource, pkgDir := ServicePathParts(rs)
				return filepath.Join("lib", "auth", resource, pkgDir, "service.gen.go")
			},
			Generate: GenerateGRPCService,
		},
		{
			Name: "grpc-service-custom",
			PathFunc: func(_ string, rs spec.ResourceSpec) string {
				resource, pkgDir := ServicePathParts(rs)
				return filepath.Join("lib", "auth", resource, pkgDir, "service.go")
			},
			Generate:     GenerateGRPCServiceCustom,
			SkipIfExists: true,
		},
		{
			Name:         "validation",
			PathFunc:     func(kind string, _ spec.ResourceSpec) string { return filepath.Join("lib", "services", kind+".go") },
			Generate:     GenerateValidation,
			SkipIfExists: true,
		},
		{
			Name:         "validation-test",
			PathFunc:     func(kind string, _ spec.ResourceSpec) string { return filepath.Join("lib", "services", kind+"_test.go") },
			Generate:     GenerateValidationTest,
			SkipIfExists: true,
		},
		{
			Name:     "api-client",
			PathFunc: func(kind string, _ spec.ResourceSpec) string { return filepath.Join("api", "client", kind+".gen.go") },
			Generate: GenerateAPIClient,
		},
		{
			Name:     "auth-registration",
			PathFunc: func(kind string, _ spec.ResourceSpec) string { return filepath.Join("lib", "auth", kind+"_register.gen.go") },
			Generate: GenerateAuthRegistration,
		},
		{
			Name:     "local-parser",
			PathFunc: func(kind string, _ spec.ResourceSpec) string { return filepath.Join("lib", "services", "local", kind+"_register.gen.go") },
			Generate: GenerateLocalParserRegistration,
		},
		{
			Name:      "cache-registration",
			PathFunc:  func(kind string, _ spec.ResourceSpec) string { return filepath.Join("lib", "cache", kind+"_register.gen.go") },
			Generate:  GenerateCacheRegistration,
			Condition: func(rs spec.ResourceSpec) bool { return rs.Cache.Enabled },
		},
		{
			Name:      "cache-accessors",
			PathFunc:  func(kind string, _ spec.ResourceSpec) string { return filepath.Join("lib", "cache", kind+".gen.go") },
			Generate:  GenerateCacheAccessors,
			Condition: func(rs spec.ResourceSpec) bool { return rs.Cache.Enabled },
		},
		{
			Name:      "cache-test-registration",
			PathFunc:  func(kind string, _ spec.ResourceSpec) string { return filepath.Join("lib", "cache", kind+"_register.gen_test.go") },
			Generate:  GenerateCacheTestRegistration,
			Condition: func(rs spec.ResourceSpec) bool { return rs.Cache.Enabled },
		},
		{
			Name:      "tctl-registration",
			PathFunc:  func(kind string, _ spec.ResourceSpec) string { return filepath.Join("tool", "tctl", "common", "resources", kind+"_register.gen.go") },
			Generate:  GenerateTCTLRegistration,
			Condition: func(rs spec.ResourceSpec) bool { return rs.Storage.Pattern != spec.StoragePatternScoped },
		},
		// NOTE: The events-proto-scaffold generator was removed because event
		// messages must live inside events.proto (for OneOf field numbers).
		// Separate scaffold protos would duplicate the definitions and cause
		// compile errors. Event messages are added to events.proto manually.
	}
}
