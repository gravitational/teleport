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
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

func (process *TeleportProcess) initDatabases() {
	if len(process.Config.Databases.Databases) == 0 &&
		len(process.Config.Databases.ResourceMatchers) == 0 &&
		len(process.Config.Databases.AWSMatchers) == 0 {
		return
	}
	process.registerWithAuthServer(types.RoleDatabase, DatabasesIdentityEvent)
	process.RegisterCriticalFunc("db.init", process.initDatabaseService)
}

func (process *TeleportProcess) initDatabaseService() (retErr error) {
	log := process.log.WithField(trace.Component, teleport.Component(
		teleport.ComponentDatabase, process.id))

	eventsCh := make(chan Event)
	process.WaitForEvent(process.ExitContext(), DatabasesIdentityEvent, eventsCh)

	var event Event
	select {
	case event = <-eventsCh:
		log.Debugf("Received event %q.", event.Name)
	case <-process.ExitContext().Done():
		log.Debug("Process is exiting.")
		return nil
	}

	conn, ok := (event.Payload).(*Connector)
	if !ok {
		return trace.BadParameter("unsupported event payload type %q", event.Payload)
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
		tunnelAddrResolver = func() (*utils.NetAddr, error) {
			addr, ok := process.singleProcessMode(resp.GetProxyListenerMode())
			if !ok {
				return nil, trace.BadParameter("failed to find reverse tunnel address, " +
					"if running in a single-process mode, make sure auth_service, " +
					"proxy_service, and db_service are all enabled")
			}

			return addr, nil
		}
	}

	// Start uploader that will scan a path on disk and upload completed
	// sessions to the auth server.
	err = process.initUploaderService(accessPoint, conn.Client)
	if err != nil {
		return trace.Wrap(err)
	}

	// Create database resources from databases defined in the static configuration.
	var databases types.Databases
	for _, db := range process.Config.Databases.Databases {
		db, err := types.NewDatabaseV3(
			types.Metadata{
				Name:        db.Name,
				Description: db.Description,
				Labels:      db.StaticLabels,
			},
			types.DatabaseSpecV3{
				Protocol: db.Protocol,
				URI:      db.URI,
				CACert:   string(db.TLS.CACert),
				TLS: types.DatabaseTLS{
					CACert:     string(db.TLS.CACert),
					ServerName: db.TLS.ServerName,
					Mode:       db.TLS.Mode.ToProto(),
				},
				AWS: types.AWS{
					Region: db.AWS.Region,
					Redshift: types.Redshift{
						ClusterID: db.AWS.Redshift.ClusterID,
					},
					RDS: types.RDS{
						InstanceID: db.AWS.RDS.InstanceID,
						ClusterID:  db.AWS.RDS.ClusterID,
					},
				},
				GCP: types.GCPCloudSQL{
					ProjectID:  db.GCP.ProjectID,
					InstanceID: db.GCP.InstanceID,
				},
				DynamicLabels: types.LabelsToV2(db.DynamicLabels),
			})
		if err != nil {
			return trace.Wrap(err)
		}
		databases = append(databases, db)
	}

	clusterName := conn.ServerIdentity.Cert.Extensions[utils.CertExtensionAuthority]

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

	authorizer, err := auth.NewAuthorizer(clusterName, accessPoint, lockWatcher)
	if err != nil {
		return trace.Wrap(err)
	}
	tlsConfig, err := conn.ServerIdentity.TLSConfig(process.Config.CipherSuites)
	if err != nil {
		return trace.Wrap(err)
	}

	asyncEmitter, err := process.newAsyncEmitter(conn.Client)
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
		Authorizer:       authorizer,
		TLSConfig:        tlsConfig,
		Limiter:          connLimiter,
		GetRotation:      process.getRotation,
		Hostname:         process.Config.Hostname,
		HostID:           process.Config.HostUUID,
		Databases:        databases,
		ResourceMatchers: process.Config.Databases.ResourceMatchers,
		AWSMatchers:      process.Config.Databases.AWSMatchers,
		OnHeartbeat:      process.onHeartbeat(teleport.ComponentDatabase),
		LockWatcher:      lockWatcher,
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
			Component:   teleport.ComponentDatabase,
			HostUUID:    conn.ServerIdentity.ID.HostUUID,
			Resolver:    tunnelAddrResolver,
			Client:      conn.Client,
			Server:      dbService,
			AccessPoint: conn.Client,
			HostSigner:  conn.ServerIdentity.KeySigner,
			Cluster:     clusterName,
			FIPS:        process.Config.FIPS,
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
		log.Info("Shutting down.")
		if dbService != nil {
			warnOnErr(dbService.Close(), process.log)
		}
		if asyncEmitter != nil {
			warnOnErr(asyncEmitter.Close(), process.log)
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
