/*
Copyright 2023 Gravitational, Inc.

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

package mongodb

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	mongooptions "go.mongodb.org/mongo-driver/mongo/options"
	mongoauth "go.mongodb.org/mongo-driver/x/mongo/driver/auth"

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

// adminClient defines a subset of functions provided by *mongo.Client
type adminClient interface {
	Database(name string, opts ...*options.DatabaseOptions) *mongo.Database
	Disconnect(ctx context.Context) error
}

func connectAsAdmin(ctx context.Context, sessionCtx *common.Session, e *Engine) (adminClient, error) {
	if adminClientFnCache == nil {
		client, err := makeRawAdminClient(ctx, sessionCtx, e)
		return client, trace.Wrap(err)
	}

	key := newAdminClientCacheKey(sessionCtx)
	adminClient, err := utils.FnCacheGet(ctx, adminClientFnCache, key, func(ctx context.Context) (*sharedAdminClient, error) {
		rawClient, err := makeRawAdminClient(ctx, sessionCtx, e)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return newSharedAdminClient(rawClient, e.Clock, adminClientCleanupTTL), nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return adminClient.acquire(), nil
}

type adminClientCacheKey struct {
	databaseName string
	databaseURI  string
	adminUser    string
}

func newAdminClientCacheKey(sessionCtx *common.Session) adminClientCacheKey {
	return adminClientCacheKey{
		databaseName: sessionCtx.Database.GetName(),
		adminUser:    sessionCtx.Database.GetAdminUser().Name,
		// Just in case the same database name is updated to a different database server.
		databaseURI: sessionCtx.Database.GetURI(),
	}
}

type sharedAdminClient struct {
	*mongo.Client
	clock clockwork.Clock
	wg    sync.WaitGroup
}

func newSharedAdminClient(rawClient *mongo.Client, clock clockwork.Clock, ttl time.Duration) *sharedAdminClient {
	c := &sharedAdminClient{
		Client: rawClient,
	}

	// Cleanup goroutine.
	go func() {
		<-clock.After(ttl)

		// Make sure all shared sessions are released.
		c.wg.Wait()
		if err := c.Client.Disconnect(context.Background()); err != nil {
			logrus.Debugf("Failed to disconnect mongodb client: %v.", err)
		} else {
			logrus.Debugf("Terminated a MongoDB connection for admin user.")
		}
	}()
	return c
}

func (c *sharedAdminClient) acquire() *sharedAdminClient {
	c.wg.Add(1)
	return c
}

func (c *sharedAdminClient) Disconnect(_ context.Context) error {
	c.wg.Done()
	return nil
}

func makeRawAdminClient(ctx context.Context, sessionCtx *common.Session, e *Engine) (*mongo.Client, error) {
	e.Log.Debugf("Creating new connection as MongoDB admin user %v on %v.", sessionCtx.Database.GetAdminUser().Name, sessionCtx.Database.GetName())

	sessionCtx = sessionCtx.WithUser(sessionCtx.Database.GetAdminUser().Name)

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientCfg, err := makeClientOptionsFromDatabaseURI(sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientCfg.SetTLSConfig(tlsConfig)
	clientCfg.SetAuth(mongooptions.Credential{
		AuthMechanism: mongoauth.MongoDBX509,
		Username:      x509Username(sessionCtx),
	})

	client, err := mongo.Connect(ctx, clientCfg)
	return client, trace.Wrap(err)
}

const (
	// adminClientFnCacheTTL specifies how long the sharedAdminClient will be
	// cached in FnCache.
	adminClientFnCacheTTL = time.Minute
	// adminClientCleanupTTL specifies how long to wait to terminate the
	// sharedAdminClient. Use a longer TTL than adminClientFnCacheTTL to make
	// sure the sharedAdminClient will be not returned by the FnCache while
	// getting terminated.
	adminClientCleanupTTL = adminClientFnCacheTTL + time.Second*10
)

var adminClientFnCache *utils.FnCache

func init() {
	if os.Getenv("TELEPORT_DISABLE_MONGODB_ADMIN_CLIENT_CACHE") != "" {
		return
	}

	var err error
	adminClientFnCache, err = utils.NewFnCache(utils.FnCacheConfig{
		TTL: adminClientFnCacheTTL,
	})
	if err != nil {
		logrus.Warn("Failed to create MongoDB admin connection cache: %v.", err)
	}
}
