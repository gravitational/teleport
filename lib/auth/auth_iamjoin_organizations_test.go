/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationstypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/join/iamjoin"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestAWSOrganizationsClientUsingAmbientCredentials(t *testing.T) {
	t.Run("when running in cloud, the client getter returns an error (ambient credentials can't be used in cloud)", func(t *testing.T) {
		modulestest.SetTestModules(t, modulestest.Modules{
			TestFeatures: modules.Features{
				Cloud: true,
			},
		})

		clientGetter, err := awsOrganizationsClientUsingAmbientCredentials(t.Context(), clockwork.NewFakeClock(), nil)
		require.NoError(t, err)

		_, err = clientGetter.Get(t.Context())
		require.Error(t, err)
	})

	t.Run("when running in non-cloud, the getter returns a valid client", func(t *testing.T) {
		modulestest.SetTestModules(t, modulestest.Modules{
			TestFeatures: modules.Features{
				Cloud: false,
			},
		})

		fakeClock := clockwork.NewFakeClock()
		mockOrganizationsAPI := &mockOrganizationsAPI{}

		clientGetter, err := awsOrganizationsClientUsingAmbientCredentials(t.Context(), fakeClock, func(c aws.Config) iamjoin.OrganizationsAPI {
			return mockOrganizationsAPI
		})
		require.NoError(t, err)

		organizationsAPI, err := clientGetter.Get(t.Context())
		require.NoError(t, err)

		describeAccountAPIOutput, err := organizationsAPI.DescribeAccount(t.Context(), &organizations.DescribeAccountInput{
			AccountId: aws.String("123456789012"),
		})
		require.NoError(t, err)
		require.Equal(t, aws.String("123456789012"), describeAccountAPIOutput.Account.Id)
		require.Equal(t, 1, mockOrganizationsAPI.numberOfRemoteAPICalls, "expected one remote API call")

		// Call again to verify caching works.
		describeAccountAPIOutput, err = organizationsAPI.DescribeAccount(t.Context(), &organizations.DescribeAccountInput{
			AccountId: aws.String("123456789012"),
		})
		require.NoError(t, err)
		require.Equal(t, aws.String("123456789012"), describeAccountAPIOutput.Account.Id)
		require.Equal(t, 1, mockOrganizationsAPI.numberOfRemoteAPICalls, "expected no additional remote API calls due to caching")

		// However, after the cache expiration time, a new call should be made.
		fakeClock.Advance(5 * time.Minute)
		describeAccountAPIOutput, err = organizationsAPI.DescribeAccount(t.Context(), &organizations.DescribeAccountInput{
			AccountId: aws.String("123456789012"),
		})
		require.NoError(t, err)
		require.Equal(t, aws.String("123456789012"), describeAccountAPIOutput.Account.Id)
		require.Equal(t, 2, mockOrganizationsAPI.numberOfRemoteAPICalls, "expected a new remote API call after cache expiration")
	})
}

type mockOrganizationsAPI struct {
	numberOfRemoteAPICalls int
}

func (m *mockOrganizationsAPI) DescribeAccount(ctx context.Context, params *organizations.DescribeAccountInput, optFns ...func(*organizations.Options)) (*organizations.DescribeAccountOutput, error) {
	m.numberOfRemoteAPICalls++
	return &organizations.DescribeAccountOutput{
		Account: &organizationstypes.Account{Id: aws.String("123456789012")},
	}, nil
}
