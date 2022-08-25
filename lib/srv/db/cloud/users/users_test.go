/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package users

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	clients "github.com/gravitational/teleport/lib/cloud"
	libaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	libsecrets "github.com/gravitational/teleport/lib/srv/db/secrets"
	"github.com/gravitational/trace"
)

var managedTags = map[string]string{
	"env":                        "test",
	libaws.TagKeyTeleportManaged: libaws.TagValueTrue,
}

func TestUsers(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	clock := clockwork.NewFakeClock()
	smMock := libsecrets.NewMockSecretsManagerClient(libsecrets.MockSecretsManagerClientConfig{
		Clock: clock,
	})
	ecMock := &cloud.ElastiCacheMock{}
	ecMock.AddMockUser(elastiCacheUser("alice", "group1"), managedTags)
	ecMock.AddMockUser(elastiCacheUser("bob", "group1", "group2"), managedTags)
	ecMock.AddMockUser(elastiCacheUser("charlie", "group2", "group3"), managedTags)
	ecMock.AddMockUser(elastiCacheUser("dan", "group3"), managedTags)
	ecMock.AddMockUser(elastiCacheUser("not-managed", "group1", "group2"), nil)

	mdbMock := &cloud.MemoryDBMock{}
	mdbMock.AddMockUser(memoryDBUser("alice", "acl1"), managedTags)
	mdbMock.AddMockUser(memoryDBUser("bob", "acl1", "acl2"), managedTags)
	mdbMock.AddMockUser(memoryDBUser("charlie", "acl2", "acl3"), managedTags)

	db1 := mustCreateElastiCacheDatabase(t, "db1", "group1")
	db2 := mustCreateElastiCacheDatabase(t, "db2", "group2")
	db3 := mustCreateElastiCacheDatabase(t, "db3", "group-not-found")
	db4 := mustCreateElastiCacheDatabase(t, "db4" /*no group*/)
	db5 := mustCreateRDSDatabase(t, "db5")
	db6 := mustCreateMemoryDBDatabase(t, "db6", "acl1")

	users, err := NewUsers(Config{
		Clients: &clients.TestCloudClients{
			ElastiCache:    ecMock,
			MemoryDB:       mdbMock,
			SecretsManager: smMock,
		},
		Clock: clock,
		UpdateMeta: func(_ context.Context, database types.Database) error {
			// Update db1 to group3 when setupAllDatabases.
			if database == db1 {
				db1Meta := db1.GetAWS()
				db1Meta.ElastiCache.UserGroupIDs = []string{"group3"}
				db1.SetStatusAWS(db1Meta)
			}
			return nil
		},
	})
	require.NoError(t, err)

	t.Run("setup single database", func(t *testing.T) {
		for _, database := range []types.Database{db1, db2, db3, db4, db5, db6} {
			users.setupDatabaseAndRotatePasswords(ctx, database)
		}

		requireDatabaseWithManagedUsers(t, users, db1, []string{"alice", "bob"})
		requireDatabaseWithManagedUsers(t, users, db2, []string{"bob", "charlie"})
		requireDatabaseWithManagedUsers(t, users, db3, nil)
		requireDatabaseWithManagedUsers(t, users, db4, nil)
		requireDatabaseWithManagedUsers(t, users, db5, nil)
		requireDatabaseWithManagedUsers(t, users, db6, []string{"alice", "bob"})
	})

	t.Run("setup all databases", func(t *testing.T) {
		clock.Advance(time.Hour)

		// Remove db2.
		users.setupAllDatabasesAndRotatePassowrds(ctx, types.Databases{db1, db3, db4, db5, db6})

		// Validate db1 is updated thourgh cfg.UpdateMeta.
		requireDatabaseWithManagedUsers(t, users, db1, []string{"charlie", "dan"})

		// Validate db2 is no longer tracked.
		_, err = users.GetPassword(ctx, db2, "charlie")
		require.True(t, trace.IsNotFound(err))

		// Validate db6 is same as before.
		requireDatabaseWithManagedUsers(t, users, db6, []string{"alice", "bob"})
	})
}

func requireDatabaseWithManagedUsers(t *testing.T, users *Users, db types.Database, managedUsers []string) {
	require.Equal(t, managedUsers, db.GetManagedUsers())
	for _, username := range managedUsers {
		password, err := users.GetPassword(context.TODO(), db, username)
		require.NoError(t, err)
		require.NotEmpty(t, password)
	}
}

func mustCreateElastiCacheDatabase(t *testing.T, name string, userGroupIDs ...string) types.Database {
	db, err := types.NewDatabaseV3(types.Metadata{
		Name: name,
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "master.redis-cluster.1234567890.use1.cache.amazonaws.com:6379",
		AWS: types.AWS{
			ElastiCache: types.ElastiCache{
				UserGroupIDs: userGroupIDs,
			},
		},
	})
	require.NoError(t, err)
	return db
}

func mustCreateMemoryDBDatabase(t *testing.T, name, aclName string) types.Database {
	db, err := types.NewDatabaseV3(types.Metadata{
		Name: name,
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      fmt.Sprintf("clustercfg.%v.xxxxxx.memorydb.us-east-1.amazonaws.com:6379", name),
		AWS: types.AWS{
			MemoryDB: types.MemoryDB{
				ACLName: aclName,
			},
		},
	})
	require.NoError(t, err)
	return db
}

func mustCreateRDSDatabase(t *testing.T, name string) types.Database {
	db, err := types.NewDatabaseV3(types.Metadata{
		Name: name,
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
	})
	require.NoError(t, err)
	return db
}

func elastiCacheUser(name string, groupIDs ...string) *elasticache.User {
	return &elasticache.User{
		UserId:       aws.String(name),
		ARN:          aws.String("arn:aws:elasticache:us-east-1:1234567890:user:" + name),
		UserName:     aws.String(name),
		UserGroupIds: aws.StringSlice(groupIDs),
	}
}

func memoryDBUser(name string, aclNames ...string) *memorydb.User {
	return &memorydb.User{
		ARN:      aws.String("arn:aws:memorydb:us-east-1:1234567890:user/" + name),
		Name:     aws.String(name),
		ACLNames: aws.StringSlice(aclNames),
	}
}
