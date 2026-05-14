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
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/azure"
)

type mockRunCommandClient struct {
}

func (m *mockRunCommandClient) RunAsync(ctx context.Context, req azure.RunCommandRequest) (*fakeRunCommandResolver, error) {
	if strings.HasPrefix(req.VMName, "bad") {
		return nil, trace.BadParameter("VM is bad: %v", req.VMName)
	}

	return &fakeRunCommandResolver{
		resolveResult: &azure.RunCommandResult{
			ExecutionState: string(armcompute.ExecutionStateSucceeded),
			ExitCode:       0,
			StdOut:         "Mock stdout",
			StdErr:         "Mock stderr",
		},
	}, nil
}

type fakeRunCommandResolver struct {
	resolveResult *azure.RunCommandResult
	resolveErr    error
}

func (f *fakeRunCommandResolver) Resolve(_ context.Context) (*azure.RunCommandResult, error) {
	return f.resolveResult, f.resolveErr
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
