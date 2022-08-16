package azure

import (
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/stretchr/testify/require"
)

func TestServerConversion(t *testing.T) {
	tests := []struct {
		name          string
		provider      string
		version       string
		state         string
		wantAvailable bool
		wantSupported bool
	}{
		{
			name:          "mysql available and supported",
			provider:      MySQLNamespace,
			version:       "5.7",
			state:         "Ready",
			wantAvailable: true,
			wantSupported: true,
		},
		{
			name:          "mysql unavailable and unsupported",
			provider:      MySQLNamespace,
			version:       "5.6",
			state:         "",
			wantAvailable: false,
			wantSupported: false,
		},
		{
			name:          "postgres available and supported",
			provider:      PostgreSQLNamespace,
			version:       "11",
			state:         "Ready",
			wantAvailable: true,
			wantSupported: true,
		},
		{
			name:          "postgres unavailable and unsupported",
			provider:      PostgreSQLNamespace,
			version:       "unknown",
			state:         "Disabled",
			wantAvailable: false,
			wantSupported: false,
		},
	}

	region := "eastus"
	dbName := "dbname"
	tags := map[string]string{"foo": "bar", "baz": "qux"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				server     *DBServer
				err        error
				fqdn       string
				serverType string
			)
			switch tt.provider {
			case MySQLNamespace:
				serverType = fmt.Sprintf("%v/servers", tt.provider)
				id := fmt.Sprintf("/subscriptions/subid/resourceGroups/group/providers/%v/dbname", serverType)
				fqdn = fmt.Sprintf("%v.mysql.database.azure.com", dbName)
				server, err = ServerFromMySQLServer(
					&armmysql.Server{
						Location: &region,
						Properties: &armmysql.ServerProperties{
							FullyQualifiedDomainName: &fqdn,
							UserVisibleState:         (*armmysql.ServerState)(&tt.state),
							Version:                  (*armmysql.ServerVersion)(&tt.version),
						},
						Tags: makeAzureTags(tags),
						ID:   &id,
						Name: &dbName,
						Type: &serverType,
					})
			case PostgreSQLNamespace:
				serverType = fmt.Sprintf("%v/servers", tt.provider)
				id := fmt.Sprintf("/subscriptions/subid/resourceGroups/group/providers/%v/dbname", serverType)
				fqdn = fmt.Sprintf("%v.postgres.database.azure.com", dbName)
				server, err = ServerFromPostgresServer(
					&armpostgresql.Server{
						Location: &region,
						Properties: &armpostgresql.ServerProperties{
							FullyQualifiedDomainName: &fqdn,
							UserVisibleState:         (*armpostgresql.ServerState)(&tt.state),
							Version:                  (*armpostgresql.ServerVersion)(&tt.version),
						},
						Tags: makeAzureTags(tags),
						ID:   &id,
						Name: &dbName,
						Type: &serverType,
					})
			default:
				require.FailNow(t, "unknown db namespace specified by test")
			}
			require.NoError(t, err)

			rid, err := arm.ParseResourceID(server.ID)
			require.NoError(t, err)

			require.Equal(t, dbName, server.Name)
			require.Equal(t, dbName, rid.Name)
			require.Equal(t, tt.provider+"/servers", server.Type)
			require.Equal(t, region, server.Location)
			require.Equal(t, tags, server.Tags)
			require.Equal(t, fqdn, server.Properties.FullyQualifiedDomainName)
			require.Equal(t, tt.state, server.Properties.UserVisibleState)
			require.Equal(t, tt.version, server.Properties.Version)
			require.Equal(t, "group", rid.ResourceGroupName)
			require.Equal(t, "subid", rid.SubscriptionID)
			require.Equal(t, tt.provider, rid.ResourceType.Namespace)
			require.Equal(t, "servers", rid.ResourceType.Type)
			require.Equal(t, tt.wantAvailable, server.IsAvailable())
			require.Equal(t, tt.wantSupported, server.IsVersionSupported())
		})
	}
}

// makeAzureTags is a helper function for making azure db server tags
func makeAzureTags(m map[string]string) map[string]*string {
	result := make(map[string]*string, len(m))
	for k, v := range m {
		v := v
		result[k] = &v
	}
	return result
}
