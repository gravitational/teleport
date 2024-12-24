/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package mocks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

type AWSConfigProvider struct {
	STSClient *STSClient
}

func (f *AWSConfigProvider) GetConfig(ctx context.Context, region string, optFns ...awsconfig.OptionsFn) (aws.Config, error) {
	stsClt := f.STSClient
	if stsClt == nil {
		stsClt = &STSClient{}
	}
	optFns = append(optFns, awsconfig.WithAssumeRoleClientProviderFunc(
		newAssumeRoleClientProviderFunc(stsClt),
	))
	return awsconfig.GetConfig(ctx, region, optFns...)
}
