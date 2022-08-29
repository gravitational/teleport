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

package service

import (
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery"
	"github.com/gravitational/trace"
)

func (process *TeleportProcess) shouldInitDiscovery() bool {
	// todo(amk): add a Config.Discovery ?
	return process.Config.SSH.Enabled && len(process.Config.Discovery.AWSMatchers) != 0
}

func (process *TeleportProcess) initDiscovery() {
	process.registerWithAuthServer(types.RoleDiscovery, DiscoveryIdentityEvent)
	process.RegisterCriticalFunc("discovery.init", process.initDiscoveryService)
}

func (process *TeleportProcess) initDiscoveryService() error {
	log := process.log.WithField(trace.Component, teleport.Component(
		teleport.ComponentDiscovery, process.id))

	conn, err := process.waitForConnector(DiscoveryIdentityEvent, log)
	if conn == nil {
		return trace.Wrap(err)
	}

	// todo(amk): prefer node watcher or accessPoint for caching?
	// accessPoint, err := process.newLocalCacheForDiscovery(conn.Client,
	// 	[]string{teleport.ComponentDiscovery})
	// if err != nil {
	// 	return trace.Wrap(err)
	// }

	nodeWatcher, err := services.NewNodeWatcher(process.ExitContext(), services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentDiscovery,
			Log:       process.log.WithField(trace.Component, teleport.ComponentDiscovery),
			Client:    conn.Client,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	clusterName := conn.ServerIdentity.ClusterName

	// asyncEmitter makes sure that sessions do not block
	// in case if connections are slow
	asyncEmitter, err := process.newAsyncEmitter(conn.Client)
	if err != nil {
		return trace.Wrap(err)
	}
	streamer, err := events.NewCheckingStreamer(events.CheckingStreamerConfig{
		Inner:       conn.Client,
		Clock:       process.Clock,
		ClusterName: clusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	streamEmitter := &events.StreamerAndEmitter{
		Emitter:  asyncEmitter,
		Streamer: streamer,
	}

	discoveryService, err := discovery.New(process.ExitContext(), &discovery.Config{
		Clients:     cloud.NewClients(),
		Matchers:    process.Config.Discovery.AWSMatchers,
		NodeWatcher: nodeWatcher,
		Emitter:     streamEmitter,
	})

	if err != nil {
		return trace.Wrap(err)
	}

	process.OnExit("discovery.stop", func(payload interface{}) {
		log.Info("Shutting down.")
		if discoveryService != nil {
			discoveryService.Stop()
		}
		if asyncEmitter != nil {
			warnOnErr(asyncEmitter.Close(), process.log)
		}
		warnOnErr(conn.Close(), log)
		log.Info("Exited.")
	})

	process.BroadcastEvent(Event{Name: DiscoveryReady, Payload: nil})
	log.Infof("Discovery service has successfully started")

	if err := discoveryService.Start(); err != nil {
		return trace.Wrap(err)
	}

	if err := discoveryService.Wait(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
