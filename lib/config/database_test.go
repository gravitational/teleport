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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
)

func TestMakeDatabaseConfig(t *testing.T) {
	t.Run("Global", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
		flags := DatabaseSampleFlags{
			RDSDiscoveryRegions: []string{"us-west-1", "us-west-2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AWSMatchers, 1)
		require.ElementsMatch(t, []string{"rds"}, databases.AWSMatchers[0].Types)
		require.ElementsMatch(t, flags.RDSDiscoveryRegions, databases.AWSMatchers[0].Regions)
	})

	t.Run("RDSProxyAutoDiscovery", func(t *testing.T) {
		t.Parallel()
		flags := DatabaseSampleFlags{
			RDSProxyDiscoveryRegions: []string{"us-west-1", "us-west-2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AWSMatchers, 1)
		require.ElementsMatch(t, []string{"rdsproxy"}, databases.AWSMatchers[0].Types)
		require.ElementsMatch(t, flags.RDSProxyDiscoveryRegions, databases.AWSMatchers[0].Regions)
	})

	t.Run("RedshiftAutoDiscovery", func(t *testing.T) {
		t.Parallel()
		flags := DatabaseSampleFlags{
			RedshiftDiscoveryRegions: []string{"us-west-1", "us-west-2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AWSMatchers, 1)
		require.ElementsMatch(t, []string{"redshift"}, databases.AWSMatchers[0].Types)
		require.ElementsMatch(t, flags.RedshiftDiscoveryRegions, databases.AWSMatchers[0].Regions)
	})

	t.Run("RedshiftServerlessAutoDiscovery", func(t *testing.T) {
		t.Parallel()
		flags := DatabaseSampleFlags{
			RedshiftServerlessDiscoveryRegions: []string{"us-west-1", "us-west-2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AWSMatchers, 1)
		require.ElementsMatch(t, []string{"redshift-serverless"}, databases.AWSMatchers[0].Types)
		require.ElementsMatch(t, flags.RedshiftServerlessDiscoveryRegions, databases.AWSMatchers[0].Regions)
	})

	t.Run("ElastiCacheAutoDiscovery", func(t *testing.T) {
		t.Parallel()
		flags := DatabaseSampleFlags{
			ElastiCacheDiscoveryRegions: []string{"us-west-1", "us-west-2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AWSMatchers, 1)
		require.ElementsMatch(t, []string{"elasticache"}, databases.AWSMatchers[0].Types)
		require.ElementsMatch(t, flags.ElastiCacheDiscoveryRegions, databases.AWSMatchers[0].Regions)
	})

	t.Run("MemoryDBAutoDiscovery", func(t *testing.T) {
		t.Parallel()
		flags := DatabaseSampleFlags{
			MemoryDBDiscoveryRegions: []string{"us-west-1", "us-west-2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AWSMatchers, 1)
		require.ElementsMatch(t, []string{"memorydb"}, databases.AWSMatchers[0].Types)
		require.ElementsMatch(t, flags.MemoryDBDiscoveryRegions, databases.AWSMatchers[0].Regions)
	})

	t.Run("OpenSearchAutoDiscovery", func(t *testing.T) {
		t.Parallel()
		flags := DatabaseSampleFlags{
			OpenSearchDiscoveryRegions: []string{"us-west-1", "us-west-2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AWSMatchers, 1)
		require.ElementsMatch(t, []string{"opensearch"}, databases.AWSMatchers[0].Types)
		require.ElementsMatch(t, flags.OpenSearchDiscoveryRegions, databases.AWSMatchers[0].Regions)
	})

	t.Run("AWS discovery tags", func(t *testing.T) {
		t.Parallel()
		flags := DatabaseSampleFlags{
			RedshiftServerlessDiscoveryRegions: []string{"us-west-1", "us-west-2"},
			AWSRawTags:                         "teleport.dev/discovery=true,env=prod",
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AWSMatchers, 1)
		require.ElementsMatch(t, []string{"redshift-serverless"}, databases.AWSMatchers[0].Types)
		require.ElementsMatch(t, flags.RedshiftServerlessDiscoveryRegions, databases.AWSMatchers[0].Regions)
		require.Equal(t, map[string]apiutils.Strings{
			"teleport.dev/discovery": {"true"},
			"env":                    {"prod"},
		}, databases.AWSMatchers[0].Tags)
	})

	t.Run("AzureMySQLAutoDiscovery", func(t *testing.T) {
		t.Parallel()
		flags := DatabaseSampleFlags{
			AzureMySQLDiscoveryRegions: []string{"eastus", "eastus2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AzureMatchers, 1)
		require.ElementsMatch(t, []string{"mysql"}, databases.AzureMatchers[0].Types)
		require.ElementsMatch(t, flags.AzureMySQLDiscoveryRegions, databases.AzureMatchers[0].Regions)
	})

	t.Run("AzurePostgresAutoDiscovery", func(t *testing.T) {
		t.Parallel()
		flags := DatabaseSampleFlags{
			AzurePostgresDiscoveryRegions: []string{"eastus", "eastus2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AzureMatchers, 1)
		require.ElementsMatch(t, []string{"postgres"}, databases.AzureMatchers[0].Types)
		require.ElementsMatch(t, flags.AzurePostgresDiscoveryRegions, databases.AzureMatchers[0].Regions)
	})

	t.Run("AzureSQLServerAutoDiscovery", func(t *testing.T) {
		t.Parallel()
		flags := DatabaseSampleFlags{
			AzureSQLServerDiscoveryRegions: []string{"eastus", "eastus2"},
		}

		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AzureMatchers, 1)
		require.ElementsMatch(t, []string{"sqlserver"}, databases.AzureMatchers[0].Types)
		require.ElementsMatch(t, flags.AzureSQLServerDiscoveryRegions, databases.AzureMatchers[0].Regions)
	})

	t.Run("Azure discovery tags,subscriptions,resource_groups", func(t *testing.T) {
		t.Parallel()
		flags := DatabaseSampleFlags{
			AzureRedisDiscoveryRegions: []string{"eastus", "eastus2"},
			AzureSubscriptions:         []string{"sub1", "sub2"},
			AzureResourceGroups:        []string{"group1", "group2"},
			AzureRawTags:               "TeleportDiscovery=true,Env=prod",
		}
		databases := generateAndParseConfig(t, flags)
		require.Len(t, databases.AzureMatchers, 1)
		require.ElementsMatch(t, []string{"redis"}, databases.AzureMatchers[0].Types)
		require.ElementsMatch(t, flags.AzureRedisDiscoveryRegions, databases.AzureMatchers[0].Regions)
		require.ElementsMatch(t, flags.AzureSubscriptions, databases.AzureMatchers[0].Subscriptions)
		require.ElementsMatch(t, flags.AzureResourceGroups, databases.AzureMatchers[0].ResourceGroups)
		require.Equal(t, map[string]apiutils.Strings{
			"TeleportDiscovery": {"true"},
			"Env":               {"prod"},
		}, databases.AzureMatchers[0].ResourceTags)
	})

	t.Run("StaticDatabase", func(t *testing.T) {
		t.Parallel()
		tempdir := t.TempDir()
		pemfile := filepath.Join(tempdir, "db-ca.pem")
		os.WriteFile(pemfile, []byte{}, 0777)
		keytabfile := filepath.Join(tempdir, "db.keytab")
		os.WriteFile(keytabfile, []byte{}, 0777)

		tests := map[string]struct {
			flags             DatabaseSampleFlags
			wantCommandLabels []CommandLabel
			wantStaticLabels  map[string]string
			requireFn         require.ErrorAssertionFunc
		}{
			"SelfHosted": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:      "sample",
					StaticDatabaseProtocol:  "postgres",
					StaticDatabaseURI:       "localhost:5432",
					StaticDatabaseRawLabels: `env=prod,arch=[5m2s:/bin/uname -m "p1 p2"]`,
					DatabaseCACertFile:      pemfile,
				},
				wantStaticLabels: map[string]string{
					"env": "prod",
				},
				wantCommandLabels: []CommandLabel{
					{
						Name:    "arch",
						Period:  time.Minute*5 + time.Second*2,
						Command: []string{"/bin/uname", "-m", `"p1 p2"`},
					},
				},
				requireFn: require.NoError,
			},
			"AWSKeyspaces": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:       "sample",
					StaticDatabaseProtocol:   "cassandra",
					StaticDatabaseURI:        "cassandra.us-west-1.amazonaws.com",
					DatabaseCACertFile:       pemfile,
					DatabaseAWSRegion:        "us-west-1",
					DatabaseAWSAccountID:     "123456789012",
					DatabaseAWSAssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					DatabaseAWSExternalID:    "externalID123",
				},
				requireFn: require.NoError,
			},
			"AWSKeyspacesDeriveURIFromAWSRegion": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:       "sample",
					StaticDatabaseProtocol:   "cassandra",
					StaticDatabaseURI:        "",
					DatabaseCACertFile:       pemfile,
					DatabaseAWSRegion:        "us-west-1",
					DatabaseAWSAccountID:     "123456789012",
					DatabaseAWSAssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					DatabaseAWSExternalID:    "externalID123",
				},
				requireFn: require.NoError,
			},
			"AWSRedshift": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:           "sample",
					StaticDatabaseProtocol:       "postgres",
					StaticDatabaseURI:            "redshift-cluster-1.abcdefghijklmnop.us-west-1.redshift.amazonaws.com:5439",
					DatabaseAWSRegion:            "us-west-1",
					DatabaseAWSRedshiftClusterID: "redshift-cluster-1",
					DatabaseAWSAssumeRoleARN:     "arn:aws:iam::123456789012:role/DBDiscoverer",
					DatabaseAWSExternalID:        "externalID123",
				},
				requireFn: require.NoError,
			},
			"AWSRDSInstance": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:       "sample",
					StaticDatabaseProtocol:   "postgres",
					StaticDatabaseURI:        "rds-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
					DatabaseAWSRegion:        "us-west-1",
					DatabaseAWSRDSInstanceID: "rsd-instance-1",
					DatabaseAWSAssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					DatabaseAWSExternalID:    "externalID123",
				},
				requireFn: require.NoError,
			},
			"AWSRDSCluster": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:       "sample",
					StaticDatabaseProtocol:   "postgres",
					StaticDatabaseURI:        "aurora-cluster-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
					DatabaseAWSRegion:        "us-west-1",
					DatabaseAWSRDSClusterID:  "aurora-cluster-1",
					DatabaseAWSAssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					DatabaseAWSExternalID:    "externalID123",
				},
				requireFn: require.NoError,
			},
			"AWSMemoryDB": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:             "sample",
					StaticDatabaseProtocol:         "redis",
					StaticDatabaseURI:              "clustercfg.my-memorydb.xxxxxx.memorydb.us-east-1.amazonaws.com:6379",
					DatabaseAWSRegion:              "us-west-1",
					DatabaseAWSMemoryDBClusterName: "my-memorydb",
					DatabaseAWSAssumeRoleARN:       "arn:aws:iam::123456789012:role/DBDiscoverer",
					DatabaseAWSExternalID:          "externalID123",
				},
				requireFn: require.NoError,
			},
			"AWSElastieCache": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:            "sample",
					StaticDatabaseProtocol:        "redis",
					StaticDatabaseURI:             "master.redis-cluster-example.abcdef.usw1.cache.amazonaws.com:6379",
					DatabaseAWSRegion:             "us-west-1",
					DatabaseAWSElastiCacheGroupID: "redis-cluster-example",
					DatabaseAWSAssumeRoleARN:      "arn:aws:iam::123456789012:role/DBDiscoverer",
					DatabaseAWSExternalID:         "externalID123",
				},
				requireFn: require.NoError,
			},
			"AD": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:     "sample",
					StaticDatabaseProtocol: "postgres",
					StaticDatabaseURI:      "localhost:5432",
					DatabaseADDomain:       "EXAMPLE.com",
					DatabaseADSPN:          "MSSQLSvc/ec2amaz-4kn05du.dbadir.teleportdemo.net:1433",
					DatabaseADKeytabFile:   keytabfile,
				},
				requireFn: require.NoError,
			},
			"GCP": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:     "sample",
					StaticDatabaseProtocol: "postgres",
					StaticDatabaseURI:      "localhost:5432",
					DatabaseGCPProjectID:   "xxx-1234",
					DatabaseGCPInstanceID:  "example",
				},
				requireFn: require.NoError,
			},
			"DynamoDBDeriveURIFromAWSRegion": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:       "sample",
					StaticDatabaseProtocol:   "dynamodb",
					DatabaseAWSAccountID:     "123456789012",
					DatabaseAWSRegion:        "us-west-1",
					DatabaseAWSAssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					DatabaseAWSExternalID:    "externalID123",
				},
				requireFn: require.NoError,
			},
			"MissingName": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:     "",
					StaticDatabaseProtocol: "postgres",
					StaticDatabaseURI:      "localhost:5432",
				},
				requireFn: require.Error,
			},
			"MissingProtocol": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:     "sample",
					StaticDatabaseProtocol: "",
					StaticDatabaseURI:      "localhost:5432",
				},
				requireFn: require.Error,
			},
			"MissingRequiredURI": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:     "sample",
					StaticDatabaseProtocol: "postgres",
					StaticDatabaseURI:      "",
				},
				requireFn: require.Error,
			},
			"BadURI": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:     "sample",
					StaticDatabaseProtocol: "postgres",
					StaticDatabaseURI:      "postgres://localhost:5432",
				},
				requireFn: require.Error,
			},
			"InvalidLabels": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:      "sample",
					StaticDatabaseProtocol:  "postgres",
					StaticDatabaseURI:       "localhost:5432",
					StaticDatabaseRawLabels: "abc",
				},
				requireFn: require.Error,
			},
			"MissingRequiredAWSAccount": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:     "sample",
					StaticDatabaseProtocol: "dynamodb",
					StaticDatabaseURI:      "dynamodb.us-west-1.amazonaws.com",
					DatabaseAWSRegion:      "us-west-1",
					DatabaseAWSAccountID:   "",
				},
				requireFn: require.Error,
			},
			"MissingAWSRegionAndURI": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:     "sample",
					StaticDatabaseProtocol: "dynamodb",
					DatabaseAWSAccountID:   "123456789012",
				},
				requireFn: require.Error,
			},
			"AWSExternalIDMissingAWSRoleARN": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:       "sample",
					StaticDatabaseProtocol:   "postgres",
					StaticDatabaseURI:        "aurora-cluster-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
					DatabaseAWSRegion:        "us-west-1",
					DatabaseAWSRDSClusterID:  "aurora-cluster-1",
					DatabaseAWSAssumeRoleARN: "", // missing role arn raises error because external id is set.
					DatabaseAWSExternalID:    "externalID123",
				},
				requireFn: require.Error,
			},
			"MissingAWSRoleARNName": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:       "sample",
					StaticDatabaseProtocol:   "postgres",
					StaticDatabaseURI:        "aurora-cluster-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
					DatabaseAWSRegion:        "us-west-1",
					DatabaseAWSRDSClusterID:  "aurora-cluster-1",
					DatabaseAWSAssumeRoleARN: "arn:aws:iam::123456789012:role", // missing role name
					DatabaseAWSExternalID:    "externalID123",
				},
				requireFn: require.Error,
			},
			"InvalidAWSRoleARNFormat": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:       "sample",
					StaticDatabaseProtocol:   "postgres",
					StaticDatabaseURI:        "aurora-cluster-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
					DatabaseAWSRegion:        "us-west-1",
					DatabaseAWSRDSClusterID:  "aurora-cluster-1",
					DatabaseAWSAssumeRoleARN: "foobar",
					DatabaseAWSExternalID:    "externalID123",
				},
				requireFn: require.Error,
			},
			"InvalidAWSRoleARNResourceService": {
				flags: DatabaseSampleFlags{
					StaticDatabaseName:       "sample",
					StaticDatabaseProtocol:   "postgres",
					StaticDatabaseURI:        "aurora-cluster-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
					DatabaseAWSRegion:        "us-west-1",
					DatabaseAWSRDSClusterID:  "aurora-cluster-1",
					DatabaseAWSAssumeRoleARN: "arn:aws:sts::123456789012:federated-user/Alice", // sts != iam
					DatabaseAWSExternalID:    "externalID123",
				},
				requireFn: require.Error,
			},
		}

		for name, tt := range tests {
			tt := tt
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				configString, err := MakeDatabaseAgentConfigString(tt.flags)
				tt.requireFn(t, err)
				if err != nil {
					return
				}

				fc, err := ReadConfig(bytes.NewBuffer([]byte(configString)))
				require.NoError(t, err)

				require.True(t, fc.Databases.Enabled())
				databases := fc.Databases
				require.Len(t, databases.Databases, 1)
				got := databases.Databases[0]
				require.Equal(t, tt.flags.StaticDatabaseName, got.Name)
				require.Equal(t, tt.flags.StaticDatabaseProtocol, got.Protocol)
				require.Equal(t, tt.flags.StaticDatabaseURI, got.URI)
				require.Equal(t, tt.wantStaticLabels, got.StaticLabels)
				require.ElementsMatch(t, tt.wantCommandLabels, got.DynamicLabels)
				require.Equal(t, tt.flags.DatabaseAWSRegion, got.AWS.Region)
				require.Equal(t, tt.flags.DatabaseAWSAccountID, got.AWS.AccountID)
				require.Equal(t, tt.flags.DatabaseAWSAssumeRoleARN, got.AWS.AssumeRoleARN)
				require.Equal(t, tt.flags.DatabaseAWSExternalID, got.AWS.ExternalID)
				require.Equal(t, tt.flags.DatabaseAWSRedshiftClusterID, got.AWS.Redshift.ClusterID)
				require.Equal(t, tt.flags.DatabaseAWSRDSClusterID, got.AWS.RDS.ClusterID)
				require.Equal(t, tt.flags.DatabaseAWSRDSInstanceID, got.AWS.RDS.InstanceID)
				require.Equal(t, tt.flags.DatabaseAWSElastiCacheGroupID, got.AWS.ElastiCache.ReplicationGroupID)
				require.Equal(t, tt.flags.DatabaseAWSMemoryDBClusterName, got.AWS.MemoryDB.ClusterName)
				require.Equal(t, tt.flags.DatabaseADDomain, got.AD.Domain)
				require.Equal(t, tt.flags.DatabaseADSPN, got.AD.SPN)
				require.Equal(t, tt.flags.DatabaseADKeytabFile, got.AD.KeytabFile)
				require.Equal(t, tt.flags.DatabaseGCPProjectID, got.GCP.ProjectID)
				require.Equal(t, tt.flags.DatabaseGCPInstanceID, got.GCP.InstanceID)
				require.Equal(t, tt.flags.DatabaseCACertFile, got.TLS.CACertFile)
			})
		}
	})

	t.Run("resource matchers", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			flags := DatabaseSampleFlags{}
			databases := generateAndParseConfig(t, flags)
			require.Len(t, databases.ResourceMatchers, 0)
		})

		t.Run("multiple labels", func(t *testing.T) {
			flags := DatabaseSampleFlags{
				DynamicResourcesRawLabels: []string{
					"env=dev",
					"env=prod,name=my-name",
				},
			}
			databases := generateAndParseConfig(t, flags)
			require.Equal(t, []ResourceMatcher{
				{
					Labels: types.Labels{
						"env": apiutils.Strings{"dev"},
					},
				},
				{
					Labels: types.Labels{
						"name": apiutils.Strings{"my-name"},
						"env":  apiutils.Strings{"prod"},
					},
				},
			}, databases.ResourceMatchers)
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
