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
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// TestAzureDBServerFetchers tests common azureFetcher functionalities and the
// azureDBServerPlugin which is used for Azure MySQL and Azure PostgreSQL.
func TestAzureDBServerFetchers(t *testing.T) {
	t.Parallel()

	const (
		group1        = "group1"
		group2        = "group2"
		eastus        = "eastus"
		eastus2       = "eastus2"
		westus        = "westus"
		subscription1 = "sub1"
		subscription2 = "sub2"
	)

	azureSub1 := makeAzureSubscription(t, subscription1)
	azureSub2 := makeAzureSubscription(t, subscription2)

	azMySQLServer1, azMySQLDB1 := makeAzureMySQLServer(t, "server-1", subscription1, group1, eastus, map[string]string{"env": "prod"})
	azMySQLServer2, _ := makeAzureMySQLServer(t, "server-2", subscription1, group1, eastus, map[string]string{"env": "dev"})
	azMySQLServer3, _ := makeAzureMySQLServer(t, "server-3", subscription1, group1, eastus2, map[string]string{"env": "prod"})
	azMySQLServer4, azMySQLDB4 := makeAzureMySQLServer(t, "server-4", subscription2, group1, westus, map[string]string{"env": "prod"})
	azMySQLServer5, _ := makeAzureMySQLServer(t, "server-5", subscription1, group2, eastus, map[string]string{"env": "prod"})
	azMySQLServerUnknownVersion, azMySQLDBUnknownVersion := makeAzureMySQLServer(t, "server-6", subscription1, group1, eastus, nil, withAzureMySQLVersion("unknown"))
	azMySQLServerUnsupportedVersion, _ := makeAzureMySQLServer(t, "server-7", subscription1, group1, eastus, nil, withAzureMySQLVersion(string(armmysql.ServerVersionFive6)))
	azMySQLServerDisabledState, _ := makeAzureMySQLServer(t, "server-8", subscription1, group1, eastus, nil, withAzureMySQLState(string(armmysql.ServerStateDisabled)))
	azMySQLServerUnknownState, azMySQLDBUnknownState := makeAzureMySQLServer(t, "server-9", subscription1, group1, eastus, nil, withAzureMySQLState("unknown"))

	azPostgresServer1, azPostgresDB1 := makeAzurePostgresServer(t, "server-1", subscription1, group1, eastus, map[string]string{"env": "prod"})
	azPostgresServer2, _ := makeAzurePostgresServer(t, "server-2", subscription1, group1, eastus, map[string]string{"env": "dev"})
	azPostgresServer3, _ := makeAzurePostgresServer(t, "server-3", subscription1, group1, eastus2, map[string]string{"env": "prod"})
	azPostgresServer4, azPostgresDB4 := makeAzurePostgresServer(t, "server-4", subscription2, group1, westus, map[string]string{"env": "prod"})
	azPostgresServer5, _ := makeAzurePostgresServer(t, "server-5", subscription1, group2, eastus, map[string]string{"env": "prod"})
	azPostgresServerUnknownVersion, azPostgresDBUnknownVersion := makeAzurePostgresServer(t, "server-6", subscription1, group1, eastus, nil, withAzurePostgresVersion("unknown"))
	azPostgresServerDisabledState, _ := makeAzurePostgresServer(t, "server-8", subscription1, group1, eastus, nil, withAzurePostgresState(string(armpostgresql.ServerStateDisabled)))
	azPostgresServerUnknownState, azPostgresDBUnknownState := makeAzurePostgresServer(t, "server-9", subscription1, group1, eastus, nil, withAzurePostgresState("unknown"))

	tests := []struct {
		name          string
		inputClients  cloud.AzureClients
		inputMatchers []types.AzureMatcher
		wantDatabases types.Databases
	}{
		{
			name: "match labels",
			inputMatchers: []types.AzureMatcher{
				{
					Subscriptions:  []string{subscription1},
					ResourceGroups: []string{group1},
					Types:          []string{types.AzureMatcherMySQL, types.AzureMatcherPostgres},
					Regions:        []string{eastus},
					ResourceTags:   types.Labels{"env": []string{"prod"}},
				},
			},
			inputClients: &cloud.TestCloudClients{
				AzureMySQLPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer1, azMySQLServer2, azMySQLServer3, azMySQLServer5},
					}),
					subscription2: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer4},
					}),
				},
				AzurePostgresPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer1, azPostgresServer2, azPostgresServer3, azPostgresServer5},
					}),
					subscription2: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer4},
					}),
				},
				AzureMySQLFlex: azure.NewMySQLFlexServersClientByAPI(&azure.ARMMySQLFlexServerMock{
					NoAuth: true,
				}),
				AzurePostgresFlex: azure.NewPostgresFlexServersClientByAPI(&azure.ARMPostgresFlexServerMock{
					NoAuth: true,
				}),
			},
			// server2 tags don't match, server3 is in eastus2, server4 is in subscription2, server5 is in group2
			wantDatabases: types.Databases{azMySQLDB1, azPostgresDB1},
		},
		{
			name: "match labels with all subscriptions, resource groups, and regions",
			inputMatchers: []types.AzureMatcher{
				{
					Subscriptions:  []string{"*"},
					ResourceGroups: []string{"*"},
					Types:          []string{types.AzureMatcherMySQL, types.AzureMatcherPostgres},
					Regions:        []string{"*"},
					ResourceTags:   types.Labels{"env": []string{"prod"}},
				},
			},
			inputClients: &cloud.TestCloudClients{
				AzureMySQLPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer1},
					}),
					subscription2: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer4},
					}),
				},
				AzurePostgresPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer1},
					}),
					subscription2: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer4},
					}),
				},
				AzureSubscriptionClient: azure.NewSubscriptionClient(&azure.ARMSubscriptionsMock{
					Subscriptions: []*armsubscription.Subscription{azureSub1, azureSub2},
				}),
				AzureMySQLFlex: azure.NewMySQLFlexServersClientByAPI(&azure.ARMMySQLFlexServerMock{
					NoAuth: true,
				}),
				AzurePostgresFlex: azure.NewPostgresFlexServersClientByAPI(&azure.ARMPostgresFlexServerMock{
					NoAuth: true,
				}),
			},
			wantDatabases: types.Databases{azMySQLDB1, azMySQLDB4, azPostgresDB1, azPostgresDB4},
		},
		{
			name: "skip unsupported and unknown database versions",
			inputMatchers: []types.AzureMatcher{
				{
					Subscriptions:  []string{subscription1},
					ResourceGroups: []string{"*"},
					Types:          []string{types.AzureMatcherMySQL, types.AzureMatcherPostgres},
					Regions:        []string{eastus},
					ResourceTags:   types.Labels{"*": []string{"*"}},
				},
			},
			inputClients: &cloud.TestCloudClients{
				AzureMySQL: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
					DBServers: []*armmysql.Server{
						azMySQLServer1,
						azMySQLServerUnknownVersion,
						azMySQLServerUnsupportedVersion,
					},
				}),
				AzurePostgres: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
					DBServers: []*armpostgresql.Server{
						azPostgresServer1,
						azPostgresServerUnknownVersion,
					},
				}),
				AzureMySQLFlex: azure.NewMySQLFlexServersClientByAPI(&azure.ARMMySQLFlexServerMock{
					NoAuth: true,
				}),
				AzurePostgresFlex: azure.NewPostgresFlexServersClientByAPI(&azure.ARMPostgresFlexServerMock{
					NoAuth: true,
				}),
			},
			wantDatabases: types.Databases{azMySQLDB1, azMySQLDBUnknownVersion, azPostgresDB1, azPostgresDBUnknownVersion},
		},
		{
			name: "skip unavailable",
			inputMatchers: []types.AzureMatcher{
				{
					Subscriptions:  []string{subscription1},
					ResourceGroups: []string{"*"},
					Types:          []string{types.AzureMatcherMySQL, types.AzureMatcherPostgres},
					Regions:        []string{eastus},
					ResourceTags:   types.Labels{"*": []string{"*"}},
				},
			},
			inputClients: &cloud.TestCloudClients{
				AzureMySQL: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
					DBServers: []*armmysql.Server{
						azMySQLServer1,
						azMySQLServerDisabledState,
						azMySQLServerUnknownState,
					},
				}),
				AzurePostgres: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
					DBServers: []*armpostgresql.Server{
						azPostgresServer1,
						azPostgresServerDisabledState,
						azPostgresServerUnknownState,
					},
				}),
				AzureMySQLFlex: azure.NewMySQLFlexServersClientByAPI(&azure.ARMMySQLFlexServerMock{
					NoAuth: true,
				}),
				AzurePostgresFlex: azure.NewPostgresFlexServersClientByAPI(&azure.ARMPostgresFlexServerMock{
					NoAuth: true,
				}),
			},
			wantDatabases: types.Databases{azMySQLDB1, azMySQLDBUnknownState, azPostgresDB1, azPostgresDBUnknownState},
		},
		{
			name: "skip access denied errors",
			inputMatchers: []types.AzureMatcher{
				{
					Subscriptions:  []string{subscription1, subscription2},
					ResourceGroups: []string{"*"},
					Types:          []string{types.AzureMatcherMySQL, types.AzureMatcherPostgres},
					Regions:        []string{eastus, westus},
					ResourceTags:   types.Labels{"*": []string{"*"}},
				},
			},
			inputClients: &cloud.TestCloudClients{
				AzureMySQLPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer1},
						NoAuth:    true,
					}),
					subscription2: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer4},
					}),
				},
				AzurePostgresPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer1},
						NoAuth:    true,
					}),
					subscription2: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer4},
					}),
				},
				AzureMySQLFlex: azure.NewMySQLFlexServersClientByAPI(&azure.ARMMySQLFlexServerMock{
					NoAuth: true,
				}),
				AzurePostgresFlex: azure.NewPostgresFlexServersClientByAPI(&azure.ARMPostgresFlexServerMock{
					NoAuth: true,
				}),
			},
			wantDatabases: types.Databases{azMySQLDB4, azPostgresDB4},
		},
		{
			name: "skip group not found errors",
			inputMatchers: []types.AzureMatcher{
				{
					Subscriptions:  []string{subscription1},
					ResourceGroups: []string{"foobar", group1, "baz"},
					Types:          []string{types.AzureMatcherMySQL, types.AzureMatcherPostgres},
					Regions:        []string{eastus, westus},
					ResourceTags:   types.Labels{"*": []string{"*"}},
				},
			},
			inputClients: &cloud.TestCloudClients{
				AzureMySQL: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
					DBServers: []*armmysql.Server{
						azMySQLServer1,
					},
				}),
				AzurePostgres: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
					DBServers: []*armpostgresql.Server{
						azPostgresServer1,
					},
				}),
				AzureMySQLFlex: azure.NewMySQLFlexServersClientByAPI(&azure.ARMMySQLFlexServerMock{
					NoAuth: true,
				}),
				AzurePostgresFlex: azure.NewPostgresFlexServersClientByAPI(&azure.ARMPostgresFlexServerMock{
					NoAuth: true,
				}),
			},
			wantDatabases: types.Databases{azMySQLDB1, azPostgresDB1},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fetchers := mustMakeAzureFetchers(t, test.inputClients, test.inputMatchers)
			require.ElementsMatch(t, test.wantDatabases, mustGetDatabases(t, fetchers))
		})
	}
}

