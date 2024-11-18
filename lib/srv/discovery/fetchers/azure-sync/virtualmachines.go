/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package azure_sync

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func (a *Fetcher) fetchVirtualMachines(ctx context.Context) ([]*accessgraphv1alpha.AzureVirtualMachine, error) {
	// Fetch the VMs
	vms, err := a.vmClient.ListVirtualMachines(ctx, "*")
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
