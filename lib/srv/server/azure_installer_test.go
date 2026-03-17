// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package server

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

type mockRunCommandClient struct {
	runFunc func(ctx context.Context, req azure.RunCommandRequest) error
}

func (m *mockRunCommandClient) Run(ctx context.Context, req azure.RunCommandRequest) error {
	if m.runFunc != nil {
		return m.runFunc(ctx, req)
	}
	return nil
}

func TestAzureInstallRequestRun(t *testing.T) {
	makeVMs := func(names ...string) []*armcompute.VirtualMachine {
		vms := make([]*armcompute.VirtualMachine, len(names))
		for i, name := range names {
			vms[i] = &armcompute.VirtualMachine{
				ID:   &name,
				Name: &name,
			}
		}
		return vms
	}
	t.Parallel()

	tests := []struct {
		name            string
		instances       []*armcompute.VirtualMachine
		runFunc         func(ctx context.Context, req azure.RunCommandRequest) error
		proxyAddrGetter func(context.Context) (string, error)
		wantErr         string
		wantFailedVMs   []string
	}{
		{
			name:      "no instances",
			instances: nil,
		},
		{
			name:      "single instance success",
			instances: makeVMs("vm-1"),
		},
		{
			name:      "single instance failure",
			instances: makeVMs("vm-1"),
			runFunc: func(ctx context.Context, req azure.RunCommandRequest) error {
				return errors.New("install failed")
			},
			wantFailedVMs: []string{"vm-1"},
		},
		{
			name:      "multiple instances all success",
			instances: makeVMs("vm-1", "vm-2", "vm-3"),
		},
		{
			name:      "multiple instances some failures",
			instances: makeVMs("vm-1", "vm-2", "vm-3"),
			runFunc: func(ctx context.Context, req azure.RunCommandRequest) error {
				if req.VMName == "vm-2" {
					return errors.New("install failed")
				}
				return nil
			},
			wantFailedVMs: []string{"vm-2"},
		},
		{
			name:      "multiple instances all failures",
			instances: makeVMs("vm-1", "vm-2", "vm-3"),
			runFunc: func(ctx context.Context, req azure.RunCommandRequest) error {
				return errors.New("install failed")
			},
			wantFailedVMs: []string{"vm-1", "vm-2", "vm-3"},
		},
		{
			name:      "proxy addr getter error",
			instances: makeVMs("vm-1"),
			proxyAddrGetter: func(ctx context.Context) (string, error) {
				return "", errors.New("proxy lookup failed")
			},
			wantErr: "proxy lookup failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockRunCommandClient{runFunc: tt.runFunc}
			proxyAddrGetter := tt.proxyAddrGetter
			if proxyAddrGetter == nil {
				proxyAddrGetter = func(ctx context.Context) (string, error) {
					return "proxy.example.com:443", nil
				}
			}

			req := &AzureInstallRequest{
				Instances: tt.instances,
				InstallerParams: &types.InstallerParams{
					JoinMethod: types.JoinMethodAzure,
					JoinToken:  "test-token",
				},
				ProxyAddrGetter: proxyAddrGetter,
				Region:          "eastus",
				ResourceGroup:   "test-rg",
			}

			failures, err := req.Run(t.Context(), client)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			var failedNames []string
			for _, vm := range failures {
				failedNames = append(failedNames, *vm.Instance.Name)
			}
			slices.Sort(failedNames)

			require.Equal(t, tt.wantFailedVMs, failedNames)
		})
	}
}
