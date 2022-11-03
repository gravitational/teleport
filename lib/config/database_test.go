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

package config

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMakeDatabaseConfig(t *testing.T) {
	t.Run("Global", func(t *testing.T) {
		flags := DatabaseSampleFlags{
			NodeName:    "testlocal",
			DataDir:     "/var/lib/data",
			ProxyServer: "localhost:3080",
			AuthToken:   "/tmp/token.txt",
			CAPins:      []string{"pin-1", "pin-2"},
		}

		configString, err := MakeDatabaseAgentConfigString(flags)
		require.NoError(t, err)

		fileConfig, err := ReadConfig(bytes.NewBuffer([]byte(configString)))
		require.NoError(t, err)

		require.Equal(t, flags.NodeName, fileConfig.NodeName)
		require.Equal(t, flags.DataDir, fileConfig.DataDir)
		require.Equal(t, flags.ProxyServer, fileConfig.ProxyServer)
		require.Equal(t, flags.AuthToken, fileConfig.AuthToken)
		require.ElementsMatch(t, flags.CAPins, fileConfig.CAPin)
	})

	t.Run("RDSAutoDiscovery", func(t *testing.T) {
		flags := DatabaseSampleFlags{
			RDSDiscoveryRegions: []string{"us-west-1", "us-west-2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AWSMatchers, 1)
		require.ElementsMatch(t, []string{"rds"}, databases.AWSMatchers[0].Types)
		require.ElementsMatch(t, flags.RDSDiscoveryRegions, databases.AWSMatchers[0].Regions)
	})

	t.Run("RDSProxyAutoDiscovery", func(t *testing.T) {
		flags := DatabaseSampleFlags{
			RDSProxyDiscoveryRegions: []string{"us-west-1", "us-west-2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AWSMatchers, 1)
		require.ElementsMatch(t, []string{"rdsproxy"}, databases.AWSMatchers[0].Types)
		require.ElementsMatch(t, flags.RDSProxyDiscoveryRegions, databases.AWSMatchers[0].Regions)
	})

	t.Run("RedshiftAutoDiscovery", func(t *testing.T) {
		flags := DatabaseSampleFlags{
			RedshiftDiscoveryRegions: []string{"us-west-1", "us-west-2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AWSMatchers, 1)
		require.ElementsMatch(t, []string{"redshift"}, databases.AWSMatchers[0].Types)
		require.ElementsMatch(t, flags.RedshiftDiscoveryRegions, databases.AWSMatchers[0].Regions)
	})

	t.Run("AzureMySQLAutoDiscovery", func(t *testing.T) {
		flags := DatabaseSampleFlags{
			AzureMySQLDiscoveryRegions: []string{"eastus", "eastus2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AzureMatchers, 1)
		require.ElementsMatch(t, []string{"mysql"}, databases.AzureMatchers[0].Types)
		require.ElementsMatch(t, flags.AzureMySQLDiscoveryRegions, databases.AzureMatchers[0].Regions)
	})

	t.Run("AzurePostgresAutoDiscovery", func(t *testing.T) {
		flags := DatabaseSampleFlags{
			AzurePostgresDiscoveryRegions: []string{"eastus", "eastus2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AzureMatchers, 1)
		require.ElementsMatch(t, []string{"postgres"}, databases.AzureMatchers[0].Types)
		require.ElementsMatch(t, flags.AzurePostgresDiscoveryRegions, databases.AzureMatchers[0].Regions)
	})

	t.Run("StaticDatabase", func(t *testing.T) {
		flags := DatabaseSampleFlags{
			StaticDatabaseName:           "sample",
			StaticDatabaseProtocol:       "postgres",
			StaticDatabaseURI:            "postgres://localhost:5432",
			StaticDatabaseRawLabels:      `env=prod,arch=[5m2s:/bin/uname -m "p1 p2"]`,
			DatabaseAWSRegion:            "us-west-1",
			DatabaseAWSRedshiftClusterID: "redshift-cluster-1",
			DatabaseADDomain:             "EXAMPLE.com",
			DatabaseADSPN:                "MSSQLSvc/ec2amaz-4kn05du.dbadir.teleportdemo.net:1433",
			DatabaseADKeytabFile:         "/path/to/keytab",
			DatabaseGCPProjectID:         "xxx-1234",
			DatabaseGCPInstanceID:        "example",
			DatabaseCACertFile:           "/path/to/pem",
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.Databases, 1)
		require.Equal(t, flags.StaticDatabaseName, databases.Databases[0].Name)
		require.Equal(t, flags.StaticDatabaseProtocol, databases.Databases[0].Protocol)
		require.Equal(t, flags.StaticDatabaseURI, databases.Databases[0].URI)
		require.Equal(t, map[string]string{"env": "prod"}, databases.Databases[0].StaticLabels)
		require.Equal(t, flags.DatabaseAWSRegion, databases.Databases[0].AWS.Region)
		require.Equal(t, flags.DatabaseAWSRedshiftClusterID, databases.Databases[0].AWS.Redshift.ClusterID)
		require.Equal(t, flags.DatabaseADDomain, databases.Databases[0].AD.Domain)
		require.Equal(t, flags.DatabaseADSPN, databases.Databases[0].AD.SPN)
		require.Equal(t, flags.DatabaseADKeytabFile, databases.Databases[0].AD.KeytabFile)
		require.Equal(t, flags.DatabaseGCPProjectID, databases.Databases[0].GCP.ProjectID)
		require.Equal(t, flags.DatabaseGCPInstanceID, databases.Databases[0].GCP.InstanceID)
		require.Equal(t, flags.DatabaseCACertFile, databases.Databases[0].TLS.CACertFile)

		require.Len(t, databases.Databases[0].DynamicLabels, 1)
		require.ElementsMatch(t, []CommandLabel{
			{
				Name:    "arch",
				Period:  time.Minute*5 + time.Second*2,
				Command: []string{"/bin/uname", "-m", `"p1 p2"`},
			},
		}, databases.Databases[0].DynamicLabels)

		t.Run("MissingFields", func(t *testing.T) {
			tests := map[string]struct {
				name     string
				protocol string
				uri      string
				tags     string
			}{
				"Name":        {protocol: "postgres", uri: "postgres://localhost:5432"},
				"Protocol":    {name: "sample", uri: "postgres://localhost:5432"},
				"URI":         {name: "sample", protocol: "postgres"},
				"InvalidTags": {name: "sample", protocol: "postgres", uri: "postgres://localhost:5432", tags: "abc"},
			}

			for name, test := range tests {
				t.Run(name, func(t *testing.T) {
					flags := DatabaseSampleFlags{
						StaticDatabaseName:      test.name,
						StaticDatabaseProtocol:  test.protocol,
						StaticDatabaseURI:       test.uri,
						StaticDatabaseRawLabels: test.tags,
					}

					_, err := MakeDatabaseAgentConfigString(flags)
					require.Error(t, err)
				})
			}

		})
	})
}

// generateAndParse generetes config using provided flags, parse them using
// `ReadConfig`, checks if the Database service is enable and return it.
func generateAndParseConfig(t *testing.T, flags DatabaseSampleFlags) Databases {
	configString, err := MakeDatabaseAgentConfigString(flags)
	require.NoError(t, err)

	fileConfig, err := ReadConfig(bytes.NewBuffer([]byte(configString)))
	require.NoError(t, err)

	require.True(t, fileConfig.Databases.Enabled())
	return fileConfig.Databases
}
