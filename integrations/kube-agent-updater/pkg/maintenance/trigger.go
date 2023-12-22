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

	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// Trigger is evaluated to decide whether a maintenance can happen or not.
// Maintenances can happen because of multiple reasons like:
// - attempt to recover from a broken state
// - we are in a maintenance window
// - emergency security patch
// Each Trigger has a Name() for logging purposes and a Default() method
// returning whether the trigger should allow the maintenance or not in case\
// of error.
type Trigger interface {
	Name() string
	CanStart(ctx context.Context, object client.Object) (bool, error)
	Default() bool
}

// Triggers is a list of Trigger. Triggers are OR-ed: any trigger firing in the
// list will cause the maintenance to be triggered.
type Triggers []Trigger

// CanStart checks if the maintenance can be started. It will return true if at
// least a Trigger approves the maintenance.
func (t Triggers) CanStart(ctx context.Context, object client.Object) bool {
	log := ctrllog.FromContext(ctx).V(1)
	for _, trigger := range t {
		start, err := trigger.CanStart(ctx, object)
		if err != nil {
			start = trigger.Default()
			log.Error(err, "trigger failed to evaluate, using its default value", "trigger", trigger.Name(), "defaultValue", start)
		} else {
			log.Info("trigger evaluated", "trigger", trigger.Name(), "result", start)
		}
		if start {
			log.Info("maintenance triggered", "trigger", trigger.Name())
			return true
		}
	}
	return false
}
