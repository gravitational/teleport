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
	"strings"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

type mockRunCommandClient struct {
}

func (m *mockRunCommandClient) Run(ctx context.Context, req azure.RunCommandRequest) (*azure.RunCommandResult, error) {
	if strings.HasPrefix(req.VMName, "bad") {
		return nil, trace.BadParameter("VM is bad: %v", req.VMName)
	}

	return &azure.RunCommandResult{
		ExecutionState: string(armcompute.ExecutionStateSucceeded),
		ExitCode:       0,
		StdOut:         "Mock stdout",
		StdErr:         "Mock stderr",
	}, nil
}

func TestAzureInstallRequestRun(t *testing.T) {
	makeVM := func(name string) *armcompute.VirtualMachine {
		return &armcompute.VirtualMachine{
			ID:   &name,
			Name: &name,
		}
	}

	makeVMs := func(names ...string) []*armcompute.VirtualMachine {
		var vms []*armcompute.VirtualMachine
		for _, name := range names {
			vms = append(vms, makeVM(name))
		}
		return vms
	}

	t.Parallel()

	client := &mockRunCommandClient{}

	tests := []struct {
		name            string
		instances       []*armcompute.VirtualMachine
		proxyAddrGetter func(context.Context) (string, error)

		wantErr string

		wantOK     []string
		wantFailed []string
	}{
		{
			name:      "success",
			instances: makeVMs("good-1", "good-2", "good-3"),
			wantOK:    []string{"good-1", "good-2", "good-3"},
		},
		{
			name:       "mixed results",
			instances:  makeVMs("good-1", "bad-2", "good-3", "bad-4"),
			wantOK:     []string{"good-1", "good-3"},
			wantFailed: []string{"bad-2", "bad-4"},
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

			proxyAddrGetter := tt.proxyAddrGetter
			if proxyAddrGetter == nil {
				proxyAddrGetter = func(ctx context.Context) (string, error) {
					return "proxy.example.com:443", nil
				}
			}

			var mu sync.Mutex
			var failed []string
			var good []string

			req := &AzureInstallRequest{
				Instances: tt.instances,
				InstallerParams: &types.InstallerParams{
					JoinMethod: types.JoinMethodAzure,
					JoinToken:  "test-token",
				},
				ProxyAddrGetter: proxyAddrGetter,
				Region:          "eastus",
				ResourceGroup:   "test-rg",
				OnRunCommandFinished: func(result AzureInstallResult) {
					mu.Lock()
					defer mu.Unlock()
					if result.Failure() {
						failed = append(failed, *result.Instance.ID)
					} else {
						good = append(good, *result.Instance.ID)
					}
				},
			}

			err := req.Run(t.Context(), client)

			slices.Sort(failed)
			slices.Sort(good)

			require.Equal(t, tt.wantFailed, failed)
			require.Equal(t, tt.wantOK, good)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
