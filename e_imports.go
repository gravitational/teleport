//go:build e_imports && !e_imports

// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package teleport

// This file should import all non-stdlib, non-teleport packages that are
// imported by any package in ./e/ but not by packages in the rest of the main
// teleport module, so tidying that doesn't have access to teleport.e (like
// Dependabot) doesn't wrongly remove the modules they belong to.

// Remember to check that e is up to date and that there is not a go.work file
// before running the following command to generate the import list. The list of
// tags that might be needed in e (currently only "piv") can be extracted with a
// (cd e && git grep //go:build).

// TODO(espadolini): turn this into a lint (needs access to teleport.e in CI and
// ideally a resolution to https://github.com/golang/go/issues/42504 )

/*
comm -13 <(
	go list ./... | sort -u | grep -Ev -e "^github.com/gravitational/teleport/e(/.*)?$" |
	xargs go list -f '{{range .Imports}}{{println .}}{{end}}' |
	sort -u | grep -Ev -e "^github.com/gravitational/teleport(/.*)?$" -e "^C$" |
	xargs go list -f '{{if not .Standard}}{{println .ImportPath}}{{end}}'
) <(
	go list -f '{{range .Imports}}{{println .}}{{end}}' -tags piv ./e/... |
	sort -u | grep -Ev -e "^github.com/gravitational/teleport(/.*)?$" -e "^C$" |
	xargs go list -f '{{if not .Standard}}{{println .ImportPath}}{{end}}'
) | awk '{ print "\t_ \"" $1 "\"" }'
*/

import (
	_ "github.com/go-piv/piv-go/piv"
	_ "github.com/gravitational/license"
	_ "gopkg.in/check.v1"
)
