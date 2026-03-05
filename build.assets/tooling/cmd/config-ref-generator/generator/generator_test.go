// Teleport
// Copyright (C) 2025  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package generator

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// replaceBackticks replaces the "BACKTICK" placeholder with actual backticks so
// we can include struct tags within inline Go source fixtures without Go
// interpreting the backticks as raw string delimiters.
func replaceBackticks(source string) string {
	return strings.ReplaceAll(source, "BACKTICK", "`")
}

// parseFixture parses a Go source string and returns a map of PackageInfo to
// DeclarationInfo for every type declaration found.
func parseFixture(t *testing.T, pkg, source string) map[PackageInfo]DeclarationInfo {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", source, parser.ParseComments)
	require.NoError(t, err)

	decls := make(map[PackageInfo]DeclarationInfo)
	ni := namedImports(file)
	for _, d := range file.Decls {
		l, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		if len(l.Specs) != 1 {
			continue
		}
		spec, ok := l.Specs[0].(*ast.TypeSpec)
		if !ok {
			continue
		}
		decls[PackageInfo{
			DeclName:    spec.Name.Name,
			PackagePath: pkg,
		}] = DeclarationInfo{
			Decl:         l,
			FilePath:     "fixture.go",
			PackageName:  pkg,
			NamedImports: ni,
		}
	}
	return decls
}

func TestGetYAMLTag(t *testing.T) {
	cases := []struct {
		desc     string
		tags     string
		expected string
	}{
		{
			desc:     "simple field name",
			tags:     replaceBackticks("BACKTICKyaml:\"field_name\"BACKTICK"),
			expected: "field_name",
		},
		{
			desc:     "field name with omitempty",
			tags:     replaceBackticks("BACKTICKyaml:\"field_name,omitempty\"BACKTICK"),
			expected: "field_name",
		},
		{
			desc:     "skip marker",
			tags:     replaceBackticks("BACKTICKyaml:\"-\"BACKTICK"),
			expected: "-",
		},
		{
			desc:     "inline only, no name",
			tags:     replaceBackticks("BACKTICKyaml:\",inline\"BACKTICK"),
			expected: "",
		},
		{
			desc:     "omitempty only, no name",
			tags:     replaceBackticks("BACKTICKyaml:\",omitempty\"BACKTICK"),
			expected: "",
		},
		{
			desc:     "no yaml tag",
			tags:     replaceBackticks("BACKTICKjson:\"other_field\"BACKTICK"),
			expected: "",
		},
		{
			desc:     "empty tags",
			tags:     "",
			expected: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.expected, getYAMLTag(tc.tags))
		})
	}
}

func TestIsYAMLInline(t *testing.T) {
	cases := []struct {
		desc     string
		tags     string
		expected bool
	}{
		{
			desc:     "inline only",
			tags:     replaceBackticks("BACKTICKyaml:\",inline\"BACKTICK"),
			expected: true,
		},
		{
			desc:     "name with inline",
			tags:     replaceBackticks("BACKTICKyaml:\"name,inline\"BACKTICK"),
			expected: true,
		},
		{
			desc:     "name without inline",
			tags:     replaceBackticks("BACKTICKyaml:\"name\"BACKTICK"),
			expected: false,
		},
		{
			desc:     "name with omitempty but not inline",
			tags:     replaceBackticks("BACKTICKyaml:\"name,omitempty\"BACKTICK"),
			expected: false,
		},
		{
			desc:     "no yaml tag",
			tags:     "",
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.expected, isYAMLInline(tc.tags))
		})
	}
}

func TestYAMLKeyToTitle(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"auth_service", "Auth Service"},
		{"ssh_service", "Ssh Service"},
		{"windows_desktop_service", "Windows Desktop Service"},
		{"db_service", "Db Service"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, yamlKeyToTitle(tc.input))
		})
	}
}

