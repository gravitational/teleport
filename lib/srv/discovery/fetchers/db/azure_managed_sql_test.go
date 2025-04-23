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

package db

import (
	"context"
	"slices"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestSQLManagedServerFetcher(t *testing.T) {
	logger := utils.NewSlogLoggerForTests()
	fetcher := &azureManagedSQLServerFetcher{}

	t.Run("NewDatabaseFromServer", func(t *testing.T) {
		// List of provisioning states that are used to identify a database is
		// available and can be discovered.
		availableStates := []armsql.ManagedInstancePropertiesProvisioningState{
			armsql.ManagedInstancePropertiesProvisioningStateCreated,
			armsql.ManagedInstancePropertiesProvisioningStateRunning,
			armsql.ManagedInstancePropertiesProvisioningStateSucceeded,
			armsql.ManagedInstancePropertiesProvisioningStateUpdating,
		}

		// For available states, it should return a parsed database.
		for _, state := range availableStates {
			t.Run(string(state), func(t *testing.T) {
				require.NotNil(t, fetcher.NewDatabaseFromServer(context.Background(), makeManagedSQLInstance(state), logger), "expected to have a database, but got nil")
			})
		}

		// The remaining possible states should not return a database.
		for _, state := range armsql.PossibleManagedInstancePropertiesProvisioningStateValues() {
			// Skip if the state was already tested.
			if slices.Contains(availableStates, state) {
				continue
			}

			t.Run(string(state), func(t *testing.T) {
				require.Nil(t, fetcher.NewDatabaseFromServer(context.Background(), makeManagedSQLInstance(state), logger), "expected to have nil, but got a database")
			})
		}

		t.Run("RandomState", func(t *testing.T) {
			require.NotNil(t,
				fetcher.NewDatabaseFromServer(
					context.Background(),
					makeManagedSQLInstance("RandomState"),
					logger,
				),
				"expected to have a database, but got nil",
			)
		})
	})
}

// makeManagedSQLInstances returns a ManagedInstance struct with the provided
// provisioning state.
func makeManagedSQLInstance(state armsql.ManagedInstancePropertiesProvisioningState) *armsql.ManagedInstance {
	return &armsql.ManagedInstance{
		ID:       to.Ptr("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-groupd/providers/Microsoft.Sql/servers/sqlserver"),
		Name:     to.Ptr("sqlserver"),
		Location: to.Ptr("westus"),
		Properties: &armsql.ManagedInstanceProperties{
			FullyQualifiedDomainName: to.Ptr("sqlserver.database.windows.net"),
			ProvisioningState:        to.Ptr(state),
		},
	}
}
