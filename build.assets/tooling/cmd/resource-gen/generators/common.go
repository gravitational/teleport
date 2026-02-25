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
	"strings"
	"text/template"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

// resourceBase holds the common fields used by all generator data structs.
type resourceBase struct {
	Kind             string // PascalCase, e.g. "Foo"
	Lower            string // lowercase, e.g. "foo"
	Plural           string // e.g. "Foos"
	PkgAlias         string // e.g. "foov1"
	ImportPath       string // e.g. "github.com/.../foo/v1"
	QualType         string // e.g. "*foov1.Foo"
	Module           string
	IsSingleton      bool
	SingletonName    string
	IsScoped         bool
	ScopeBy          string // param name, e.g. "namespace"
	ScopeByPascal    string // PascalCase, e.g. "Namespace" (for proto getters)
	Ops              spec.OperationSet
	FullServiceName  string // e.g. "teleport.foo.v1.FooService"
	ShortServiceName string // e.g. "FooService"
	Article          string // "a" or "an" based on first letter of kind
	CacheEnabled         bool // true if cache collection generation is enabled
	HasAudit             bool // true if any audit event will be emitted
	EnableLifecycleHooks bool // true if lifecycle hooks are enabled
}

// indefiniteArticle returns "an" if word starts with a vowel, "a" otherwise.
func indefiniteArticle(word string) string {
	if len(word) > 0 && strings.ContainsRune("AEIOUaeiou", rune(word[0])) {
		return "an"
	}
	return "a"
}

// newResourceBase creates a resourceBase from a ResourceSpec.
func newResourceBase(rs spec.ResourceSpec, module string) resourceBase {
	kind := rs.KindPascal
	pkgAlias := protoPackageAlias(rs.ServiceName)
	return resourceBase{
		Kind:             kind,
		Lower:            rs.Kind,
		Plural:           pluralize(kind),
		PkgAlias:         pkgAlias,
		ImportPath:       protoGoImportPath(rs.ServiceName, module),
		QualType:         fmt.Sprintf("*%s.%s", pkgAlias, kind),
		Module:           module,
		IsSingleton:      rs.Storage.Pattern == spec.StoragePatternSingleton,
		SingletonName:    rs.Storage.SingletonName,
		IsScoped:         rs.Storage.Pattern == spec.StoragePatternScoped,
		ScopeBy:          rs.Storage.ScopeBy,
		ScopeByPascal:    snakeToCamel(rs.Storage.ScopeBy),
		Article:          indefiniteArticle(kind),
		CacheEnabled:     rs.Cache.Enabled,
		Ops:              rs.Operations,
		FullServiceName:  rs.ServiceName,
		ShortServiceName: serviceShortName(rs.ServiceName),
		HasAudit: (rs.Operations.Get && rs.Audit.EmitOnGet) ||
			(rs.Operations.Create && rs.Audit.EmitOnCreate) ||
			(rs.Operations.Update && rs.Audit.EmitOnUpdate) ||
			(rs.Operations.Upsert && rs.Audit.EmitOnUpdate) ||
			(rs.Operations.Delete && rs.Audit.EmitOnDelete),
		EnableLifecycleHooks: rs.Hooks.EnableLifecycleHooks,
	}
}

// render executes a text/template against data and returns the result.
func render(name string, tmpl string, data any) (string, error) {
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return "", trace.Wrap(err)
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", trace.Wrap(err)
	}
	return b.String(), nil
}