func TestReferenceDataFromDeclaration_ScalarFields(t *testing.T) {
	const pkg = "example.com/config"
	source := replaceBackticks(`
package config

// SSH is the 'ssh_service' section of the config file.
type SSH struct {
	// EnabledFlag enables or disables the SSH service.
	EnabledFlag string BACKTICKyaml:"enabled,omitempty"BACKTICK
	// ListenAddress is the address the SSH service listens on.
	ListenAddress string BACKTICKyaml:"listen_addr,omitempty"BACKTICK
	// MaxConnections is the maximum number of concurrent connections.
	MaxConnections int64 BACKTICKyaml:"max_connections,omitempty"BACKTICK
}
`)
	allDecls := parseFixture(t, pkg, source)

	decl := allDecls[PackageInfo{DeclName: "SSH", PackagePath: pkg}]
	refs, err := ReferenceDataFromDeclaration("example.com", decl, allDecls, nil)
	require.NoError(t, err)

	entry, ok := refs[PackageInfo{DeclName: "SSH", PackagePath: pkg}]
	require.True(t, ok, "expected entry for SSH")

	assert.Equal(t, "SSH", entry.SectionName)
	assert.Contains(t, entry.Description, "ssh_service")

	require.Equal(t, 3, len(entry.Fields))
	fieldNames := make([]string, len(entry.Fields))
	for i, f := range entry.Fields {
		fieldNames[i] = f.Name
	}
	// Fields are sorted alphabetically.
	assert.Equal(t, []string{"enabled", "listen_addr", "max_connections"}, fieldNames)

	assert.Contains(t, entry.YAMLExample, "enabled:")
	assert.Contains(t, entry.YAMLExample, "listen_addr:")
	assert.Contains(t, entry.YAMLExample, "max_connections:")
}

func TestReferenceDataFromDeclaration_SkipsIgnoredAndUnexported(t *testing.T) {
	const pkg = "example.com/config"
	source := replaceBackticks(`
package config

type MyService struct {
	// Enabled is exported and included.
	Enabled string BACKTICKyaml:"enabled"BACKTICK
	// Internal is excluded by yaml:"-".
	Internal string BACKTICKyaml:"-"BACKTICK
	// private is unexported.
	private string BACKTICKyaml:"private"BACKTICK
}
`)
	allDecls := parseFixture(t, pkg, source)
	decl := allDecls[PackageInfo{DeclName: "MyService", PackagePath: pkg}]
	refs, err := ReferenceDataFromDeclaration("example.com", decl, allDecls, nil)
	require.NoError(t, err)

	entry := refs[PackageInfo{DeclName: "MyService", PackagePath: pkg}]
	require.Equal(t, 1, len(entry.Fields))
	assert.Equal(t, "enabled", entry.Fields[0].Name)
}

func TestReferenceDataFromDeclaration_InlineEmbeddedStruct(t *testing.T) {
	const pkg = "example.com/config"
	source := replaceBackticks(`
package config

// Service is the common configuration for a Teleport service.
type Service struct {
	// EnabledFlag enables or disables the service.
	EnabledFlag string BACKTICKyaml:"enabled,omitempty"BACKTICK
	// ListenAddress is the address the service listens on.
	ListenAddress string BACKTICKyaml:"listen_addr,omitempty"BACKTICK
}

// Auth is the 'auth_service' section.
type Auth struct {
	Service BACKTICKyaml:",inline"BACKTICK
	// ClusterName is the name of the CA cluster.
	ClusterName string BACKTICKyaml:"cluster_name,omitempty"BACKTICK
}
`)
	allDecls := parseFixture(t, pkg, source)
	decl := allDecls[PackageInfo{DeclName: "Auth", PackagePath: pkg}]
	refs, err := ReferenceDataFromDeclaration("example.com", decl, allDecls, nil)
	require.NoError(t, err)

	entry := refs[PackageInfo{DeclName: "Auth", PackagePath: pkg}]

	// Inlined Service fields appear alongside Auth's own fields.
	fieldNames := make([]string, len(entry.Fields))
	for i, f := range entry.Fields {
		fieldNames[i] = f.Name
	}
	assert.Contains(t, fieldNames, "enabled")
	assert.Contains(t, fieldNames, "listen_addr")
	assert.Contains(t, fieldNames, "cluster_name")

	// Service must NOT appear as a separate sub-section because it is inlined.
	_, hasService := refs[PackageInfo{DeclName: "Service", PackagePath: pkg}]
	assert.False(t, hasService, "inlined Service struct should not have its own sub-section")
}

