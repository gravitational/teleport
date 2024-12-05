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
)

// StaticTrigger is a fake Trigger that return a static answer. This is used
// for testing purposes and is inherently disruptive.
type StaticTrigger struct {
	name     string
	canStart bool
	err      error
}

// Name returns the StaticTrigger name.
func (m StaticTrigger) Name() string {
	return m.name
}

// CanStart returns the statically defined maintenance approval result.
func (m StaticTrigger) CanStart(_ context.Context, _ client.Object) (bool, error) {
	return m.canStart, m.err
}

// Default returns the default behavior if the trigger fails. This cannot
// happen for a StaticTrigger and is here solely to implement the Trigger
// interface.
func (m StaticTrigger) Default() bool {
	return m.canStart
}

// NewMaintenanceStaticTrigger creates a StaticTrigger
func NewMaintenanceStaticTrigger(name string, canStart bool) Trigger {
	return StaticTrigger{
		name:     name,
		canStart: canStart,
	}
}
