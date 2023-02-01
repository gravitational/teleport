// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package db

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/lib/utils"
)

func TestSQLManagedServerFetcher(t *testing.T) {
	logger := utils.NewLoggerForTests()
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
				require.NotNil(t, fetcher.NewDatabaseFromServer(makeManagedSQLInstance(state), logger), "expected to have a database, but got nil")
			})
		}

		// The remaining possible states should not return a database.
		for _, state := range armsql.PossibleManagedInstancePropertiesProvisioningStateValues() {
			// Skip if the state was already tested.
			if slices.Contains(availableStates, state) {
				continue
			}

			t.Run(string(state), func(t *testing.T) {
				require.Nil(t, fetcher.NewDatabaseFromServer(makeManagedSQLInstance(state), logger), "expected to have nil, but got a database")
			})
		}

		t.Run("RandomState", func(t *testing.T) {
			require.NotNil(t,
				fetcher.NewDatabaseFromServer(
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
