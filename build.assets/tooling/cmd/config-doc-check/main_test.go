// Teleport
// Copyright (C) 2026  Gravitational, Inc.
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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigKeyTreeBuilder(t *testing.T) {
	rootAbs, err := filepath.Abs("../../../..")
	require.NoError(t, err)
	treeBuilder := &configKeyTreeBuilder{
		rootPath:        rootAbs,
		sourcePackages:  make(map[string]*sourcePackage),
		treeCache:       make(map[string]*yamlKeyTree),
		inProgressTypes: make(map[string]bool),
	}

	authTree, err := treeBuilder.treeForTypeName(fmt.Sprintf("%s/lib/config", teleportPackagePrefix), "Auth")
	require.NoError(t, err)
	require.Contains(t, authTree.children, "enabled")
	require.Contains(t, authTree.children, "proxy_checks_host_keys")
	require.Nil(t, authTree.children["proxy_checks_host_keys"])
	require.Contains(t, authTree.children["authentication"].children, "second_factor")

	globalTree, err := treeBuilder.treeForTypeName(fmt.Sprintf("%s/lib/config", teleportPackagePrefix), "Global")
	require.NoError(t, err)
	require.Contains(t, globalTree.children, "data_dir")
	require.NotContains(t, globalTree.children, "datadir")
	require.Contains(t, globalTree.children["join_params"].children, "method")

	sshTree, err := treeBuilder.treeForTypeName(fmt.Sprintf("%s/lib/config", teleportPackagePrefix), "SSH")
	require.NoError(t, err)
	require.True(t, sshTree.children["labels"].hasInlineMap)

	discoveryTree, err := treeBuilder.treeForTypeName(fmt.Sprintf("%s/lib/config", teleportPackagePrefix), "Discovery")
	require.NoError(t, err)
	require.Contains(t, discoveryTree.children["aws"].children, "types")
}

func TestLoadCheckerConfig(t *testing.T) {
	config, err := loadConfigFile("config.yaml")
	require.NoError(t, err)
	require.Len(t, config.ServiceSections, 12)
	require.Equal(t, serviceSectionInfo{
		Name:        "Instance-wide settings",
		ExamplePath: "docs/pages/includes/config-reference/instance-wide.yaml",
		SectionKey:  "teleport",
		TypeName:    "Global",
	}, config.ServiceSections[0])
}

func TestLoadCheckerConfigErrors(t *testing.T) {
	validSection := `
  - name: Auth Service
    example_path: auth-service.yaml
    section_key: auth_service
    type_name: Auth
`
	tests := []struct {
		name        string
		contents    string
		missingFile bool
		wantError   string
	}{
		{
			name:        "file does not exist",
			missingFile: true,
			wantError:   "opening configuration file",
		},
		{
			name:      "malformed YAML",
			contents:  "source_path: [",
			wantError: "parsing configuration file",
		},
		{
			name:      "missing source path",
			contents:  "service_sections:" + validSection,
			wantError: "checker config has no source path",
		},
		{
			name:      "missing service sections",
			contents:  "source_path: .\n",
			wantError: "checker config has no service sections",
		},
		{
			name: "missing section name",
			contents: `source_path: .
service_sections:
  - example_path: auth-service.yaml
    section_key: auth_service
    type_name: Auth
`,
			wantError: "must define name, example_path, section_key, and type_name",
		},
		{
			name: "missing example path",
			contents: `source_path: .
service_sections:
  - name: Auth Service
    section_key: auth_service
    type_name: Auth
`,
			wantError: "must define name, example_path, section_key, and type_name",
		},
		{
			name: "missing section key",
			contents: `source_path: .
service_sections:
  - name: Auth Service
    example_path: auth-service.yaml
    type_name: Auth
`,
			wantError: "must define name, example_path, section_key, and type_name",
		},
		{
			name: "missing type name",
			contents: `source_path: .
service_sections:
  - name: Auth Service
    example_path: auth-service.yaml
    section_key: auth_service
`,
			wantError: "must define name, example_path, section_key, and type_name",
		},
		{
			name: "duplicate section key",
			contents: `source_path: .
service_sections:
  - name: Auth Service
    example_path: auth-service.yaml
    section_key: auth_service
    type_name: Auth
  - name: Another Auth Service
    example_path: another-auth-service.yaml
    section_key: auth_service
    type_name: Auth
`,
			wantError: `duplicate service section key "auth_service"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			if !test.missingFile {
				require.NoError(t, os.WriteFile(configPath, []byte(test.contents), 0o600))
			}

			config, err := loadConfigFile(configPath)
			require.Nil(t, config)
			require.ErrorContains(t, err, test.wantError)
		})
	}
}

func TestCompareTrees(t *testing.T) {
	structTree := &yamlKeyTree{children: map[string]*yamlKeyTree{
		"shared": {children: map[string]*yamlKeyTree{
			"struct_only": nil,
		}},
		"missing": nil,
	}}
	exampleTree := &yamlKeyTree{children: map[string]*yamlKeyTree{
		"shared": {children: map[string]*yamlKeyTree{
			"doc_only": nil,
		}},
		"extra": nil,
	}}

	require.Equal(t, []difference{
		{
			Path:         "service",
			OnlyInStruct: []string{"missing"},
			OnlyInDoc:    []string{"extra"},
		},
		{
			Path:         "service.shared",
			OnlyInStruct: []string{"struct_only"},
			OnlyInDoc:    []string{"doc_only"},
		},
	}, compareTrees(structTree, exampleTree, "service"))
}
