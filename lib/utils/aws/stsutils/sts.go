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

package stsutils

import (
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// NewFromConfig wraps [sts.NewFromConfig] and applies FIPS settings
// according to environment variables.
//
// If TELEPORT_UNSTABLE_DISABLE_STS_FIPS is set to "yes" or a true-value
// (according to [strconv.ParseBool]), then FIPS is disabled for the returned
// [sts.Client], regardless of any other settings.
func NewFromConfig(cfg aws.Config, optFns ...func(*sts.Options)) *sts.Client {
	if disableFIPSEndpoint() {
		// append so it overrides any preceding settings.
		optFns = append(optFns, func(opts *sts.Options) {
			opts.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateDisabled
		})
	}
	return sts.NewFromConfig(cfg, optFns...)
}

func disableFIPSEndpoint() bool {
	const envVar = "TELEPORT_UNSTABLE_DISABLE_STS_FIPS"

	// Disable FIPS endpoint?
	if val := os.Getenv(envVar); val != "" {
		b, _ := strconv.ParseBool(val)
		return b || val == "yes"
	}

	return false
}