func makeAzureSubscription(t *testing.T, subID string) *armsubscription.Subscription {
	return &armsubscription.Subscription{
		SubscriptionID: &subID,
		State:          to.Ptr(armsubscription.SubscriptionStateEnabled),
	}
}

func makeAzureMySQLServer(t *testing.T, name, subscription, group, region string, labels map[string]string, opts ...func(*armmysql.Server)) (*armmysql.Server, types.Database) {
	resourceType := "Microsoft.DBforMySQL/servers"
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
		subscription,
		group,
		resourceType,
		name,
	)

	fqdn := name + ".mysql" + azureutils.DatabaseEndpointSuffix
	state := armmysql.ServerStateReady
	version := armmysql.ServerVersionFive7
	server := &armmysql.Server{
		Location: &region,
		Properties: &armmysql.ServerProperties{
			FullyQualifiedDomainName: &fqdn,
			UserVisibleState:         &state,
			Version:                  &version,
		},
		Tags: labelsToAzureTags(labels),
		ID:   &id,
		Name: &name,
		Type: &resourceType,
	}
	for _, opt := range opts {
		opt(server)
	}

	azureDBServer := azure.ServerFromMySQLServer(server)

	database, err := common.NewDatabaseFromAzureServer(azureDBServer)
	require.NoError(t, err)
	common.ApplyAzureDatabaseNameSuffix(database, types.AzureMatcherMySQL)
	return server, database
}

