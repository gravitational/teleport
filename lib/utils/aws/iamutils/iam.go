// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package iamutils

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// NewFromConfig wraps [iam.NewFromConfig] and applies FIPS settings
// according to environment variables.
//
// See [awsutils.IsFIPSDisabledByEnv].
func NewFromConfig(cfg aws.Config, optFns ...func(*iam.Options)) *iam.Client {
	if awsutils.IsFIPSDisabledByEnv() {
		// append so it overrides any preceding settings.
		optFns = append(optFns, func(opts *iam.Options) {
			opts.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateDisabled
		})
	}
	return iam.NewFromConfig(cfg, optFns...)
}