func TestReferenceDataFromDeclaration_AnonymousEmbeddedStruct(t *testing.T) {
	const pkg = "example.com/config"
	// No explicit yaml tag — yaml.v2 inlines anonymous embedded fields by default.
	source := replaceBackticks(`
package config

type Base struct {
	// Name is the identifier.
	Name string BACKTICKyaml:"name"BACKTICK
}

type Child struct {
	Base
	// Extra is an additional field.
	Extra string BACKTICKyaml:"extra"BACKTICK
}
`)
	allDecls := parseFixture(t, pkg, source)
	decl := allDecls[PackageInfo{DeclName: "Child", PackagePath: pkg}]
	refs, err := ReferenceDataFromDeclaration("example.com", decl, allDecls, nil)
	require.NoError(t, err)

	entry := refs[PackageInfo{DeclName: "Child", PackagePath: pkg}]
	fieldNames := make([]string, len(entry.Fields))
	for i, f := range entry.Fields {
		fieldNames[i] = f.Name
	}
	assert.Contains(t, fieldNames, "name")
	assert.Contains(t, fieldNames, "extra")
}

func TestReferenceDataFromDeclaration_NestedCustomType(t *testing.T) {
	const pkg = "example.com/config"
	source := replaceBackticks(`
package config

// AuthenticationConfig holds authentication configuration.
type AuthenticationConfig struct {
	// Type is the authentication type.
	Type string BACKTICKyaml:"type"BACKTICK
}

// Auth is the 'auth_service' section.
type Auth struct {
	// Authentication holds authentication configuration information.
	Authentication *AuthenticationConfig BACKTICKyaml:"authentication,omitempty"BACKTICK
}
`)
	allDecls := parseFixture(t, pkg, source)
	decl := allDecls[PackageInfo{DeclName: "Auth", PackagePath: pkg}]
	refs, err := ReferenceDataFromDeclaration("example.com", decl, allDecls, nil)
	require.NoError(t, err)

	// The root entry's field should reference AuthenticationConfig.
	root := refs[PackageInfo{DeclName: "Auth", PackagePath: pkg}]
	require.Equal(t, 1, len(root.Fields))
	assert.Equal(t, "authentication", root.Fields[0].Name)
	assert.Contains(t, root.Fields[0].Type, "Authentication")

	// A separate sub-section must exist for AuthenticationConfig.
	authEntry, ok := refs[PackageInfo{DeclName: "AuthenticationConfig", PackagePath: pkg}]
	require.True(t, ok, "expected sub-section for AuthenticationConfig")
	require.Equal(t, 1, len(authEntry.Fields))
	assert.Equal(t, "type", authEntry.Fields[0].Name)
}

func TestReferenceDataFromDeclaration_YAMLExample(t *testing.T) {
	const pkg = "example.com/config"
	source := replaceBackticks(`
package config

type Proxy struct {
	// Enabled enables the Proxy service.
	Enabled string BACKTICKyaml:"enabled"BACKTICK
	// PublicAddr is the public address of the Proxy.
	PublicAddr []string BACKTICKyaml:"public_addr,omitempty"BACKTICK
	// MaxConns is the max connection count.
	MaxConns int BACKTICKyaml:"max_conns"BACKTICK
	// TLSEnabled enables TLS.
	TLSEnabled bool BACKTICKyaml:"tls_enabled"BACKTICK
}
`)
	allDecls := parseFixture(t, pkg, source)
	decl := allDecls[PackageInfo{DeclName: "Proxy", PackagePath: pkg}]
	refs, err := ReferenceDataFromDeclaration("example.com", decl, allDecls, nil)
	require.NoError(t, err)

	entry := refs[PackageInfo{DeclName: "Proxy", PackagePath: pkg}]
	assert.Contains(t, entry.YAMLExample, `enabled: "string"`)
	assert.Contains(t, entry.YAMLExample, "max_conns: 1")
	assert.Contains(t, entry.YAMLExample, "tls_enabled: true")
	assert.Contains(t, entry.YAMLExample, "public_addr:")
}

