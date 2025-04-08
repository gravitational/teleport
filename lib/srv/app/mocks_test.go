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

package app

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/sts"
)

// mockCredentialsProvider mocks AWS credentials.Provider interface.
type mockCredentialsProvider struct {
	retrieveValue credentials.Value
	retrieveError error
}

func (m mockCredentialsProvider) Retrieve() (credentials.Value, error) {
	return m.retrieveValue, m.retrieveError
}
func (m mockCredentialsProvider) IsExpired() bool {
	return false
}

// mockAssumeRoler mocks AWS stscreds.AssumeRoler interface.
type mockAssumeRoler struct {
	output *sts.AssumeRoleOutput
}

func (m mockAssumeRoler) AssumeRole(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	return m.output, nil
}
