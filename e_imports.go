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
	_ "github.com/alecthomas/kingpin/v2"
	_ "github.com/beevik/etree"
	_ "github.com/coreos/go-oidc/oidc"
	_ "github.com/go-piv/piv-go/piv"
	_ "github.com/google/go-attestation/attest"
	_ "github.com/gravitational/form"
	_ "github.com/gravitational/license"
	_ "github.com/gravitational/roundtrip"
	_ "github.com/gravitational/trace"
	_ "github.com/gravitational/trace/trail"
	_ "github.com/jonboulle/clockwork"
	_ "github.com/julienschmidt/httprouter"
	_ "github.com/mitchellh/mapstructure"
	_ "github.com/okta/okta-sdk-golang/v2/okta"
	_ "golang.org/x/time/rate"
	_ "google.golang.org/api/admin/directory/v1"
	_ "google.golang.org/api/cloudidentity/v1"
	_ "google.golang.org/genproto/googleapis/rpc/status"
	_ "gopkg.in/check.v1"
	_ "k8s.io/apimachinery/pkg/util/yaml"

	_ "github.com/gravitational/teleport/api/breaker"
	_ "github.com/gravitational/teleport/api/client"
	_ "github.com/gravitational/teleport/api/client/proto"
	_ "github.com/gravitational/teleport/api/client/webclient"
	_ "github.com/gravitational/teleport/api/constants"
	_ "github.com/gravitational/teleport/api/defaults"
	_ "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	_ "github.com/gravitational/teleport/api/types"
	_ "github.com/gravitational/teleport/api/types/events"
	_ "github.com/gravitational/teleport/api/types/wrappers"
	_ "github.com/gravitational/teleport/api/utils"
	_ "github.com/gravitational/teleport/api/utils/keys"
	_ "github.com/gravitational/teleport/api/utils/retryutils"
)
