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

package users

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
)

// cosmosDBFetcher is a fetcher for discovering Azure CosmosDB database accounts
// users.
type cosmosDBFetcher struct {
	cfg Config
}

// newCosmosDBFetcher creates a new instance of CosmosDB fetcher.
func newCosmosDBFetcher(cfg Config) (Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &cosmosDBFetcher{cfg}, nil
}

// GetType returns the database type of the fetcher.
func (f *cosmosDBFetcher) GetType() string {
	return types.DatabaseTypeCosmosDBMongo
}

// FetchDatabaseUsers fetches users for provided database.
func (f *cosmosDBFetcher) FetchDatabaseUsers(ctx context.Context, database types.Database) ([]User, error) {
	resourceID, err := arm.ParseResourceID(database.GetAzure().ResourceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return one user per account key.
	var users []User
	for _, k := range armcosmos.PossibleKeyKindValues() {
		users = append(users, &cosmosUser{
			log:               f.cfg.Log,
			clients:           f.cfg.Clients,
			kind:              k,
			resourceGroupName: resourceID.ResourceGroupName,
			subscriptionID:    resourceID.SubscriptionID,
			accountName:       resourceID.Name,
		})
	}

	return users, nil
}

// cosmosUser represents a CosmosDB account key.
type cosmosUser struct {
	log               logrus.FieldLogger
	clients           cloud.Clients
	kind              armcosmos.KeyKind
	resourceGroupName string
	subscriptionID    string
	accountName       string
}

// GetDatabaseUsername returns in-database username for the user.
func (u *cosmosUser) GetDatabaseUsername() string {
	return string(u.kind)
}

// GetID returns a globally unique ID for the user.
func (u *cosmosUser) GetID() string {
	return fmt.Sprintf("%s-%s", u.accountName, string(u.kind))
}

// GetPassword returns the password used for database login.
func (u *cosmosUser) GetPassword(ctx context.Context) (string, error) {
	cl, err := u.clients.GetAzureCosmosDatabaseAccountsClient(u.subscriptionID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	pass, err := cl.GetKey(ctx, u.resourceGroupName, u.accountName, u.kind)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return pass, nil
}

// RotatePassword rotates user's password.
// TODO(gabrielcorado): Rotating passwords causes database instability, we need
// a mechanism to progressively rotate the passwords and fallback to secondary
// when rotating like described at: https://learn.microsoft.com/en-us/azure/cosmos-db/database-security?tabs=mongo-api#key-rotation
func (u *cosmosUser) RotatePassword(ctx context.Context) error {
	return nil
}

// Setup preforms any setup necessary like creating password secret.
// In case of CosmosDB this is unnecessary.
func (*cosmosUser) Setup(ctx context.Context) error {
	return nil
}

// Teardown performs any teardown necessary like deleting password secret.
// In case of CosmosDB this is unnecessary.
func (*cosmosUser) Teardown(ctx context.Context) error {
	return nil
}
