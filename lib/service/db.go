/*
Copyright 2020-2021 Gravitational, Inc.

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

package service

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/db"
)

func (process *TeleportProcess) shouldInitDatabases() bool {
	databasesCfg := len(process.Config.Databases.Databases) > 0
	resourceMatchersCfg := len(process.Config.Databases.ResourceMatchers) > 0
	awsMatchersCfg := len(process.Config.Databases.AWSMatchers) > 0
	azureMatchersCfg := len(process.Config.Databases.AzureMatchers) > 0
	anyCfg := databasesCfg || resourceMatchersCfg || awsMatchersCfg || azureMatchersCfg

	return process.Config.Databases.Enabled && anyCfg
}

func (process *TeleportProcess) initDatabases() {
	process.RegisterWithAuthServer(types.RoleDatabase, DatabasesIdentityEvent)
	process.RegisterCriticalFunc("db.init", process.initDatabaseService)
}

func (process *TeleportProcess) initDatabaseService() (retErr error) {
	log := process.log.WithField(trace.Component, teleport.Component(
		teleport.ComponentDatabase, process.id))

	conn, err := process.WaitForConnector(DatabasesIdentityEvent, log)
	if conn == nil {
		return trace.Wrap(err)
	}

	accessPoint, err := process.newLocalCacheForDatabase(conn.Client, []string{teleport.ComponentDatabase})
	if err != nil {
		return trace.Wrap(err)
	}
	resp, err := accessPoint.GetClusterNetworkingConfig(process.ExitContext())
	if err != nil {
		return trace.Wrap(err)
	}

	tunnelAddrResolver := conn.TunnelProxyResolver()
	if tunnelAddrResolver == nil {
		tunnelAddrResolver = process.SingleProcessModeResolver(resp.GetProxyListenerMode())

		// run the resolver. this will check configuration for errors.
		_, _, err := tunnelAddrResolver(process.ExitContext())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Create database resources from databases defined in the static configuration.
	var databases types.Databases
	for _, db := range process.Config.Databases.Databases {
		db, err := db.ToDatabase()
		if err != nil {
			return trace.Wrap(err)
		}
		databases = append(databases, db)
	}

	lockWatcher, err := services.NewLockWatcher(process.ExitContext(), services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentDatabase,
			Log:       log,
			Client:    conn.Client,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	clusterName := conn.ServerIdentity.ClusterName
	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: clusterName,
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
		Logger:      log,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	tlsConfig, err := conn.ServerIdentity.TLSConfig(process.Config.CipherSuites)
	if err != nil {
		return trace.Wrap(err)
	}

	asyncEmitter, err := process.NewAsyncEmitter(conn.Client)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if retErr != nil {
			warnOnErr(asyncEmitter.Close(), process.log)
		}
	}()

	streamer, err := events.NewCheckingStreamer(events.CheckingStreamerConfig{
		Inner:       conn.Client,
		Clock:       process.Clock,
		ClusterName: clusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	connLimiter, err := limiter.NewLimiter(process.Config.Databases.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyGetter := reversetunnel.NewConnectedProxyGetter()

	connMonitor, err := srv.NewConnectionMonitor(srv.ConnectionMonitorConfig{
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
		Clock:       process.Config.Clock,
		ServerID:    process.Config.HostUUID,
		Emitter:     asyncEmitter,
		Logger:      process.log,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Create and start the database service.
	dbService, err := db.New(process.ExitContext(), db.Config{
		Clock:       process.Clock,
		DataDir:     process.Config.DataDir,
		AuthClient:  conn.Client,
		AccessPoint: accessPoint,
		StreamEmitter: &events.StreamerAndEmitter{
			Emitter:  asyncEmitter,
			Streamer: streamer,
		},
		Authorizer:           authorizer,
		TLSConfig:            tlsConfig,
		Limiter:              connLimiter,
		GetRotation:          process.GetRotation,
		Hostname:             process.Config.Hostname,
		HostID:               process.Config.HostUUID,
		Databases:            databases,
		CloudLabels:          process.cloudLabels,
		ResourceMatchers:     process.Config.Databases.ResourceMatchers,
		AWSMatchers:          process.Config.Databases.AWSMatchers,
		AzureMatchers:        process.Config.Databases.AzureMatchers,
		OnHeartbeat:          process.OnHeartbeat(teleport.ComponentDatabase),
		ConnectionMonitor:    connMonitor,
		ConnectedProxyGetter: proxyGetter,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := dbService.Start(process.ExitContext()); err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if retErr != nil {
			warnOnErr(dbService.Close(), process.log)
		}
	}()

	// Create and start the agent pool.
	agentPool, err := reversetunnel.NewAgentPool(
		process.ExitContext(),
		reversetunnel.AgentPoolConfig{
			Component:            teleport.ComponentDatabase,
			HostUUID:             conn.ServerIdentity.ID.HostUUID,
			Resolver:             tunnelAddrResolver,
			Client:               conn.Client,
			Server:               dbService,
			AccessPoint:          conn.Client,
			HostSigner:           conn.ServerIdentity.KeySigner,
			Cluster:              clusterName,
			FIPS:                 process.Config.FIPS,
			ConnectedProxyGetter: proxyGetter,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := agentPool.Start(); err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if retErr != nil {
			agentPool.Stop()
		}
	}()

	// Execute this when the process running database proxy service exits.
	process.OnExit("db.stop", func(payload interface{}) {
		if dbService != nil {
			if payload == nil {
				log.Info("Shutting down immediately.")
				warnOnErr(dbService.Close(), log)
			} else {
				log.Info("Shutting down gracefully.")
				warnOnErr(dbService.Shutdown(payloadContext(payload, log)), log)
			}
		}
		if asyncEmitter != nil {
			warnOnErr(asyncEmitter.Close(), log)
		}
		if agentPool != nil {
			agentPool.Stop()
		}
		warnOnErr(conn.Close(), log)
		log.Info("Exited.")
	})

	process.BroadcastEvent(Event{Name: DatabasesReady, Payload: nil})
	log.Infof("Database service has successfully started: %v.", databases)

	// Block and wait while the server and agent pool are running.
	if err := dbService.Wait(); err != nil {
		return trace.Wrap(err)
	}
	agentPool.Wait()

	return nil
}
