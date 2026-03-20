// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package config

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/gravitational/trace"

	apiutilsaws "github.com/gravitational/teleport/api/utils/aws"
)

// LoadDefaultConfig loads an AWS config using the SDK defaults, validating any
// explicitly requested region before use.
func LoadDefaultConfig(ctx context.Context, optFns ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
	var loadOptions awsconfig.LoadOptions

	for _, optFn := range optFns {
		if err := optFn(&loadOptions); err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
	}

	if loadOptions.Region != "" {
		// Use the weak check here rather than the strict one because the region
		// may come from customer-supplied configuration targeting an AWS compatible
		// service that uses different region naming schema.
		// To avoid breaking them, we introduced the weak check that allows any
		// string containing hiphens, numbers and lowercase letters.
		if err := apiutilsaws.IsValidRegionWithWeakCheck(loadOptions.Region); err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	return cfg, trace.Wrap(err)
}
