/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package azure

// DiscoveredVM describes an Azure virtual machine returned by discovery.
type DiscoveredVM struct {
	// ID is the ARM resource ID, e.g. "/subscriptions/.../virtualMachines/foo".
	ID string
	// SubscriptionID is the Azure subscription containing the VM.
	SubscriptionID string
	// Name is the VM's display name.
	Name string
	// VMID is Azure's unique identifier for the VM.
	VMID string
	// Location is the Azure region containing the VM.
	Location string
	// ResourceGroup is the Azure resource group containing the VM.
	ResourceGroup string
	// Tags are the VM tags. Empty map (not nil) when the VM has no tags.
	Tags map[string]string
}
