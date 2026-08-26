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

package events

import (
	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/gravitational/teleport/api/types"
)

const (
	// UseFIPSQueryParam is the URL query parameter used for enabling
	// FIPS endpoints for AWS S3/Dynamo.
	UseFIPSQueryParam = "use_fips_endpoint"
)

var (
	fipsToAWS = map[types.ClusterAuditConfigSpecV2_FIPSEndpointState]aws.FIPSEndpointState{
		types.ClusterAuditConfigSpecV2_FIPS_UNSET:    aws.FIPSEndpointStateUnset,
		types.ClusterAuditConfigSpecV2_FIPS_ENABLED:  aws.FIPSEndpointStateEnabled,
		types.ClusterAuditConfigSpecV2_FIPS_DISABLED: aws.FIPSEndpointStateDisabled,
	}
)

// FIPSProtoStateToAWSState converts a FIPS proto state to an aws endpoints.FIPSEndpointState
func FIPSProtoStateToAWSState(state types.ClusterAuditConfigSpecV2_FIPSEndpointState) aws.FIPSEndpointState {
	return fipsToAWS[state]
}
