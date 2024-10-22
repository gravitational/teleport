package azure_sync

import (
	"fmt"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"google.golang.org/protobuf/proto"
)

func MergeResources(results ...*Resources) *Resources {
	if len(results) == 0 {
		return &Resources{}
	}
	if len(results) == 1 {
		return results[0]
	}
	result := &Resources{}
	for _, r := range results {
		result.Users = append(result.Users, r.Users...)
		result.VirtualMachines = append(result.VirtualMachines, r.VirtualMachines...)
	}
	result.Users = common.DeduplicateSlice(result.Users, azureUserKey)
	result.VirtualMachines = common.DeduplicateSlice(result.VirtualMachines, azureVmKey)
	return result
}

func newResourceList() *accessgraphv1alpha.AzureResourceList {
	return &accessgraphv1alpha.AzureResourceList{
		Resources: make([]*accessgraphv1alpha.AzureResource, 0),
	}
}

func ReconcileResults(old *Resources, new *Resources) (upsert, delete *accessgraphv1alpha.AzureResourceList) {
	upsert, delete = newResourceList(), newResourceList()
	reconciledResources := []*reconcilePair{
		reconcile(old.VirtualMachines, new.VirtualMachines, azureVmKey, azureVmWrap),
		reconcile(old.Users, new.Users, azureUserKey, azureUsersWrap),
	}
	for _, res := range reconciledResources {
		upsert.Resources = append(upsert.Resources, res.upsert.Resources...)
		delete.Resources = append(delete.Resources, res.delete.Resources...)
	}
	return upsert, delete
}

type reconcilePair struct {
	upsert, delete *accessgraphv1alpha.AzureResourceList
}

func reconcile[T proto.Message](
	oldItems []T,
	newItems []T,
	keyFn func(T) string,
	wrapFn func(T) *accessgraphv1alpha.AzureResource,
) *reconcilePair {
	// Remove duplicates from the new items
	newItems = common.DeduplicateSlice(newItems, keyFn)
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

func azureVmKey(vm *accessgraphv1alpha.AzureVirtualMachine) string {
	return fmt.Sprintf("%s:%s", vm.SubscriptionId, vm.Id)
}

func azureVmWrap(vm *accessgraphv1alpha.AzureVirtualMachine) *accessgraphv1alpha.AzureResource {
	return &accessgraphv1alpha.AzureResource{Resource: &accessgraphv1alpha.AzureResource_VirtualMachine{VirtualMachine: vm}}
}

func azureUserKey(user *accessgraphv1alpha.AzureUser) string {
	return fmt.Sprintf("%s:%s", user.SubscriptionId, user.Id)
}

func azureUsersWrap(user *accessgraphv1alpha.AzureUser) *accessgraphv1alpha.AzureResource {
	return &accessgraphv1alpha.AzureResource{Resource: &accessgraphv1alpha.AzureResource_User{User: user}}
}
