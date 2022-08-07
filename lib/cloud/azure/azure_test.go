package azure

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestServerConversion(t *testing.T) {
	tests := []struct {
		name    string
		dbType  string
		version string
		state   string
	}{
		{
			name:    "mysql conversion",
			dbType:  "mysql",
			version: "5.7",
			state:   "Ready",
		},
		{
			name:    "postgres conversion",
			dbType:  "postgres",
			version: "11",
			state:   "Ready",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				server     Server
				err        error
				provider   string
				port       string
				fqdn       string
				serverType string
			)
			region := "eastus"
			dbName := "dbname"
			typeFmt := "%v/servers"
			idFmt := "/subscriptions/subid/resourceGroups/group/providers/%v/dbname"
			tags := map[string]string{"foo": "bar", "baz": "qux"}
			switch tt.dbType {
			case defaults.ProtocolMySQL:
				provider = "Microsoft.DBforMySQL"
				port = "3306"
				serverType = fmt.Sprintf(typeFmt, provider)
				id := fmt.Sprintf(idFmt, serverType)
				fqdn = fmt.Sprintf("dbname.%v.database.azure.com", tt.dbType)
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
			case defaults.ProtocolPostgres:
				provider = "Microsoft.DBforPostgreSQL"
				port = "5432"
				serverType = fmt.Sprintf(typeFmt, provider)
				id := fmt.Sprintf(idFmt, serverType)
				fqdn = fmt.Sprintf("dbname.%v.database.azure.com", tt.dbType)
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
				require.FailNow(t, "unknown db protocol specified by test")
			}
			require.NoError(t, err)

			require.Equal(t, tt.dbType, server.Protocol())
			require.Equal(t, tt.state, server.State())
			require.Equal(t, tt.version, server.Version())
			require.Equal(t, provider, server.ID().ProviderNamespace)
			require.Equal(t, "group", server.ID().ResourceGroup)
			require.Equal(t, "subid", server.ID().SubscriptionID)
			require.Equal(t, "servers", server.ID().ResourceType)
			require.Equal(t, dbName, server.ID().ResourceName)
			require.Equal(t, dbName, server.Name())
			require.Equal(t, fqdn+":"+port, server.Endpoint())
			require.Equal(t, region, server.Region())
			require.Equal(t, tags, server.Tags())
		})
	}
}

// makeAzureTags is a test helper util function
func makeAzureTags(m map[string]string) map[string]*string {
	result := make(map[string]*string, len(m))
	for k, v := range m {
		v := v
		result[k] = &v
	}
	return result
}
