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
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

// registrationData holds common fields used by most registration templates.
// FullServiceName and ShortServiceName are inherited from resourceBase.
type registrationData struct {
	resourceBase
}

func newRegistrationData(rs spec.ResourceSpec, module string) registrationData {
	return registrationData{
		resourceBase: newResourceBase(rs, module),
	}
}

// authRegistrationData extends registrationData with auth-specific fields.
type authRegistrationData struct {
	registrationData
	ProtoAlias    string
	SvcAlias      string
	SvcImportPath string
}

var authRegistrationTmpl = mustReadTemplate("auth_registration.go.tmpl")

// GenerateAuthRegistration renders an init()-based auth gRPC registration
// that wires up the generated service with the auth server.
func GenerateAuthRegistration(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	base := newRegistrationData(rs, module)
	resource, pkgDir := ServicePathParts(rs)

	data := authRegistrationData{
		registrationData: base,
		ProtoAlias:       base.PkgAlias + "pb",
		SvcAlias:         base.PkgAlias,
		SvcImportPath:    module + "/lib/auth/" + resource + "/" + pkgDir,
	}

	return render("authRegistration", authRegistrationTmpl, data)
}

// localParserData holds fields for the local events parser template.
type localParserData struct {
	registrationData
	PrefixConst string
}

var localParserRegistrationTmpl = mustReadTemplate("local_parser.go.tmpl")

// GenerateLocalParserRegistration renders an init()-based local events parser
// that handles OpDelete and OpPut for the resource kind.
func GenerateLocalParserRegistration(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	base := newRegistrationData(rs, module)
	data := localParserData{
		registrationData: base,
		PrefixConst:      base.Lower + "Prefix",
	}

	return render("localParserRegistration", localParserRegistrationTmpl, data)
}

var cacheRegistrationTmpl = mustReadTemplate("cache_registration.go.tmpl")

// GenerateCacheRegistration renders an init()-based cache collection registration
// with a fully-wired collection builder.
func GenerateCacheRegistration(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	data := newRegistrationData(rs, module)
	return render("cacheRegistration", cacheRegistrationTmpl, data)
}

// tctlRegistrationData extends registrationData with tctl-specific fields.
type tctlRegistrationData struct {
	registrationData
	Description        string
	MFARequired        bool
	Columns            []columnDef
	VerboseColumns     []columnDef
	HasTimestampColumn bool
}

var tctlRegistrationTmpl = mustReadTemplate("tctl_registration.go.tmpl")

// GenerateTCTLRegistration renders an init()-based tctl handler with full
// CRUD operations and a collection type for text output.
func GenerateTCTLRegistration(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	base := newRegistrationData(rs, module)
	description := rs.Tctl.Description
	if description == "" {
		description = base.Kind + " resources."
	}

	timestampSet := make(map[string]bool, len(rs.Tctl.TimestampColumns))
	for _, tc := range rs.Tctl.TimestampColumns {
		timestampSet[tc] = true
	}

	columns := resolveColumns(rs.Tctl.Columns, timestampSet)
	verboseColumns := resolveColumns(rs.Tctl.VerboseColumns, timestampSet)

	hasTimestamp := false
	for _, c := range columns {
		if c.IsTimestamp {
			hasTimestamp = true
			break
		}
	}
	if !hasTimestamp {
		for _, c := range verboseColumns {
			if c.IsTimestamp {
				hasTimestamp = true
				break
			}
		}
	}

	data := tctlRegistrationData{
		registrationData:   base,
		Description:        description,
		MFARequired:        rs.Tctl.MFARequired,
		Columns:            columns,
		VerboseColumns:     verboseColumns,
		HasTimestampColumn: hasTimestamp,
	}

	return render("tctlRegistration", tctlRegistrationTmpl, data)
}

var cacheTestRegistrationTmpl = mustReadTemplate("cache_test_registration.go.tmpl")

// GenerateCacheTestRegistration renders an init()-based test resource
// registration for cache_test.go infrastructure.
func GenerateCacheTestRegistration(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	data := newRegistrationData(rs, module)
	return render("cacheTestRegistration", cacheTestRegistrationTmpl, data)
}

// cacheAccessorsData extends registrationData for cache accessor generation.
// QualType and IsSingleton are inherited from the embedded resourceBase.
type cacheAccessorsData struct {
	registrationData
}

var cacheAccessorsTmpl = mustReadTemplate("cache_accessors.go.tmpl")

// GenerateCacheAccessors renders cache Get/List methods on *Cache that use
// genericGetter/genericLister with the collection from the byKind registry.
func GenerateCacheAccessors(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	data := cacheAccessorsData{
		registrationData: newRegistrationData(rs, module),
	}

	return render("cacheAccessors", cacheAccessorsTmpl, data)
}
