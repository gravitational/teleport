package azure_sync

import (
	"context"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (a *azureFetcher) fetchVirtualMachines(ctx context.Context) ([]*accessgraphv1alpha.AzureVirtualMachine, error) {
	// Get the VM client
	cli, err := a.CloudClients.GetAzureVirtualMachinesClient(a.GetSubscriptionID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch the VMs
	vms, err := cli.ListVirtualMachines(ctx, "*")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return the VMs as protobuf messages
	pbVms := make([]*accessgraphv1alpha.AzureVirtualMachine, 0, len(vms))
	for _, vm := range vms {
		pbVm := accessgraphv1alpha.AzureVirtualMachine{
			Id:             *vm.ID,
			SubscriptionId: a.GetSubscriptionID(),
			LastSyncTime:   timestamppb.Now(),
			Name:           *vm.Name,
		}
		pbVms = append(pbVms, &pbVm)
	}
	return pbVms, nil
}
