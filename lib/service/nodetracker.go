/*
Copyright 2021 Gravitational, Inc.

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
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/nodetracker"

	nodetrackerimpl "github.com/gravitational/teleport/lib/nodetracker/implementation"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// initNodeTracker gets called if teleport runs with 'node_tracker_service'
// enabled.
// It starts the node tracker service responsible of tracking node to proxy
// relationships in IOT mode
func (process *TeleportProcess) initNodeTracker() {
	process.RegisterCriticalFunc("nodetracker.init", process.initNodeTrackerService)
}

func (process *TeleportProcess) initNodeTrackerService() error {
	log := process.log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentNodeTracker, process.id),
	})

	if !modules.GetModules().Features().NodeTracker {
		log.Info("this Teleport cluster is not licensed for node tracker service access, please contact the cluster administrator")
		return nil
	}

	listener, err := process.importOrCreateListener(listenerNodeTracker, process.Config.NodeTracker.ListenAddr.Addr)
	if err != nil {
		return trace.Wrap(err)
	}
	warnOnErr(process.closeImportedDescriptors(teleport.ComponentMetrics), log)

	nodetrackerimpl.NewServer(listener, process.Config.NodeTracker.ProxyKeepAliveInterval.Duration()) // this should ultimately be initialized somewhere else

	process.RegisterFunc("nodetracker.service", func() error {
		log.Infof("Starting node tracker service on %v.", listener.Addr().String())
		if err := nodetracker.GetServer().Start(); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})

	process.OnExit("nodetracker.shutdown", func(payload interface{}) {
		log.Infof("Shutting down gracefully.")
		nodetracker.GetServer().Stop()
		if listener != nil {
			listener.Close()
		}
		log.Infof("Exited.")
	})

	return nil
}
