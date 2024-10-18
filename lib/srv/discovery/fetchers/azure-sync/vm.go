package azure_sync

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	"github.com/gravitational/trace"
)

func (a *azureFetcher) pollVirtualMachines(ctx context.Context) ([]*armcompute.VirtualMachine, error) {
	cli, err := a.CloudClients.GetAzureVirtualMachinesClient(a.GetSubscriptionID())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	vms, err := cli.ListVirtualMachines(ctx, "*")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return vms, nil
}
