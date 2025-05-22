//go:build e_imports && !e_imports

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package teleport

// This file should import all non-stdlib, non-teleport packages that are
// imported by any package in ./e/, so tidying that doesn't have access to
// teleport.e (like Dependabot) doesn't wrongly remove the modules that the
// imported packages belong to.

// Remember to check that e is up to date and that there is not a go.work file
// before running the following command to generate the import list. The list of
// tags that might be needed in e (currently "piv" and "tpmsimulator") can be
// extracted with a (cd e && git grep //go:build).

// TODO(espadolini): turn this into a lint (needs access to teleport.e in CI and
// ideally a resolution to https://github.com/golang/go/issues/42504 )

/*
go list -f '
{{- range .Imports}}{{println .}}{{end -}}
{{- range .TestImports}}{{println .}}{{end -}}
{{- range .XTestImports}}{{println .}}{{end -}}
' -tags piv,tpmsimulator ./e/... |
sort -u |
xargs go list -find -f '{{if (and
(not .Standard)
(ne .Module.Path "github.com/gravitational/teleport")
)}}{{printf "\t_ \"%v\"" .ImportPath}}{{end}}'
*/

import (
	_ "connectrpc.com/connect"
	_ "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	_ "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	_ "github.com/aws/aws-sdk-go-v2/aws"
	_ "github.com/aws/aws-sdk-go-v2/aws/arn"
	_ "github.com/aws/aws-sdk-go-v2/config"
	_ "github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	_ "github.com/aws/aws-sdk-go-v2/service/athena"
	_ "github.com/aws/aws-sdk-go-v2/service/athena/types"
	_ "github.com/aws/aws-sdk-go-v2/service/glue"
	_ "github.com/aws/aws-sdk-go-v2/service/identitystore"
	_ "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	_ "github.com/aws/aws-sdk-go-v2/service/organizations"
	_ "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	_ "github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	_ "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	_ "github.com/aws/aws-sdk-go-v2/service/sts/types"
	_ "github.com/beevik/etree"
	_ "github.com/coreos/go-semver/semver"
	_ "github.com/crewjam/saml"
	_ "github.com/crewjam/saml/samlsp"
	_ "github.com/elimity-com/scim/schema"
	_ "github.com/ghodss/yaml"
	_ "github.com/go-jose/go-jose/v3"
	_ "github.com/go-jose/go-jose/v3/json"
	_ "github.com/go-piv/piv-go/piv"
	_ "github.com/gogo/protobuf/proto"
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/google/go-attestation/attest"
	_ "github.com/google/go-cmp/cmp"
	_ "github.com/google/go-cmp/cmp/cmpopts"
	_ "github.com/google/go-containerregistry/pkg/v1"
	_ "github.com/google/go-github/v70/github"
	_ "github.com/google/go-tpm-tools/simulator"
	_ "github.com/google/safetext/shsprintf"
	_ "github.com/google/uuid"
	_ "github.com/gravitational/license"
	_ "github.com/gravitational/license/constants"
	_ "github.com/gravitational/license/generate"
	_ "github.com/gravitational/roundtrip"
	_ "github.com/gravitational/trace"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/pgtype"
	_ "github.com/jackc/pgx/v5/pgtype/zeronull"
	_ "github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jcmturner/gokrb5/v8/gssapi"
	_ "github.com/jcmturner/gokrb5/v8/spnego"
	_ "github.com/jonboulle/clockwork"
	_ "github.com/julienschmidt/httprouter"
	_ "github.com/mitchellh/mapstructure"
	_ "github.com/okta/okta-sdk-golang/v2/okta"
	_ "github.com/okta/okta-sdk-golang/v2/okta/query"
	_ "github.com/patrickmn/go-cache"
	_ "github.com/pquerna/otp/totp"
	_ "github.com/prometheus/client_golang/prometheus"
	_ "github.com/russellhaering/gosaml2"
	_ "github.com/russellhaering/gosaml2/types"
	_ "github.com/russellhaering/goxmldsig"
	_ "github.com/scim2/filter-parser/v2"
	_ "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	_ "github.com/sigstore/sigstore-go/pkg/bundle"
	_ "github.com/sigstore/sigstore-go/pkg/root"
	_ "github.com/sigstore/sigstore-go/pkg/tuf"
	_ "github.com/sigstore/sigstore-go/pkg/verify"
	_ "github.com/sigstore/sigstore/pkg/signature"
	_ "github.com/sijms/go-ora/v2"
	_ "github.com/sijms/go-ora/v2/network"
	_ "github.com/stretchr/testify/assert"
	_ "github.com/stretchr/testify/mock"
	_ "github.com/stretchr/testify/require"
	_ "github.com/stretchr/testify/suite"
	_ "github.com/vulcand/predicate/builder"
	_ "github.com/zitadel/oidc/v3/example/server/storage"
	_ "github.com/zitadel/oidc/v3/pkg/client/rp"
	_ "github.com/zitadel/oidc/v3/pkg/oidc"
	_ "github.com/zitadel/oidc/v3/pkg/op"
	_ "gitlab.com/gitlab-org/api/client-go"
	_ "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	_ "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/filters"
	_ "golang.org/x/crypto/bcrypt"
	_ "golang.org/x/crypto/ssh"
	_ "golang.org/x/crypto/ssh/agent"
	_ "golang.org/x/exp/constraints"
	_ "golang.org/x/mod/semver" //nolint:depguard // Usage precedes the x/mod/semver rule.
	_ "golang.org/x/net/html"
	_ "golang.org/x/net/http/httpproxy"
	_ "golang.org/x/net/http2"
	_ "golang.org/x/oauth2"
	_ "golang.org/x/oauth2/google"
	_ "golang.org/x/sync/errgroup"
	_ "golang.org/x/sync/semaphore"
	_ "golang.org/x/sync/singleflight"
	_ "golang.org/x/time/rate"
	_ "google.golang.org/api/admin/directory/v1"
	_ "google.golang.org/api/cloudidentity/v1"
	_ "google.golang.org/api/option"
	_ "google.golang.org/genproto/googleapis/rpc/status"
	_ "google.golang.org/grpc"
	_ "google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/connectivity"
	_ "google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/health"
	_ "google.golang.org/grpc/health/grpc_health_v1"
	_ "google.golang.org/grpc/metadata"
	_ "google.golang.org/grpc/stats"
	_ "google.golang.org/grpc/status"
	_ "google.golang.org/grpc/test/bufconn"
	_ "google.golang.org/protobuf/encoding/protojson"
	_ "google.golang.org/protobuf/proto"
	_ "google.golang.org/protobuf/reflect/protoreflect"
	_ "google.golang.org/protobuf/runtime/protoimpl"
	_ "google.golang.org/protobuf/testing/protocmp"
	_ "google.golang.org/protobuf/types/known/emptypb"
	_ "google.golang.org/protobuf/types/known/fieldmaskpb"
	_ "google.golang.org/protobuf/types/known/structpb"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	_ "google.golang.org/protobuf/types/known/wrapperspb"
	_ "gopkg.in/check.v1"
	_ "k8s.io/apimachinery/pkg/util/yaml"

	_ "github.com/gravitational/teleport/api"
	_ "github.com/gravitational/teleport/api/accessrequest"
	_ "github.com/gravitational/teleport/api/breaker"
	_ "github.com/gravitational/teleport/api/client"
	_ "github.com/gravitational/teleport/api/client/proto"
	_ "github.com/gravitational/teleport/api/client/webclient"
	_ "github.com/gravitational/teleport/api/constants"
	_ "github.com/gravitational/teleport/api/defaults"
	_ "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalauditstorage/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	_ "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	_ "github.com/gravitational/teleport/api/metadata"
	_ "github.com/gravitational/teleport/api/mfa"
	_ "github.com/gravitational/teleport/api/trail"
	_ "github.com/gravitational/teleport/api/types"
	_ "github.com/gravitational/teleport/api/types/accessgraph"
	_ "github.com/gravitational/teleport/api/types/accesslist"
	_ "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	_ "github.com/gravitational/teleport/api/types/clusterconfig"
	_ "github.com/gravitational/teleport/api/types/common"
	_ "github.com/gravitational/teleport/api/types/discoveryconfig"
	_ "github.com/gravitational/teleport/api/types/events"
	_ "github.com/gravitational/teleport/api/types/externalauditstorage"
	_ "github.com/gravitational/teleport/api/types/externalauditstorage/convert/v1"
	_ "github.com/gravitational/teleport/api/types/header"
	_ "github.com/gravitational/teleport/api/types/header/convert/legacy"
	_ "github.com/gravitational/teleport/api/types/header/convert/v1"
	_ "github.com/gravitational/teleport/api/types/samlsp"
	_ "github.com/gravitational/teleport/api/types/secreports"
	_ "github.com/gravitational/teleport/api/types/secreports/convert/v1"
	_ "github.com/gravitational/teleport/api/types/trait"
	_ "github.com/gravitational/teleport/api/types/trait/convert/v1"
	_ "github.com/gravitational/teleport/api/types/userloginstate"
	_ "github.com/gravitational/teleport/api/types/wrappers"
	_ "github.com/gravitational/teleport/api/utils"
	_ "github.com/gravitational/teleport/api/utils/aws"
	_ "github.com/gravitational/teleport/api/utils/entraid"
	_ "github.com/gravitational/teleport/api/utils/grpc/interceptors"
	_ "github.com/gravitational/teleport/api/utils/keys"
	_ "github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	_ "github.com/gravitational/teleport/api/utils/retryutils"
	_ "github.com/gravitational/teleport/api/utils/sshutils"
	_ "github.com/gravitational/teleport/api/utils/tlsutils"
)
