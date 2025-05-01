/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package helpers

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"

	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func nullImpersonationCheck(context.Context, string, authztypes.SelfSubjectAccessReviewInterface) error {
	return nil
}

func StartAndWait(process *service.TeleportProcess, expectedEvents []string) ([]service.Event, error) {
	// start the process
	err := process.Start()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// wait for all events to arrive or a timeout. if all the expected events
	// from above are not received, this instance will not start
	receivedEvents := make([]service.Event, 0, len(expectedEvents))
	ctx, cancel := context.WithTimeout(process.ExitContext(), 30*time.Second)
	defer cancel()
	for _, eventName := range expectedEvents {
		if event, err := process.WaitForEvent(ctx, eventName); err == nil {
			receivedEvents = append(receivedEvents, event)
		}
	}

	if len(receivedEvents) < len(expectedEvents) {
		return nil, trace.BadParameter("timed out, only %v/%v events received. received: %v, expected: %v",
			len(receivedEvents), len(expectedEvents), receivedEvents, expectedEvents)
	}

	// Not all services follow a non-blocking Start/Wait pattern. This means a
	// *Ready event may be emit slightly before the service actually starts for
	// blocking services. Long term those services should be re-factored, until
	// then sleep for 250ms to handle this situation.
	time.Sleep(250 * time.Millisecond)

	return receivedEvents, nil
}

func EnableDesktopService(config *servicecfg.Config) {
	config.WindowsDesktop.Enabled = true
	config.WindowsDesktop.ListenAddr = *utils.MustParseAddr("127.0.0.1:0")
}
