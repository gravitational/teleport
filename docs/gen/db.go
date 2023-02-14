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
package main

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
)

func genDBCreateUserDBNameWarning() error {
	var protcolsRequireDBName []string
	for _, protcol := range defaults.DatabaseProtocols {
		if role.RequireDatabaseNameMatcher(protcol) {
			protcolsRequireDBName = append(protcolsRequireDBName, defaults.ReadableDatabaseProtocol(protcol))
		}
	}

	return trace.Wrap(generateMDX(
		outputPath("database-access", "create-user-db-name-warning.mdx"),
		`<Admonition type="warning">
Database names are only enforced for {{ andSlice . }} databases.
</Admonition>
`,
		protcolsRequireDBName,
	))
}

func genDBReferenceTCLAuthSign() error {
	return trace.Wrap(generateMDX(
		outputPath("database-access", "reference", "tctl-auth-sign-flags.mdx"),
		`| Flag | Defaut | Description |
| - | - | - |
{{ range $index, $flag := . }}{{- if not $flag.Hidden -}}
| {{ flagName $flag }} | {{ flagDefault $flag }} | {{ $flag.Help }} |
{{end}}{{end}}
`,
		tctlApp.GetCommand("auth").GetCommand("sign").Model().Flags,
	))
}
