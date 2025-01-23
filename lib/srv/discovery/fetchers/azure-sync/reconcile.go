/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package azuresync

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/utils/slices"
)

// MergeResources merges Azure resources fetched from multiple configured Azure fetchers
func MergeResources(results ...*Resources) *Resources {
	if len(results) == 0 {
		return &Resources{}
	}
	if len(results) == 1 {
		return results[0]
	}
	result := &Resources{}
	for _, r := range results {
		result.Principals = append(result.Principals, r.Principals...)
		result.RoleAssignments = append(result.RoleAssignments, r.RoleAssignments...)
		result.RoleDefinitions = append(result.RoleDefinitions, r.RoleDefinitions...)
		result.VirtualMachines = append(result.VirtualMachines, r.VirtualMachines...)
	}
	result.Principals = slices.DeduplicateKey(result.Principals, azurePrincipalsKey)
	result.RoleAssignments = slices.DeduplicateKey(result.RoleAssignments, azureRoleAssignKey)
	result.RoleDefinitions = slices.DeduplicateKey(result.RoleDefinitions, azureRoleDefKey)
	result.VirtualMachines = slices.DeduplicateKey(result.VirtualMachines, azureVmKey)
	return result
}

// newResourceList creates a new resource list message
func newResourceList() *accessgraphv1alpha.AzureResourceList {
	return &accessgraphv1alpha.AzureResourceList{
		Resources: make([]*accessgraphv1alpha.AzureResource, 0),
	}
}

// ReconcileResults compares previously and currently fetched results and determines which resources to upsert and
// which to delete.
func ReconcileResults(old *Resources, new *Resources) (upsert, delete *accessgraphv1alpha.AzureResourceList) {
	upsert, delete = newResourceList(), newResourceList()
	reconciledResources := []*reconcilePair{
		reconcile(old.Principals, new.Principals, azurePrincipalsKey, azurePrincipalsWrap),
		reconcile(old.RoleAssignments, new.RoleAssignments, azureRoleAssignKey, azureRoleAssignWrap),
		reconcile(old.RoleDefinitions, new.RoleDefinitions, azureRoleDefKey, azureRoleDefWrap),
		reconcile(old.VirtualMachines, new.VirtualMachines, azureVmKey, azureVmWrap),
	}
	for _, res := range reconciledResources {
		upsert.Resources = append(upsert.Resources, res.upsert.Resources...)
		delete.Resources = append(delete.Resources, res.delete.Resources...)
	}
	return upsert, delete
}

// reconcilePair contains the Azure resources to upsert and delete
type reconcilePair struct {
	upsert, delete *accessgraphv1alpha.AzureResourceList
}

// reconcile compares old and new items to build a list of resources to upsert and delete in the Access Graph
func reconcile[T proto.Message](
	oldItems []T,
	newItems []T,
	keyFn func(T) string,
	wrapFn func(T) *accessgraphv1alpha.AzureResource,
) *reconcilePair {
	// Remove duplicates from the new items
	newItems = slices.DeduplicateKey(newItems, keyFn)
	upsertRes := newResourceList()
	deleteRes := newResourceList()

	// Delete all old items if there are no new items
	if len(newItems) == 0 {
		for _, item := range oldItems {
			deleteRes.Resources = append(deleteRes.Resources, wrapFn(item))
		}
		return &reconcilePair{upsertRes, deleteRes}
	}

	// Create all new items if there are no old items
	if len(oldItems) == 0 {
		for _, item := range newItems {
			upsertRes.Resources = append(upsertRes.Resources, wrapFn(item))
		}
		return &reconcilePair{upsertRes, deleteRes}
	}

	// Map old and new items by their key
	oldMap := make(map[string]T, len(oldItems))
	for _, item := range oldItems {
		oldMap[keyFn(item)] = item
	}
	newMap := make(map[string]T, len(newItems))
	for _, item := range newItems {
		newMap[keyFn(item)] = item
	}

	// Append new or modified items to the upsert list
	for _, item := range newItems {
		if oldItem, ok := oldMap[keyFn(item)]; !ok || !proto.Equal(oldItem, item) {
			upsertRes.Resources = append(upsertRes.Resources, wrapFn(item))
		}
	}

	// Append removed items to the delete list
	for _, item := range oldItems {
		if _, ok := newMap[keyFn(item)]; !ok {
			deleteRes.Resources = append(deleteRes.Resources, wrapFn(item))
		}
	}
	return &reconcilePair{upsertRes, deleteRes}
}

func azurePrincipalsKey(user *accessgraphv1alpha.AzurePrincipal) string {
	return fmt.Sprintf("%s:%s", user.SubscriptionId, user.Id)
}

func azurePrincipalsWrap(principal *accessgraphv1alpha.AzurePrincipal) *accessgraphv1alpha.AzureResource {
	return &accessgraphv1alpha.AzureResource{Resource: &accessgraphv1alpha.AzureResource_Principal{Principal: principal}}
}

func azureRoleAssignKey(roleAssign *accessgraphv1alpha.AzureRoleAssignment) string {
	return fmt.Sprintf("%s:%s", roleAssign.SubscriptionId, roleAssign.Id)
}

func azureRoleAssignWrap(roleAssign *accessgraphv1alpha.AzureRoleAssignment) *accessgraphv1alpha.AzureResource {
	return &accessgraphv1alpha.AzureResource{Resource: &accessgraphv1alpha.AzureResource_RoleAssignment{RoleAssignment: roleAssign}}
}

func azureRoleDefKey(roleDef *accessgraphv1alpha.AzureRoleDefinition) string {
	return fmt.Sprintf("%s:%s", roleDef.SubscriptionId, roleDef.Id)
}

func azureRoleDefWrap(roleDef *accessgraphv1alpha.AzureRoleDefinition) *accessgraphv1alpha.AzureResource {
	return &accessgraphv1alpha.AzureResource{Resource: &accessgraphv1alpha.AzureResource_RoleDefinition{RoleDefinition: roleDef}}
}

func azureVmKey(vm *accessgraphv1alpha.AzureVirtualMachine) string {
	return fmt.Sprintf("%s:%s", vm.SubscriptionId, vm.Id)
}

func azureVmWrap(vm *accessgraphv1alpha.AzureVirtualMachine) *accessgraphv1alpha.AzureResource {
	return &accessgraphv1alpha.AzureResource{Resource: &accessgraphv1alpha.AzureResource_VirtualMachine{VirtualMachine: vm}}
}
