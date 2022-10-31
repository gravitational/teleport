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

package events

import (
	"github.com/aws/aws-sdk-go/aws/endpoints"

	"github.com/gravitational/teleport/api/types"
)

const (
	// UseFIPSQueryParam is the URL query parameter used for enabling
	// FIPS endpoints for AWS S3/Dynamo.
	UseFIPSQueryParam = "use_fips_endpoint"
)

var (
	fipsToAWS = map[types.ClusterAuditConfigSpecV2_FIPSEndpointState]endpoints.FIPSEndpointState{
		types.ClusterAuditConfigSpecV2_FIPS_UNSET:    endpoints.FIPSEndpointStateUnset,
		types.ClusterAuditConfigSpecV2_FIPS_ENABLED:  endpoints.FIPSEndpointStateEnabled,
		types.ClusterAuditConfigSpecV2_FIPS_DISABLED: endpoints.FIPSEndpointStateDisabled,
	}
)

// FIPSProtoStateToAWSState converts a FIPS proto state to an aws endpoints.FIPSEndpointState
func FIPSProtoStateToAWSState(state types.ClusterAuditConfigSpecV2_FIPSEndpointState) endpoints.FIPSEndpointState {
	return fipsToAWS[state]
}
