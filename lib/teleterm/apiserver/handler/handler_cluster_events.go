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

package handler

import (
	"time"

	api "github.com/gravitational/teleport/lib/teleterm/api/protogen/golang/v1"

	"github.com/gravitational/trace"
)

func (s *Handler) ClusterEvents(_ *api.ClusterEventsRequest, stream api.TerminalService_ClusterEventsServer) error {
	// TODO: Move the logic from this handler to a separate place.
	log := s.DaemonService.Log().WithField(trace.Component, "conn:cevents")

	select {
	case <-time.After(time.Second):
		message := "Couldn't acquire lock within a second, another stream is already active."
		log.Info(message)
		return trace.AlreadyExists(message)
	case <-stream.Context().Done():
		log.Info("The client has disconnected while waiting for the lock, closing the stream.")
		return nil
	case s.DaemonService.ClusterEventsSem <- struct{}{}:
		defer func() { <-s.DaemonService.ClusterEventsSem }()
	}

	log.Info("Opened the stream.")

	for {
		select {
		case <-stream.Context().Done():
			log.Info("The client has disconnected, closing the stream.")
			return nil
		case clusterEvent, ok := <-s.DaemonService.ClusterEventsC():
			if !ok {
				log.Info("The ClusterEvents channel has been closed, closing the stream.")
				return nil
			}

			log.Debugf("Sending a message: %v", clusterEvent)

			if err := stream.Send(clusterEvent); err != nil {
				log.WithError(err).Error("Failed to send a message, closing the stream.")
				return err
			}
		}
	}
}