func makeAzurePostgresServer(t *testing.T, name, subscription, group, region string, labels map[string]string, opts ...func(*armpostgresql.Server)) (*armpostgresql.Server, types.Database) {
	resourceType := "Microsoft.DBforPostgreSQL/servers"
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
		subscription,
		group,
		resourceType,
		name,
	)

	fqdn := name + ".postgres" + azureutils.DatabaseEndpointSuffix
	state := armpostgresql.ServerStateReady
	version := armpostgresql.ServerVersionEleven
	server := &armpostgresql.Server{
		Location: &region,
		Properties: &armpostgresql.ServerProperties{
			FullyQualifiedDomainName: &fqdn,
			UserVisibleState:         &state,
			Version:                  &version,
		},
		Tags: labelsToAzureTags(labels),
		ID:   &id,
		Name: &name,
		Type: &resourceType,
	}
	for _, opt := range opts {
		opt(server)
	}

	azureDBServer := azure.ServerFromPostgresServer(server)

	database, err := common.NewDatabaseFromAzureServer(azureDBServer)
	require.NoError(t, err)
	common.ApplyAzureDatabaseNameSuffix(database, types.AzureMatcherPostgres)
	return server, database
}

// withAzureMySQLState returns an option function to makeARMMySQLServer to overwrite state.
func withAzureMySQLState(state string) func(*armmysql.Server) {
	return func(server *armmysql.Server) {
		state := armmysql.ServerState(state) // ServerState is a type alias for string
		server.Properties.UserVisibleState = &state
	}
}

// withAzureMySQLVersion returns an option function to makeARMMySQLServer to overwrite version.
func withAzureMySQLVersion(version string) func(*armmysql.Server) {
	return func(server *armmysql.Server) {
		version := armmysql.ServerVersion(version) // ServerVersion is a type alias for string
		server.Properties.Version = &version
	}
}

// withAzurePostgresState returns an option function to makeARMPostgresServer to overwrite state.
func withAzurePostgresState(state string) func(*armpostgresql.Server) {
	return func(server *armpostgresql.Server) {
		state := armpostgresql.ServerState(state) // ServerState is a type alias for string
		server.Properties.UserVisibleState = &state
	}
}

// withAzurePostgresVersion returns an option function to makeARMPostgresServer to overwrite version.
func withAzurePostgresVersion(version string) func(*armpostgresql.Server) {
	return func(server *armpostgresql.Server) {
		version := armpostgresql.ServerVersion(version) // ServerVersion is a type alias for string
		server.Properties.Version = &version
	}
}

func labelsToAzureTags(labels map[string]string) map[string]*string {
	tags := make(map[string]*string, len(labels))
	for k, v := range labels {
		v := v
		tags[k] = &v
	}
	return tags
}
