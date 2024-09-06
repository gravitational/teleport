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

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status defines the observed state of the Teleport resource
type Status struct {
	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions"`
	// +optional
	TeleportResourceID int64 `json:"teleportResourceID"`
}

// DeepCopyInto deep-copies one resource status into another.
// Required to satisfy runtime.Object interface.
func (status *Status) DeepCopyInto(out *Status) {
	*out = Status{}
	out.Conditions = make([]metav1.Condition, len(status.Conditions))
	copy(out.Conditions, status.Conditions)
	out.TeleportResourceID = status.TeleportResourceID
}
