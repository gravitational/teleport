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

// TriggerMock is a fake Trigger that return a static answer. This is used
// for testing purposes and is inherently disruptive.
type TriggerMock struct {
	name     string
	canStart bool
}

// Name returns the TriggerMock name.
func (m TriggerMock) Name() string {
	return m.name
}

// CanStart returns the statically defined maintenance approval result.
func (m TriggerMock) CanStart(_ context.Context, _ client.Object) (bool, error) {
	return m.canStart, nil
}

// Default returns the default behavior if the trigger fails. This cannot
// happen for a TriggerMock and is here solely to implement the Trigger
// interface.
func (m TriggerMock) Default() bool {
	return m.canStart
}

// NewMaintenanceTriggerMock creates a TriggerMock
func NewMaintenanceTriggerMock(name string, canStart bool) Trigger {
	return TriggerMock{
		name:     name,
		canStart: canStart,
	}
}
