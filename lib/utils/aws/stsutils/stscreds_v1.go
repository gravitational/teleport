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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"

	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// NewCredentialsV1 wraps [stscreds.NewCredentials] and applies FIPS settings
// according to environment variables.
//
// See [awsutils.IsFIPSDisabledByEnv].
func NewCredentialsV1(
	c client.ConfigProvider,
	roleARN string,
	options ...func(*stscreds.AssumeRoleProvider),
) *credentials.Credentials {
	if awsutils.IsFIPSDisabledByEnv() {
		c = fipsDisabledProvider{provider: c}
	}
	return stscreds.NewCredentials(c, roleARN, options...)
}

type fipsDisabledProvider struct {
	provider client.ConfigProvider
}

func (p fipsDisabledProvider) ClientConfig(serviceName string, cfgs ...*aws.Config) client.Config {
	cfgs = append(cfgs, aws.NewConfig().WithUseFIPSEndpoint(false))
	cfg := p.provider.ClientConfig(serviceName, cfgs...)
	return cfg
}
