package azure_sync

import (
	"fmt"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func azureVmKey(vm *accessgraphv1alpha.AzureVirtualMachine) string {
	return fmt.Sprintf("%s:%s", vm.SubscriptionId, vm.Id)
}

func azureUserKey(user *accessgraphv1alpha.AzureUser) string {
	return fmt.Sprintf("%s:%s", user.SubscriptionId, user.Id)
}
