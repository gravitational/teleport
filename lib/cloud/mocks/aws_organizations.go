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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/organizations/organizationsiface"
)

type MockAwsOrganizationsService struct {
	organizationsiface.OrganizationsAPI
	Accounts []*organizations.Account
}

func (m *MockAwsOrganizationsService) ListAccountsPagesWithContext(
	_ aws.Context,
	_ *organizations.ListAccountsInput,
	output func(*organizations.ListAccountsOutput, bool) bool,
	_ ...request.Option) error {
	output(&organizations.ListAccountsOutput{Accounts: m.Accounts}, true)
	return nil
}
