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
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

type mockRunCommandClient struct{}

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

type blockingFakeRunCommandClient struct {
	started     chan struct{}
	unblockOnce sync.Once
	unblockCh   chan struct{}

	mu      sync.Mutex
	blocked map[string]struct{}
}

func newBlockingRunCommandClient(instanceCount int) *blockingFakeRunCommandClient {
	return &blockingFakeRunCommandClient{
		started:   make(chan struct{}, instanceCount),
		unblockCh: make(chan struct{}),
		blocked:   make(map[string]struct{}),
	}
}

func (m *blockingFakeRunCommandClient) Run(ctx context.Context, req azure.RunCommandRequest) (*azure.RunCommandResult, error) {
	m.mu.Lock()
	m.blocked[req.VMName] = struct{}{}
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.blocked, req.VMName)
		m.mu.Unlock()
	}()

	select {
	case m.started <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// block so other installs can enter Run. If the installer becomes
	// sequential, blockUntil will time out waiting for the later starts instead
	// of observing the full instance set.
	select {
	case <-m.unblockCh:
		return &azure.RunCommandResult{
			ExecutionState: string(armcompute.ExecutionStateSucceeded),
			ExitCode:       0,
			StdOut:         "Mock stdout",
			StdErr:         "Mock stderr",
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *blockingFakeRunCommandClient) blockUntil(count int) []string {
	for len(m.blockedVMs()) < count {
		select {
		case <-m.started:
		case <-time.After(5 * time.Second):
			return m.blockedVMs()
		}
	}
	return m.blockedVMs()
}

func (m *blockingFakeRunCommandClient) unblock() {
	m.unblockOnce.Do(func() {
		close(m.unblockCh)
	})
}

func (m *blockingFakeRunCommandClient) blockedVMs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return slices.Collect(maps.Keys(m.blocked))
}

func newFakeSemaphore(maxLeases int) *fakeSemaphore {
	return &fakeSemaphore{
		sem:    make(chan struct{}, maxLeases),
		closed: make(chan struct{}),
	}
}

type fakeSemaphore struct {
	sem    chan struct{}
	closed chan struct{}
}

func (f *fakeSemaphore) acquire(ctx context.Context) (func(), error) {
	select {
	case f.sem <- struct{}{}:
	case <-f.closed:
		return nil, errors.New("fake semaphore closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return f.release, nil
}

func (f *fakeSemaphore) release() {
	<-f.sem
}

func (f *fakeSemaphore) getActive() int {
	return len(f.sem)
}

func (f *fakeSemaphore) close() {
	close(f.closed)
}

func TestAzureInstallRequestRun(t *testing.T) {
	makeVM := func(name string) *azure.VirtualMachine {
		return &azure.VirtualMachine{
			ID:   name,
			Name: name,
		}
	}

	makeVMs := func(names ...string) []*azure.VirtualMachine {
		var vms []*azure.VirtualMachine
		for _, name := range names {
			vms = append(vms, makeVM(name))
		}
		return vms
	}

	t.Parallel()

	client := &mockRunCommandClient{}

	tests := []struct {
		name            string
		instances       []*azure.VirtualMachine
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
						failed = append(failed, result.Instance.ID)
					} else {
						good = append(good, result.Instance.ID)
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

	runAzureInstallRequest := func(t *testing.T, req *AzureInstallRequest, client azure.RunCommandClient) <-chan error {
		t.Helper()
		errCh := make(chan error, 1)
		go func() { errCh <- req.Run(t.Context(), client) }()
		return errCh
	}
	makeTestAzureInstallRequest := func(instances []*azure.VirtualMachine) *AzureInstallRequest {
		return &AzureInstallRequest{
			Instances: instances,
			InstallerParams: &types.InstallerParams{
				JoinMethod: types.JoinMethodAzure,
				JoinToken:  "test-token",
			},
			ProxyAddrGetter: func(ctx context.Context) (string, error) {
				return "proxy.example.com:443", nil
			},
			Region:        "eastus",
			ResourceGroup: "test-rg",
			AcquireLease: func(context.Context) (func(), error) {
				return func() {}, nil
			},
		}
	}

	t.Run("runs installations in parallel", func(t *testing.T) {
		t.Parallel()

		instances := makeVMs("vm-1", "vm-2", "vm-3")
		client := newBlockingRunCommandClient(len(instances))
		defer client.unblock()
		req := makeTestAzureInstallRequest(instances)

		runErrCh := runAzureInstallRequest(t, req, client)
		require.ElementsMatch(t, []string{"vm-1", "vm-2", "vm-3"}, client.blockUntil(len(instances)))

		client.unblock()
		require.NoError(t, <-runErrCh)
	})

	t.Run("acquires and releases lease", func(t *testing.T) {
		t.Parallel()

		instances := makeVMs("vm-1", "vm-2", "vm-3")
		client := newBlockingRunCommandClient(len(instances))
		defer client.unblock()
		leases := newFakeSemaphore(3)
		req := makeTestAzureInstallRequest(instances)
		req.AcquireLease = leases.acquire

		runErrCh := runAzureInstallRequest(t, req, client)

		require.ElementsMatch(t, []string{"vm-1", "vm-2", "vm-3"}, client.blockUntil(len(instances)))
		require.Equal(t, len(instances), leases.getActive())
		client.unblock()
		require.NoError(t, <-runErrCh)
		require.Zero(t, leases.getActive())
	})

	t.Run("returns acquire lease error", func(t *testing.T) {
		t.Parallel()

		instances := makeVMs("vm-1", "vm-2")
		client := newBlockingRunCommandClient(len(instances))
		defer client.unblock()
		leases := newFakeSemaphore(1)
		req := makeTestAzureInstallRequest(instances)
		req.AcquireLease = leases.acquire

		runErrCh := runAzureInstallRequest(t, req, client)
		require.ElementsMatch(t, []string{"vm-1"}, client.blockUntil(1))
		require.Equal(t, 1, leases.getActive())
		leases.close()
		require.ErrorContains(t, <-runErrCh, "fake semaphore closed")
		require.Empty(t, client.blockedVMs())
		require.Zero(t, leases.getActive())
	})
}
