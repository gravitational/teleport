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
		name     string
		protocol string
		version  string
		state    string
		wantErr  error
	}{
		{
			name: "one",
			protocol: "mysql",
			version: "5.7",
			state: "Ready",
			wantErr: nil,
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
			switch tt.protocol {
			case defaults.ProtocolMySQL:
				provider = "Microsoft.DBforMySQL"
				port = "3306"
				serverType = fmt.Sprintf(typeFmt, provider)
				id := fmt.Sprintf(idFmt, serverType)
				fqdn = fmt.Sprintf("dbname.%v.database.azure.com", tt.protocol)
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
				fqdn = fmt.Sprintf("dbname.%v.database.azure.com", tt.protocol)
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
			require.ErrorIs(t, err, tt.wantErr)

			require.Equal(t, tt.protocol, server.GetProtocol())
			require.Equal(t, tt.state, server.GetState())
			require.Equal(t, tt.version, server.GetVersion())
			require.Equal(t, provider, server.GetID().ProviderNamespace)
			require.Equal(t, "group", server.GetID().ResourceGroup)
			require.Equal(t, "subid", server.GetID().SubscriptionID)
			require.Equal(t, serverType, server.GetID().ResourceType)
			require.Equal(t, dbName, server.GetID().ResourceName)
			require.Equal(t, dbName, server.GetName())
			require.Equal(t, fqdn+":"+port, server.GetEndpoint())
			require.Equal(t, region, server.GetRegion())
			require.Equal(t, tags, server.GetTags())
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