func TestGenerate(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	goSource := `package config

// FileConfig is the top-level configuration struct.
type FileConfig struct {
	Auth Auth ` + "`" + `yaml:"auth_service,omitempty"` + "`" + `
	SSH  SSH  ` + "`" + `yaml:"ssh_service,omitempty"` + "`" + `
}

// Auth is the auth_service section.
type Auth struct {
	// EnabledFlag enables or disables the Auth Service.
	EnabledFlag string ` + "`" + `yaml:"enabled,omitempty"` + "`" + `
	// ListenAddress is the address the Auth Service listens on.
	ListenAddress string ` + "`" + `yaml:"listen_addr,omitempty"` + "`" + `
}

// SSH is the ssh_service section.
type SSH struct {
	// EnabledFlag enables or disables the SSH Service.
	EnabledFlag string ` + "`" + `yaml:"enabled,omitempty"` + "`" + `
}
`
	err := os.WriteFile(filepath.Join(srcDir, "fileconf.go"), []byte(goSource), 0644)
	require.NoError(t, err)

	tmplPath := filepath.Join(t.TempDir(), "generator.tmpl")
	err = os.WriteFile(tmplPath, []byte(`---
title: "{{ title .YAMLKey }}"
---
{{ .Root.Description }}
{{ range .Root.Fields }}{{ .Name }}
{{ end }}`), 0644)
	require.NoError(t, err)

	tmpl, err := NewTemplate(tmplPath)
	require.NoError(t, err)

	conf := GeneratorConfig{
		SourcePath:           srcDir,
		ModulePath:           srcDir,
		ModulePrefix:         "example.com",
		DestinationDirectory: destDir,
		EntryType:            "FileConfig",
		ServiceSuffix:        "_service",
	}

	err = Generate(conf, tmpl)
	require.NoError(t, err)

	entries, err := os.ReadDir(destDir)
	require.NoError(t, err)
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	assert.Contains(t, names, "auth-service.mdx")
	assert.Contains(t, names, "ssh-service.mdx")

	authContent, err := os.ReadFile(filepath.Join(destDir, "auth-service.mdx"))
	require.NoError(t, err)
	assert.Contains(t, string(authContent), "Auth Service")
	assert.Contains(t, string(authContent), "enabled")
	assert.Contains(t, string(authContent), "listen_addr")

	sshContent, err := os.ReadFile(filepath.Join(destDir, "ssh-service.mdx"))
	require.NoError(t, err)
	assert.Contains(t, string(sshContent), "Ssh Service")
	assert.Contains(t, string(sshContent), "enabled")
}

func TestGenerate_MissingEntryType(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := os.WriteFile(filepath.Join(srcDir, "config.go"), []byte("package config\n"), 0644)
	require.NoError(t, err)

	tmplPath := filepath.Join(t.TempDir(), "generator.tmpl")
	err = os.WriteFile(tmplPath, []byte(""), 0644)
	require.NoError(t, err)
	tmpl, err := NewTemplate(tmplPath)
	require.NoError(t, err)

	conf := GeneratorConfig{
		SourcePath:           srcDir,
		ModulePath:           srcDir,
		ModulePrefix:         "example.com",
		DestinationDirectory: destDir,
		EntryType:            "NonExistentConfig",
	}

	err = Generate(conf, tmpl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NonExistentConfig")
}

func TestIndentLines(t *testing.T) {
	input := "foo: bar\nbaz: qux\n\nend: true\n"
	expected := "  foo: bar\n  baz: qux\n\n  end: true\n"
	assert.Equal(t, expected, indentLines(input, "  "))
}

func TestSplitCamelCase(t *testing.T) {
	cases := []struct {
		input      string
		exceptions []string
		expected   string
	}{
		{"Auth", nil, "Auth"},
		{"AuthService", nil, "Auth Service"},
		{"TLSConfig", []string{"TLS"}, "TLS Config"},
		{"WindowsDesktopService", nil, "Windows Desktop Service"},
		{"AWSKMS", []string{"AWS", "KMS"}, "AWS KMS"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, splitCamelCase(tc.input, tc.exceptions))
		})
	}
}
