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

package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	v1 "k8s.io/api/core/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gravitational/teleport/lib/automaticupgrades/maintenance"
)

const maintenanceScheduleKeyName = "agent-maintenance-schedule"

// windowTrigger allows a maintenance to start if we are in a planned
// maintenance window. Maintenance windows are discovered by the agent and
// written to a secret (shared for all the agents). If the secret is stale or
// missing the trigger will assume the agent is not working properly and allow
// maintenance.
type windowTrigger struct {
	name string
	kclient.Client
	clock clockwork.Clock
}

// Name returns the trigger name.
func (w windowTrigger) Name() string {
	return w.name
}

// CanStart implements maintenance.Trigger and checks if we are in a
// maintenance window.
func (w windowTrigger) CanStart(ctx context.Context, object kclient.Object) (bool, error) {
	log := ctrllog.FromContext(ctx).V(1)
	secretName := fmt.Sprintf("%s-shared-state", object.GetName())
	var secret v1.Secret
	err := w.Get(ctx, kclient.ObjectKey{Namespace: object.GetNamespace(), Name: secretName}, &secret)
	if err != nil {
		return false, trace.Wrap(err)
	}
	rawData, ok := secret.Data[maintenanceScheduleKeyName]
	if !ok {
		return false, trace.Errorf("secret %s does not have key %s", secretName, maintenanceScheduleKeyName)
	}
	var maintenanceSchedule kubeScheduleRepr
	err = json.Unmarshal(rawData, &maintenanceSchedule)
	if err != nil {
		return false, trace.WrapWithMessage(err, "failed to unmarshall schedule")
	}
	now := w.clock.Now()
	if !maintenanceSchedule.isValid(now) {
		return false, trace.Errorf("maintenance schedule is stale or invalid")
	}
	for _, window := range maintenanceSchedule.Windows {
		if window.inWindow(now) {
			log.Info("maintenance window active", "start", window.Start, "end", window.Stop)
			return true, nil
		}
	}
	return false, nil
}

// Default defines what to do in case of failure. The windowTrigger should
// trigger a maintenance if it fails to evaluate the next maintenance windows.
// Not having a sane and up-to-date secret means the agent might not work as
// intended.
func (w windowTrigger) Default() bool {
	return true
}

// kubeSchedulerRepr is the structure containing the maintenance schedule
// sent by the agent through a Kubernetes secret.
type kubeScheduleRepr struct {
	Windows []windowRepr `json:"windows"`
}

// isValid checks if the schedule is valid. A schedule is considered invalid if
// it has no upcoming or ongoing maintenance window, or if it contains a window
// whose start is after its end. This could happen if the agent looses
// connectivity to its cluster or if we have a bug in the window calculation.
// In this case we don't want to honor the schedule and will consider the
// agent is not working properly.
func (s kubeScheduleRepr) isValid(now time.Time) bool {
	valid := false
	for _, window := range s.Windows {
		if window.Start.After(window.Stop) {
			return false
		}
		if window.Stop.After(now) {
			valid = true
		}
	}
	return valid
}

type windowRepr struct {
	Start time.Time `json:"start"`
	Stop  time.Time `json:"stop"`
}

// inWindow checks if a given time is in the window.
func (w windowRepr) inWindow(now time.Time) bool {
	return now.After(w.Start) && now.Before(w.Stop)
}

// NewWindowTrigger returns a new Trigger validating if the agent is within its
// maintenance window.
func NewWindowTrigger(name string, client kclient.Client) maintenance.Trigger {
	return windowTrigger{
		name:   name,
		Client: client,
		clock:  clockwork.NewRealClock(),
	}
}
