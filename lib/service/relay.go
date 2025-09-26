// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package service

import (
	"context"
	"sync/atomic"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv"
)

func (process *TeleportProcess) initRelay() {
	process.RegisterWithAuthServer(apitypes.RoleRelay, RelayIdentityEvent)
	process.RegisterCriticalFunc("relay.run", process.runRelayService)
}

func (process *TeleportProcess) runRelayService() error {
	log := process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentRelay, process.id))

	defer func() {
		if err := process.closeImportedDescriptors(teleport.ComponentRelay); err != nil {
			log.WarnContext(process.ExitContext(), "Failed closing imported file descriptors.", "error", err)
		}
	}()

	conn, err := process.WaitForConnector(RelayIdentityEvent, log)
	if conn == nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	var relayServer atomic.Pointer[presencev1.RelayServer]
	relayServer.Store(&presencev1.RelayServer{
		Kind:    apitypes.KindRelayServer,
		SubKind: "",
		Version: apitypes.V1,
		Metadata: &headerv1.Metadata{
			Name: process.Config.HostUUID,
		},
		Spec: &presencev1.RelayServer_Spec{},
	})

	hb, err := srv.NewRelayServerHeartbeat(srv.HeartbeatV2Config[*presencev1.RelayServer]{
		InventoryHandle: process.inventoryHandle,
		GetResource: func(context.Context) (*presencev1.RelayServer, error) {
			return relayServer.Load(), nil
		},

		// there's no fallback announce mode, the relay service only works with
		// clusters recent enough to support relay heartbeats through the ICS
		Announcer: nil,

		OnHeartbeat: process.OnHeartbeat(teleport.ComponentRelay),
	}, log)
	if err != nil {
		return trace.Wrap(err)
	}
	go hb.Run()
	defer hb.Close()

	process.BroadcastEvent(Event{Name: RelayReady})
	log.InfoContext(process.ExitContext(), "The relay service has successfully started.")

	exitEvent, _ := process.WaitForEvent(process.ExitContext(), TeleportExitEvent)
	ctx, _ := exitEvent.Payload.(context.Context)
	if ctx == nil {
		// if we're here it's because we got an ungraceful exit event or
		// WaitForEvent errored out because of the ungraceful shutdown; either
		// way, process.ExitContext() is a done context and all operations
		// should get canceled immediately
		log.InfoContext(ctx, "Stopping the relay service ungracefully.")
		ctx = process.ExitContext()
	} else {
		log.InfoContext(ctx, "Stopping the relay service.")
	}

	warnOnErr(ctx, conn.Close(), log)

	log.InfoContext(ctx, "The relay service has stopped.")

	return nil
}
