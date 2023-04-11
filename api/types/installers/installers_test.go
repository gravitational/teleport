/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package installers

import (
	"html/template"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestInstallers(t *testing.T) {
	for _, installer := range []types.Installer{
		DefaultAgentlessInstaller,
		DefaultInstaller,
	} {
		for _, tt := range []struct {
			name     string
			tmpl     Template
			contains []string
		}{
			{
				name: "with major version",
				tmpl: Template{
					PublicProxyAddr: "https://localhost:3080/",
					MajorVersion:    "v12",
					TeleportPackage: "teleport",
				},
				contains: []string{
					`if [ -n "v12" ]`,
					"then\n      REPO_CHANNEL=\"stable/v12\"",
				},
			},
			{
				name: "without major version, stable/cloud is used",
				tmpl: Template{
					PublicProxyAddr: "https://localhost:3080/",
					TeleportPackage: "teleport",
					RepoChannel:     "stable/cloud",
				},
				contains: []string{
					`if [ -n "" ]`,
					"then\n      REPO_CHANNEL=\"stable/\"\n",
				},
			},
		} {
			t.Run(installer.GetName()+"/"+tt.name, func(t *testing.T) {
				instTmpl, err := template.New("").Parse(installer.GetScript())
				require.NoError(t, err)

				var buf strings.Builder
				err = instTmpl.Execute(&buf, tt.tmpl)
				require.NoError(t, err)

				script := buf.String()

				for _, expectedString := range tt.contains {
					require.Contains(t, script, expectedString)
				}
			})
		}
	}
}
