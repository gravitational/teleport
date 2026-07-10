/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package azure

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

const (
	testSubID = "11111111-1111-1111-1111-111111111111"
	rgName    = "rg1"

	vmResourceID = "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1"

	vmssResourceID   = "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachineScaleSets/vmss1"
	vmssVMResourceID = "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachineScaleSets/vmss1/virtualMachines/0"
)

const (
	testPowerStateRunning            = "PowerState/running"
	testPowerStateStopped            = "PowerState/stopped"
	testProvisioningStateSucceeded   = "ProvisioningState/succeeded"
	testProvisioningStateUnavailable = "ProvisioningState/Unavailable"
)

func testVMAgent(code string) *armcompute.VirtualMachineAgentInstanceView {
	return &armcompute.VirtualMachineAgentInstanceView{
		Statuses: []*armcompute.InstanceViewStatus{{Code: to.Ptr(code)}},
	}
}

func testPowerStatus(code string) []*armcompute.InstanceViewStatus {
	return []*armcompute.InstanceViewStatus{{Code: to.Ptr(code)}}
}

func testRegularVMWithInstanceView(agent *armcompute.VirtualMachineAgentInstanceView, statuses []*armcompute.InstanceViewStatus) armcompute.VirtualMachine {
	return armcompute.VirtualMachine{
		Properties: &armcompute.VirtualMachineProperties{
			InstanceView: &armcompute.VirtualMachineInstanceView{
				VMAgent:  agent,
				Statuses: statuses,
			},
		},
	}
}

func testScaleSetVMWithInstanceView(agent *armcompute.VirtualMachineAgentInstanceView, statuses []*armcompute.InstanceViewStatus) armcompute.VirtualMachineScaleSetVM {
	return armcompute.VirtualMachineScaleSetVM{
		Properties: &armcompute.VirtualMachineScaleSetVMProperties{
			InstanceView: &armcompute.VirtualMachineScaleSetVMInstanceView{
				VMAgent:  agent,
				Statuses: statuses,
			},
		},
	}
}

type fakeTokenCredential struct{}

func (fakeTokenCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     "fake-token",
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}

type fakeAzureRunCommandTransport struct {
	runCommandInProgress bool

	runCommandGetBody string
	vmStatusCode      int
	vmStatusBody      string
	vmssStatusCode    int
	vmssStatusBody    string

	runCommandPutCalls   int
	runCommandGetCalls   int
	vmStatusCalls        int
	vmssStatusCalls      int
	getCommandResultCall int
	putPaths             []string
	putRequestBody       string
}

func newFakeAzureRunCommandTransport() *fakeAzureRunCommandTransport {
	return &fakeAzureRunCommandTransport{
		runCommandGetBody: testRunCommandResultBody(string(armcompute.ExecutionStateSucceeded), 0, "install ok", ""),
		vmStatusCode:      http.StatusOK,
		vmStatusBody:      testVMInstanceViewBody(testProvisioningStateSucceeded, testPowerStateRunning),
		vmssStatusCode:    http.StatusOK,
		vmssStatusBody:    testVMInstanceViewBody(testProvisioningStateSucceeded, testPowerStateRunning),
	}
}

func (t *fakeAzureRunCommandTransport) Do(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	switch {
	case req.Method == http.MethodPut && strings.Contains(path, "/runCommands/"+runCommandName):
		t.runCommandPutCalls++
		t.putPaths = append(t.putPaths, path)
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		t.putRequestBody = string(body)
		if t.runCommandInProgress {
			return testHTTPResponse(req, http.StatusCreated, `{"properties":{"provisioningState":"Creating"}}`, map[string]string{
				"Azure-AsyncOperation": "https://management.azure.com/poll",
			}), nil
		}
		return testHTTPResponse(req, http.StatusOK, `{}`, nil), nil

	case req.Method == http.MethodGet && path == "/poll":
		t.getCommandResultCall++
		return testHTTPResponse(req, http.StatusOK, `{"status":"InProgress"}`, nil), nil

	case req.Method == http.MethodGet && strings.Contains(path, "/runCommands/"+runCommandName):
		t.runCommandGetCalls++
		return testHTTPResponse(req, http.StatusOK, t.runCommandGetBody, nil), nil

	case req.Method == http.MethodGet && strings.Contains(path, "/virtualMachineScaleSets/") && strings.Contains(path, "/virtualMachines/"):
		t.vmssStatusCalls++
		return testHTTPResponse(req, t.vmssStatusCode, t.vmssStatusBody, nil), nil

	case req.Method == http.MethodGet && strings.Contains(path, "/virtualMachines/"):
		t.vmStatusCalls++
		return testHTTPResponse(req, t.vmStatusCode, t.vmStatusBody, nil), nil
	}

	return testHTTPResponse(req, http.StatusNotFound, `{"error":{"code":"NotFound","message":"unexpected request"}}`, nil), nil
}

func testHTTPResponse(req *http.Request, statusCode int, body string, headers map[string]string) *http.Response {
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	for key, value := range headers {
		header.Set(key, value)
	}
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func testRunCommandResultBody(state string, exitCode int32, stdout, stderr string) string {
	return fmt.Sprintf(`{"properties":{"instanceView":{"executionState":%q,"exitCode":%d,"output":%q,"error":%q}}}`, state, exitCode, stdout, stderr)
}

func testVMInstanceViewBody(agentCode, powerCode string) string {
	return fmt.Sprintf(`{"properties":{"instanceView":{"vmAgent":{"statuses":[{"code":%q}]},"statuses":[{"code":%q}]}}}`, agentCode, powerCode)
}

func newTestRunCommandClient(t *testing.T, transport *fakeAzureRunCommandTransport) *runCommandClient {
	t.Helper()

	client, err := NewRunCommandClient(testSubID, fakeTokenCredential{}, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Transport: transport,
		},
	})
	require.NoError(t, err)
	runCommandClient, ok := client.(*runCommandClient)
	require.True(t, ok, "got %T", client)
	return runCommandClient
}

func TestGetVirtualMachine(t *testing.T) {
	ctx := context.Background()
	validResourceID := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm"

	for _, tc := range []struct {
		desc        string
		resourceID  string
		client      *ARMComputeMock
		assertError require.ErrorAssertionFunc
		assertVM    require.ValueAssertionFunc
	}{
		{
			desc:       "vm with valid user identities",
			resourceID: validResourceID,
			client: &ARMComputeMock{
				GetResult: armcompute.VirtualMachine{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
					Identity: &armcompute.VirtualMachineIdentity{
						PrincipalID: to.Ptr("system assigned"),
						Type:        to.Ptr(armcompute.ResourceIdentityTypeSystemAssigned),
						UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
							"identity1": {},
							"identity2": {},
						},
					},
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.ElementsMatch(t, []Identity{
					{ResourceID: "system assigned"},
					{ResourceID: "identity1"},
					{ResourceID: "identity2"},
				}, vm.Identities)
			},
		},
		{
			desc:       "vm without identity",
			resourceID: validResourceID,
			client: &ARMComputeMock{
				GetResult: armcompute.VirtualMachine{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.Empty(t, vm.Identities)
			},
		},
		{
			desc:       "vm with only user managed identities",
			resourceID: validResourceID,
			client: &ARMComputeMock{
				GetResult: armcompute.VirtualMachine{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
					Identity: &armcompute.VirtualMachineIdentity{
						UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
							"identity1": {},
							"identity2": {},
						},
					},
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.ElementsMatch(t, []Identity{
					{ResourceID: "identity1"},
					{ResourceID: "identity2"},
				}, vm.Identities)
			},
		},
		{
			desc:        "invalid resource ID",
			resourceID:  "random-id",
			client:      &ARMComputeMock{},
			assertError: require.Error,
			assertVM:    require.Nil,
		},
		{
			desc:       "client error",
			resourceID: validResourceID,
			client: &ARMComputeMock{
				GetErr: fmt.Errorf("client error"),
			},
			assertError: require.Error,
			assertVM:    require.Nil,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			vmClient := NewVirtualMachinesClientByAPI(VirtualMachinesClientConfig{
				VirtualMachineAPI: tc.client,
			})

			vm, err := vmClient.Get(ctx, tc.resourceID)
			tc.assertError(t, err)
			tc.assertVM(t, vm)
		})
	}
}

func TestGetScaleSetVirtualMachine(t *testing.T) {
	ctx := context.Background()
	validResourceID := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/vmss/virtualMachines/0"

	for _, tc := range []struct {
		desc        string
		resourceID  string
		client      *ARMScaleSetVMsMock
		assertError require.ErrorAssertionFunc
		assertVM    require.ValueAssertionFunc
	}{
		{
			desc:       "vm with valid user identities",
			resourceID: validResourceID,
			client: &ARMScaleSetVMsMock{
				GetResult: armcompute.VirtualMachineScaleSetVM{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
					Identity: &armcompute.VirtualMachineIdentity{
						PrincipalID: to.Ptr("system assigned"),
						Type:        to.Ptr(armcompute.ResourceIdentityTypeSystemAssigned),
						UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
							"identity1": {},
							"identity2": {},
						},
					},
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.ElementsMatch(t, []Identity{
					{ResourceID: "system assigned"},
					{ResourceID: "identity1"},
					{ResourceID: "identity2"},
				}, vm.Identities)
			},
		},
		{
			desc:       "vm without identity",
			resourceID: validResourceID,
			client: &ARMScaleSetVMsMock{
				GetResult: armcompute.VirtualMachineScaleSetVM{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.Empty(t, vm.Identities)
			},
		},
		{
			desc:       "vm with only user managed identities",
			resourceID: validResourceID,
			client: &ARMScaleSetVMsMock{
				GetResult: armcompute.VirtualMachineScaleSetVM{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
					Identity: &armcompute.VirtualMachineIdentity{
						UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
							"identity1": {},
							"identity2": {},
						},
					},
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.ElementsMatch(t, []Identity{
					{ResourceID: "identity1"},
					{ResourceID: "identity2"},
				}, vm.Identities)
			},
		},
		{
			desc:       "client error",
			resourceID: validResourceID,
			client: &ARMScaleSetVMsMock{
				GetErr: fmt.Errorf("client error"),
			},
			assertError: require.Error,
			assertVM:    require.Nil,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			vmClient := NewVirtualMachinesClientByAPI(VirtualMachinesClientConfig{
				ScaleSetVMsAPI: tc.client,
			})

			vm, err := vmClient.Get(ctx, tc.resourceID)
			tc.assertError(t, err)
			tc.assertVM(t, vm)
		})
	}
}

// collectVMIDs returns the VMID field from each VirtualMachine.
// VMID is the Azure-assigned UUID for the VM (e.g. "22222222-..."), distinct from
// the ARM resource ID. It is used throughout the tests as a stable identifier
// to verify which VMs were returned.
func collectVMIDs(vms []*VirtualMachine) []string {
	ids := make([]string, 0, len(vms))
	for _, vm := range vms {
		ids = append(ids, vm.VMID)
	}
	return ids
}

func TestListVirtualMachines(t *testing.T) {
	t.Parallel()

	// A well-formed regular VM whose ARM ID is parseable.
	regularVM := &armcompute.VirtualMachine{
		ID:       to.Ptr(vmResourceID),
		Name:     to.Ptr("vm1"),
		Location: to.Ptr("eastus"),
		Properties: &armcompute.VirtualMachineProperties{
			VMID: to.Ptr("vm-uuid-1"),
		},
	}

	buildVMWithVMID := func(vmID string) *armcompute.VirtualMachine {
		return &armcompute.VirtualMachine{
			ID:       to.Ptr(vmResourceID),
			Name:     to.Ptr("vm1"),
			Location: to.Ptr("eastus"),
			Properties: &armcompute.VirtualMachineProperties{
				VMID: to.Ptr(vmID),
			},
		}
	}

	// A well-formed uniform scale set with a parseable ARM ID.
	uniformScaleSet := &armcompute.VirtualMachineScaleSet{
		ID:   to.Ptr(vmssResourceID),
		Name: to.Ptr("vmss1"),
	}

	// A VM belonging to the scale set above.
	scaleSetVM := armcompute.VirtualMachineScaleSetVM{
		ID:         to.Ptr(vmssVMResourceID),
		Name:       to.Ptr("vmss1_0"),
		Location:   to.Ptr("eastus"),
		InstanceID: to.Ptr("0"),
		Properties: &armcompute.VirtualMachineScaleSetVMProperties{
			VMID: to.Ptr("vmss-vm-uuid-1"),
		},
	}

	for _, tc := range []struct {
		desc           string
		vmAPI          *ARMComputeMock
		scaleSetsAPI   *ARMScaleSetsMock
		scaleSetVMsAPI *ARMScaleSetVMsMock
		resourceGroup  string
		wantVMIDs      []string
	}{
		{
			desc: "regular VMs only, no scale sets",
			vmAPI: &ARMComputeMock{
				VirtualMachines: map[string][]*armcompute.VirtualMachine{
					rgName: {regularVM},
				},
			},
			scaleSetsAPI:   &ARMScaleSetsMock{},
			scaleSetVMsAPI: &ARMScaleSetVMsMock{},
			resourceGroup:  rgName,
			wantVMIDs:      []string{"vm-uuid-1"},
		},
		{
			desc:  "uniform VMSS VMs only, no regular VMs",
			vmAPI: &ARMComputeMock{VirtualMachines: map[string][]*armcompute.VirtualMachine{}},
			scaleSetsAPI: &ARMScaleSetsMock{
				ScaleSetRecords: []*armcompute.VirtualMachineScaleSet{uniformScaleSet},
			},
			scaleSetVMsAPI: &ARMScaleSetVMsMock{GetResult: scaleSetVM},
			resourceGroup:  rgName,
			wantVMIDs:      []string{"vmss-vm-uuid-1"},
		},
		{
			desc: "combines regular VMs and uniform VMSS VMs",
			vmAPI: &ARMComputeMock{
				VirtualMachines: map[string][]*armcompute.VirtualMachine{
					rgName: {regularVM},
				},
			},
			scaleSetsAPI: &ARMScaleSetsMock{
				ScaleSetRecords: []*armcompute.VirtualMachineScaleSet{uniformScaleSet},
			},
			scaleSetVMsAPI: &ARMScaleSetVMsMock{GetResult: scaleSetVM},
			resourceGroup:  rgName,
			wantVMIDs:      []string{"vm-uuid-1", "vmss-vm-uuid-1"},
		},
		{
			desc: "flexible scale set is skipped by the VMSS lister",
			vmAPI: &ARMComputeMock{
				VirtualMachines: map[string][]*armcompute.VirtualMachine{
					rgName: {regularVM},
				},
			},
			scaleSetsAPI: &ARMScaleSetsMock{
				ScaleSetRecords: []*armcompute.VirtualMachineScaleSet{
					{
						ID:   to.Ptr(vmssResourceID),
						Name: to.Ptr("vmss1"),
						Properties: &armcompute.VirtualMachineScaleSetProperties{
							OrchestrationMode: to.Ptr(armcompute.OrchestrationModeFlexible),
						},
					},
				},
			},
			// scaleSetVMsAPI would return a VM, but it must never be called for
			// a flexible scale set.
			scaleSetVMsAPI: &ARMScaleSetVMsMock{GetResult: scaleSetVM},
			resourceGroup:  rgName,
			wantVMIDs:      []string{"vm-uuid-1"},
		},
		{
			// When listing VMs within a scale set fails, the scale set is skipped
			// (inner loop breaks) but processing continues. Regular VMs are unaffected.
			desc:  "scale set VM listing error skips that scale set gracefully",
			vmAPI: &ARMComputeMock{VirtualMachines: map[string][]*armcompute.VirtualMachine{}},
			scaleSetsAPI: &ARMScaleSetsMock{
				ScaleSetRecords: []*armcompute.VirtualMachineScaleSet{uniformScaleSet},
			},
			scaleSetVMsAPI: &ARMScaleSetVMsMock{ListErr: trace.NotFound("vms not found")},
			resourceGroup:  rgName,
			wantVMIDs:      []string{},
		},
		{
			desc:  "scale set with invalid ARM ID is skipped, valid scale sets still processed",
			vmAPI: &ARMComputeMock{VirtualMachines: map[string][]*armcompute.VirtualMachine{}},
			scaleSetsAPI: &ARMScaleSetsMock{
				ScaleSetRecords: []*armcompute.VirtualMachineScaleSet{
					{ID: to.Ptr("not-a-valid-arm-id"), Name: to.Ptr("bad")},
					uniformScaleSet,
				},
			},
			scaleSetVMsAPI: &ARMScaleSetVMsMock{GetResult: scaleSetVM},
			resourceGroup:  rgName,
			wantVMIDs:      []string{"vmss-vm-uuid-1"},
		},
		{
			desc: "wildcard resource group returns VMs across all resource groups",
			vmAPI: &ARMComputeMock{
				VirtualMachines: map[string][]*armcompute.VirtualMachine{
					"rg1": {
						{
							ID:   to.Ptr("/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1"),
							Name: to.Ptr("vm1"),
							Properties: &armcompute.VirtualMachineProperties{
								VMID: to.Ptr("uuid-1"),
							},
						},
					},
					"rg2": {
						{
							ID:   to.Ptr("/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/rg2/providers/Microsoft.Compute/virtualMachines/vm2"),
							Name: to.Ptr("vm2"),
							Properties: &armcompute.VirtualMachineProperties{
								VMID: to.Ptr("uuid-2"),
							},
						},
					},
				},
			},
			scaleSetsAPI:   &ARMScaleSetsMock{},
			scaleSetVMsAPI: &ARMScaleSetVMsMock{},
			resourceGroup:  types.Wildcard,
			wantVMIDs:      []string{"uuid-1", "uuid-2"},
		},
		{
			desc: "existing resource group",
			vmAPI: &ARMComputeMock{
				VirtualMachines: map[string][]*armcompute.VirtualMachine{
					"rg1": {
						buildVMWithVMID("vm1"),
						buildVMWithVMID("vm2"),
					},
					"rg2": {
						buildVMWithVMID("vm3"),
						buildVMWithVMID("vm4"),
					},
				},
			},
			scaleSetsAPI:   &ARMScaleSetsMock{},
			scaleSetVMsAPI: &ARMScaleSetVMsMock{},
			resourceGroup:  "rg1",
			wantVMIDs:      []string{"vm1", "vm2"},
		},
		{
			desc: "nonexistent resource group",
			vmAPI: &ARMComputeMock{
				VirtualMachines: map[string][]*armcompute.VirtualMachine{
					"rg1": {
						buildVMWithVMID("vm1"),
						buildVMWithVMID("vm2"),
					},
					"rg2": {
						buildVMWithVMID("vm3"),
						buildVMWithVMID("vm4"),
					},
				},
			},
			scaleSetsAPI:   &ARMScaleSetsMock{},
			scaleSetVMsAPI: &ARMScaleSetVMsMock{},
			resourceGroup:  "rgfake",
			wantVMIDs:      []string{},
		},
		{
			desc: "all resource groups",
			vmAPI: &ARMComputeMock{
				VirtualMachines: map[string][]*armcompute.VirtualMachine{
					"rg1": {
						buildVMWithVMID("vm1"),
						buildVMWithVMID("vm2"),
					},
					"rg2": {
						buildVMWithVMID("vm3"),
						buildVMWithVMID("vm4"),
					},
				},
			},
			scaleSetsAPI:   &ARMScaleSetsMock{},
			scaleSetVMsAPI: &ARMScaleSetVMsMock{},
			resourceGroup:  types.Wildcard,
			wantVMIDs:      []string{"vm1", "vm2", "vm3", "vm4"},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			client := NewVirtualMachinesClientByAPI(VirtualMachinesClientConfig{
				VirtualMachineAPI: tc.vmAPI,
				ScaleSetsAPI:      tc.scaleSetsAPI,
				ScaleSetVMsAPI:    tc.scaleSetVMsAPI,
			})

			vms, err := client.ListVirtualMachines(t.Context(), tc.resourceGroup)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.wantVMIDs, collectVMIDs(vms))
		})
	}
}

// TestListVirtualMachines_VMSSFields verifies that VirtualMachines returned for
// uniform scale set VMs are populated with the correct scale set metadata.
func TestListVirtualMachines_VMSSFields(t *testing.T) {
	t.Parallel()

	scaleSetVM := armcompute.VirtualMachineScaleSetVM{
		ID:         to.Ptr(vmssVMResourceID),
		Name:       to.Ptr("vmss1_0"),
		Location:   to.Ptr("eastus"),
		InstanceID: to.Ptr("0"),
		Properties: &armcompute.VirtualMachineScaleSetVMProperties{
			VMID: to.Ptr("vmss-vm-uuid-1"),
		},
	}

	client := NewVirtualMachinesClientByAPI(VirtualMachinesClientConfig{
		VirtualMachineAPI: &ARMComputeMock{VirtualMachines: map[string][]*armcompute.VirtualMachine{}},
		ScaleSetsAPI: &ARMScaleSetsMock{
			ScaleSetRecords: []*armcompute.VirtualMachineScaleSet{
				{ID: to.Ptr(vmssResourceID), Name: to.Ptr("vmss1")},
			},
		},
		ScaleSetVMsAPI: &ARMScaleSetVMsMock{GetResult: scaleSetVM},
	})

	vms, err := client.ListVirtualMachines(t.Context(), rgName)
	require.NoError(t, err)
	require.Len(t, vms, 1)

	vm := vms[0]
	require.Equal(t, "vmss1", vm.UniformScaleSetName)
	require.Equal(t, "0", vm.UniformScaleSetVMInstanceID)
	require.Equal(t, testSubID, vm.Subscription)
	require.Equal(t, rgName, vm.ResourceGroup)
	require.Equal(t, "vmss-vm-uuid-1", vm.VMID)
	require.Equal(t, "eastus", vm.Location)
}

func TestVmScaleSetIsFlexible(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc       string
		properties *armcompute.VirtualMachineScaleSetProperties
		want       bool
	}{
		{
			desc:       "nil properties is not flexible (treated as Uniform)",
			properties: nil,
			want:       false,
		},
		{
			desc: "flexible orchestration mode",
			properties: &armcompute.VirtualMachineScaleSetProperties{
				OrchestrationMode: to.Ptr(armcompute.OrchestrationModeFlexible),
			},
			want: true,
		},
		{
			desc: "uniform orchestration mode",
			properties: &armcompute.VirtualMachineScaleSetProperties{
				OrchestrationMode: to.Ptr(armcompute.OrchestrationModeUniform),
			},
			want: false,
		},
		{
			desc:       "properties without orchestration mode is not flexible",
			properties: &armcompute.VirtualMachineScaleSetProperties{},
			want:       false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.want, vmScaleSetIsFlexible(tc.properties))
		})
	}
}

func TestRunCommandRequestIsUniformVMSS(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc string
		req  RunCommandRequest
		want bool
	}{
		{
			desc: "regular VM, no scale set fields",
			req:  RunCommandRequest{VMName: "vm1", ResourceGroup: "rg1"},
			want: false,
		},
		{
			desc: "both ScaleSetName and InstanceID set",
			req:  RunCommandRequest{VMName: "vmss1_0", UniformScaleSetName: "vmss1", UniformScaleSetVMInstanceID: "0"},
			want: true,
		},
		{
			desc: "only ScaleSetName, no InstanceID",
			req:  RunCommandRequest{VMName: "vmss1_0", UniformScaleSetName: "vmss1"},
			want: false,
		},
		{
			desc: "only InstanceID, no ScaleSetName",
			req:  RunCommandRequest{VMName: "vmss1_0", UniformScaleSetVMInstanceID: "0"},
			want: false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.want, tc.req.isUniformVMSS())
		})
	}
}

func TestRunCommandClientRun(t *testing.T) {
	t.Parallel()

	t.Run("regular VM returns command result", func(t *testing.T) {
		transport := newFakeAzureRunCommandTransport()
		client := newTestRunCommandClient(t, transport)

		result, err := client.Run(t.Context(), RunCommandRequest{
			Region:        "eastus",
			ResourceGroup: "rg1",
			VMName:        "vm1",
			Script:        "echo ok",
		})
		require.NoError(t, err)
		require.Equal(t, &RunCommandResult{
			ExecutionState: string(armcompute.ExecutionStateSucceeded),
			ExitCode:       0,
			StdOut:         "install ok",
			StdErr:         "",
		}, result)

		require.Equal(t, 1, transport.runCommandPutCalls)
		require.Equal(t, 1, transport.runCommandGetCalls)
		require.Len(t, transport.putPaths, 1)
		require.Contains(t, transport.putPaths[0], "/virtualMachines/vm1/runCommands/"+runCommandName)
		require.Contains(t, transport.putRequestBody, `"location":"eastus"`)
		require.Contains(t, transport.putRequestBody, `"asyncExecution":false`)
		require.Contains(t, transport.putRequestBody, `"script":"echo ok"`)
	})

	t.Run("regular VM timeout returns VM not running error when status says stopped", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			transport := newFakeAzureRunCommandTransport()
			transport.runCommandInProgress = true
			transport.vmStatusBody = testVMInstanceViewBody(testProvisioningStateSucceeded, testPowerStateStopped)
			client := newTestRunCommandClient(t, transport)

			result, err := client.Run(t.Context(), RunCommandRequest{
				Region:        "eastus",
				ResourceGroup: "rg1",
				VMName:        "vm1",
				Script:        "echo ok",
			})
			require.Nil(t, result)
			require.ErrorIs(t, err, &VMNotRunningError{}, "got %T: %v", err, err)
			require.NotErrorIs(t, err, context.DeadlineExceeded, "got %T: %v", err, err)
			require.Equal(t, 1, transport.runCommandPutCalls)
			require.Zero(t, transport.runCommandGetCalls)
			require.Equal(t, 1, transport.vmStatusCalls)
			require.NotZero(t, transport.getCommandResultCall)
		})
	})

	t.Run("regular VM timeout returns VM agent error when agent is unavailable", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			transport := newFakeAzureRunCommandTransport()
			transport.runCommandInProgress = true
			transport.vmStatusBody = testVMInstanceViewBody(testProvisioningStateUnavailable, testPowerStateRunning)
			client := newTestRunCommandClient(t, transport)

			result, err := client.Run(t.Context(), RunCommandRequest{
				Region:        "eastus",
				ResourceGroup: "rg1",
				VMName:        "vm1",
				Script:        "echo ok",
			})
			require.Nil(t, result)
			require.ErrorIs(t, err, &VMAgentNotAvailableError{}, "got %T: %v", err, err)
			require.NotErrorIs(t, err, context.DeadlineExceeded, "got %T: %v", err, err)
			require.Equal(t, 1, transport.vmStatusCalls)
		})
	})

	t.Run("regular VM timeout keeps original error when status cannot be fetched", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			transport := newFakeAzureRunCommandTransport()
			transport.runCommandInProgress = true
			transport.vmStatusCode = http.StatusForbidden
			transport.vmStatusBody = `{"error":{"code":"AuthorizationFailed","message":"missing read permission"}}`
			client := newTestRunCommandClient(t, transport)

			result, err := client.Run(t.Context(), RunCommandRequest{
				Region:        "eastus",
				ResourceGroup: "rg1",
				VMName:        "vm1",
				Script:        "echo ok",
			})
			require.Nil(t, result)
			require.ErrorIs(t, err, context.DeadlineExceeded, "got %T: %v", err, err)
			require.NotErrorIs(t, err, &VMNotRunningError{}, "got %T: %v", err, err)
			require.NotErrorIs(t, err, &VMAgentNotAvailableError{}, "got %T: %v", err, err)
			require.Equal(t, 1, transport.vmStatusCalls)
		})
	})

	t.Run("uniform VMSS timeout returns VM not running error when status says stopped", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			transport := newFakeAzureRunCommandTransport()
			transport.runCommandInProgress = true
			transport.vmssStatusBody = testVMInstanceViewBody(testProvisioningStateSucceeded, testPowerStateStopped)
			client := newTestRunCommandClient(t, transport)

			result, err := client.Run(t.Context(), RunCommandRequest{
				Region:                      "eastus",
				ResourceGroup:               "rg1",
				VMName:                      "vmss1_0",
				UniformScaleSetName:         "vmss1",
				UniformScaleSetVMInstanceID: "0",
				Script:                      "echo ok",
			})
			require.Nil(t, result)
			require.ErrorIs(t, err, &VMNotRunningError{}, "got %T: %v", err, err)
			require.NotErrorIs(t, err, context.DeadlineExceeded, "got %T: %v", err, err)
			require.Equal(t, 1, transport.runCommandPutCalls)
			require.Zero(t, transport.runCommandGetCalls)
			require.Equal(t, 1, transport.vmssStatusCalls)
			require.Len(t, transport.putPaths, 1)
			require.Contains(t, transport.putPaths[0], "/virtualMachineScaleSets/vmss1/virtualMachines/0/runCommands/"+runCommandName)
		})
	})
}

func TestRunCommandClientCheckVMStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc            string
		req             RunCommandRequest
		vmAPI           *ARMComputeMock
		scaleSetVMsAPI  *ARMScaleSetVMsMock
		requireErr      require.ErrorAssertionFunc
		requireErrIs    error
		requireNotErrIs error
	}{
		{
			desc: "regular VM running with ready agent",
			req:  RunCommandRequest{ResourceGroup: "rg1", VMName: "vm1"},
			vmAPI: &ARMComputeMock{
				GetResult: testRegularVMWithInstanceView(testVMAgent(testProvisioningStateSucceeded), testPowerStatus(testPowerStateRunning)),
			},
			requireErr: require.NoError,
		},
		{
			desc: "regular VM stopped returns VM not running error",
			req:  RunCommandRequest{ResourceGroup: "rg1", VMName: "vm1"},
			vmAPI: &ARMComputeMock{
				GetResult: testRegularVMWithInstanceView(testVMAgent(testProvisioningStateSucceeded), testPowerStatus(testPowerStateStopped)),
			},
			requireErr:      require.Error,
			requireErrIs:    &VMNotRunningError{},
			requireNotErrIs: &VMAgentNotAvailableError{},
		},
		{
			desc: "regular VM running with unavailable agent returns VM agent error",
			req:  RunCommandRequest{ResourceGroup: "rg1", VMName: "vm1"},
			vmAPI: &ARMComputeMock{
				GetResult: testRegularVMWithInstanceView(testVMAgent(testProvisioningStateUnavailable), testPowerStatus(testPowerStateRunning)),
			},
			requireErr:      require.Error,
			requireErrIs:    &VMAgentNotAvailableError{},
			requireNotErrIs: &VMNotRunningError{},
		},
		{
			desc: "status fetch errors do not block run command",
			req:  RunCommandRequest{ResourceGroup: "rg1", VMName: "vm1"},
			vmAPI: &ARMComputeMock{
				GetErr: trace.AccessDenied("missing read permission"),
			},
			requireErr: require.NoError,
		},
		{
			desc: "missing instance view does not block run command",
			req:  RunCommandRequest{ResourceGroup: "rg1", VMName: "vm1"},
			vmAPI: &ARMComputeMock{
				GetResult: armcompute.VirtualMachine{},
			},
			requireErr: require.NoError,
		},
		{
			desc: "uniform VMSS stopped returns VM not running error",
			req: RunCommandRequest{
				ResourceGroup:               "rg1",
				VMName:                      "vmss1_0",
				UniformScaleSetName:         "vmss1",
				UniformScaleSetVMInstanceID: "0",
			},
			scaleSetVMsAPI: &ARMScaleSetVMsMock{
				GetResult: testScaleSetVMWithInstanceView(testVMAgent(testProvisioningStateSucceeded), testPowerStatus(testPowerStateStopped)),
			},
			requireErr:      require.Error,
			requireErrIs:    &VMNotRunningError{},
			requireNotErrIs: &VMAgentNotAvailableError{},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			client := &runCommandClient{
				virtualMachineAPI: tc.vmAPI,
				scaleSetVMsAPI:    tc.scaleSetVMsAPI,
				logger:            logtest.NewLogger(),
			}

			err := client.checkVMStatus(t.Context(), tc.req)
			tc.requireErr(t, err)
			if tc.requireErrIs != nil {
				require.ErrorIs(t, err, tc.requireErrIs, "got %T: %v", err, err)
			}
			if tc.requireNotErrIs != nil {
				require.NotErrorIs(t, err, tc.requireNotErrIs, "got %T: %v", err, err)
			}
		})
	}
}

func TestCommandResultFromInstanceView(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc         string
		properties   *armcompute.VirtualMachineRunCommandProperties
		wantErr      bool
		wantState    string
		wantExitCode int32
		wantStdOut   string
		wantStdErr   string
		wantFailure  bool
	}{
		{
			desc:    "nil properties returns error",
			wantErr: true,
		},
		{
			desc:       "nil InstanceView returns error",
			properties: &armcompute.VirtualMachineRunCommandProperties{},
			wantErr:    true,
		},
		{
			desc: "succeeded command with zero exit code",
			properties: &armcompute.VirtualMachineRunCommandProperties{
				InstanceView: &armcompute.VirtualMachineRunCommandInstanceView{
					ExecutionState: to.Ptr(armcompute.ExecutionStateSucceeded),
					ExitCode:       to.Ptr(int32(0)),
					Output:         to.Ptr("install ok"),
					Error:          to.Ptr(""),
				},
			},
			wantState:    string(armcompute.ExecutionStateSucceeded),
			wantExitCode: 0,
			wantStdOut:   "install ok",
			wantStdErr:   "",
			wantFailure:  false,
		},
		{
			desc: "failed state with non-zero exit code",
			properties: &armcompute.VirtualMachineRunCommandProperties{
				InstanceView: &armcompute.VirtualMachineRunCommandInstanceView{
					ExecutionState: to.Ptr(armcompute.ExecutionStateFailed),
					ExitCode:       to.Ptr(int32(1)),
					Output:         to.Ptr(""),
					Error:          to.Ptr("command not found"),
				},
			},
			wantState:    string(armcompute.ExecutionStateFailed),
			wantExitCode: 1,
			wantStdErr:   "command not found",
			wantFailure:  true,
		},
		{
			desc: "succeeded state with non-zero exit code is still a failure",
			properties: &armcompute.VirtualMachineRunCommandProperties{
				InstanceView: &armcompute.VirtualMachineRunCommandInstanceView{
					ExecutionState: to.Ptr(armcompute.ExecutionStateSucceeded),
					ExitCode:       to.Ptr(int32(127)),
					Output:         to.Ptr(""),
					Error:          to.Ptr("script exited with code 127"),
				},
			},
			wantState:    string(armcompute.ExecutionStateSucceeded),
			wantExitCode: 127,
			wantStdErr:   "script exited with code 127",
			wantFailure:  true,
		},
		{
			desc: "nil pointer fields in InstanceView default to zero values",
			properties: &armcompute.VirtualMachineRunCommandProperties{
				InstanceView: &armcompute.VirtualMachineRunCommandInstanceView{},
			},
			wantState:    "",
			wantExitCode: 0,
			wantFailure:  true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			result, err := commandResultFromInstanceView(tc.properties)
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tc.wantState, result.ExecutionState)
			require.Equal(t, tc.wantExitCode, result.ExitCode)
			require.Equal(t, tc.wantStdOut, result.StdOut)
			require.Equal(t, tc.wantStdErr, result.StdErr)
			require.Equal(t, tc.wantFailure, result.Failure())
		})
	}
}

func TestVirtualMachineFromVirtualMachine(t *testing.T) {
	const (
		validResourceID = "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm-name"
		subscriptionID  = "11111111-1111-1111-1111-111111111111"
		resourceGroup   = "rg"
	)
	linuxOS := armcompute.OperatingSystemTypesLinux

	for _, tc := range []struct {
		desc        string
		vm          *armcompute.VirtualMachine
		assertError require.ErrorAssertionFunc
		assertVM    require.ValueAssertionFunc
	}{
		{
			desc: "vm with all fields populated",
			vm: &armcompute.VirtualMachine{
				ID:       to.Ptr(validResourceID),
				Name:     to.Ptr("vm-name"),
				Location: to.Ptr("eastus"),
				Properties: &armcompute.VirtualMachineProperties{
					VMID: to.Ptr("22222222-2222-2222-2222-222222222222"),
				},
				Tags: map[string]*string{
					"env":  to.Ptr("prod"),
					"team": to.Ptr("infra"),
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, &VirtualMachine{
					ID:            validResourceID,
					VMID:          "22222222-2222-2222-2222-222222222222",
					Name:          "vm-name",
					Location:      "eastus",
					Subscription:  subscriptionID,
					ResourceGroup: resourceGroup,
					Tags: map[string]string{
						"env":  "prod",
						"team": "infra",
					},
				}, vm)
			},
		},
		{
			desc: "vm with nil Properties does not panic and has empty VMID",
			vm: &armcompute.VirtualMachine{
				ID:       to.Ptr(validResourceID),
				Name:     to.Ptr("vm-name"),
				Location: to.Ptr("eastus"),
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Empty(t, vm.VMID)
				require.Equal(t, validResourceID, vm.ID)
				require.Equal(t, subscriptionID, vm.Subscription)
				require.Equal(t, resourceGroup, vm.ResourceGroup)
			},
		},
		{
			desc: "vm without tags returns empty (not nil) map",
			vm: &armcompute.VirtualMachine{
				ID:   to.Ptr(validResourceID),
				Name: to.Ptr("vm-name"),
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.NotNil(t, vm.Tags)
				require.Empty(t, vm.Tags)
			},
		},
		{
			desc: "vm not part of a Scale Set has empty Scale Set fields",
			vm: &armcompute.VirtualMachine{
				ID:   to.Ptr(validResourceID),
				Name: to.Ptr("vm-name"),
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Empty(t, vm.UniformScaleSetName)
				require.Empty(t, vm.UniformScaleSetVMInstanceID)
			},
		},
		{
			desc:        "nil vm returns error",
			vm:          nil,
			assertError: require.Error,
			assertVM:    require.Nil,
		},
		{
			desc: "invalid resource ID returns error",
			vm: &armcompute.VirtualMachine{
				ID:   to.Ptr("not-a-valid-arm-id"),
				Name: to.Ptr("vm-name"),
			},
			assertError: require.Error,
			assertVM:    require.Nil,
		},
		{
			desc: "vm with operating system defined",
			vm: &armcompute.VirtualMachine{
				ID:       to.Ptr(validResourceID),
				Name:     to.Ptr("vm-name"),
				Location: to.Ptr("eastus"),
				Properties: &armcompute.VirtualMachineProperties{
					VMID: to.Ptr("22222222-2222-2222-2222-222222222222"),
					StorageProfile: &armcompute.StorageProfile{
						OSDisk: &armcompute.OSDisk{
							OSType: &linuxOS,
						},
					},
				},
				Tags: map[string]*string{
					"env":  to.Ptr("prod"),
					"team": to.Ptr("infra"),
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, &VirtualMachine{
					ID:            validResourceID,
					VMID:          "22222222-2222-2222-2222-222222222222",
					Name:          "vm-name",
					Location:      "eastus",
					Subscription:  subscriptionID,
					ResourceGroup: resourceGroup,
					Tags: map[string]string{
						"env":  "prod",
						"team": "infra",
					},
					operatingSystem: &linuxOS,
				}, vm)
				require.True(t, vm.IsLinuxOrUnknown(), "expected IsLinuxOrUnknown() to return true for Linux VM")
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			vm, err := virtualMachineFromARMComputeVirtualMachine(tc.vm)
			tc.assertError(t, err)
			tc.assertVM(t, vm)
		})
	}
}

func TestVirtualMachineFromVirtualMachineScaleSetVM(t *testing.T) {
	const (
		validResourceID = "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/vmss/virtualMachines/0"
		subscriptionID  = "11111111-1111-1111-1111-111111111111"
		resourceGroup   = "rg"
		scaleSetName    = "vmss"
	)
	linuxOS := armcompute.OperatingSystemTypesLinux

	for _, tc := range []struct {
		desc           string
		vm             *armcompute.VirtualMachineScaleSetVM
		scaleSetName   string
		resourceGroup  string
		subscriptionID string
		assertError    require.ErrorAssertionFunc
		assertVM       require.ValueAssertionFunc
	}{
		{
			desc: "scale set vm with all fields populated",
			vm: &armcompute.VirtualMachineScaleSetVM{
				ID:         to.Ptr(validResourceID),
				Name:       to.Ptr("vmss_0"),
				Location:   to.Ptr("eastus"),
				InstanceID: to.Ptr("0"),
				Properties: &armcompute.VirtualMachineScaleSetVMProperties{
					VMID: to.Ptr("22222222-2222-2222-2222-222222222222"),
				},
				Tags: map[string]*string{
					"env":  to.Ptr("prod"),
					"team": to.Ptr("infra"),
				},
			},
			scaleSetName:   scaleSetName,
			resourceGroup:  resourceGroup,
			subscriptionID: subscriptionID,
			assertError:    require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, &VirtualMachine{
					ID:                          validResourceID,
					VMID:                        "22222222-2222-2222-2222-222222222222",
					Name:                        "vmss_0",
					Location:                    "eastus",
					Subscription:                subscriptionID,
					ResourceGroup:               resourceGroup,
					UniformScaleSetName:         scaleSetName,
					UniformScaleSetVMInstanceID: "0",
					Tags: map[string]string{
						"env":  "prod",
						"team": "infra",
					},
				}, vm)
			},
		},
		{
			desc: "scale set vm with nil Properties does not panic and has empty VMID",
			vm: &armcompute.VirtualMachineScaleSetVM{
				ID:         to.Ptr(validResourceID),
				Name:       to.Ptr("vmss_0"),
				Location:   to.Ptr("eastus"),
				InstanceID: to.Ptr("0"),
			},
			scaleSetName:   scaleSetName,
			resourceGroup:  resourceGroup,
			subscriptionID: subscriptionID,
			assertError:    require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Empty(t, vm.VMID)
				require.Equal(t, validResourceID, vm.ID)
				require.Equal(t, scaleSetName, vm.UniformScaleSetName)
				require.Equal(t, "0", vm.UniformScaleSetVMInstanceID)
			},
		},
		{
			desc: "scale set vm without tags returns empty (not nil) map",
			vm: &armcompute.VirtualMachineScaleSetVM{
				ID:         to.Ptr(validResourceID),
				Name:       to.Ptr("vmss_0"),
				InstanceID: to.Ptr("0"),
			},
			scaleSetName:   scaleSetName,
			resourceGroup:  resourceGroup,
			subscriptionID: subscriptionID,
			assertError:    require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.NotNil(t, vm.Tags)
				require.Empty(t, vm.Tags)
			},
		},
		{
			desc: "uses caller-provided subscription, resource group, and scale set name without parsing ID",
			vm: &armcompute.VirtualMachineScaleSetVM{
				ID:         to.Ptr("not-a-valid-arm-id"),
				Name:       to.Ptr("vmss_0"),
				InstanceID: to.Ptr("0"),
			},
			scaleSetName:   scaleSetName,
			resourceGroup:  resourceGroup,
			subscriptionID: subscriptionID,
			assertError:    require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, subscriptionID, vm.Subscription)
				require.Equal(t, resourceGroup, vm.ResourceGroup)
				require.Equal(t, scaleSetName, vm.UniformScaleSetName)
			},
		},
		{
			desc:           "nil vm returns error",
			vm:             nil,
			scaleSetName:   scaleSetName,
			resourceGroup:  resourceGroup,
			subscriptionID: subscriptionID,
			assertError:    require.Error,
			assertVM:       require.Nil,
		},
		{
			desc: "scale set vm with all fields populated",
			vm: &armcompute.VirtualMachineScaleSetVM{
				ID:         to.Ptr(validResourceID),
				Name:       to.Ptr("vmss_0"),
				Location:   to.Ptr("eastus"),
				InstanceID: to.Ptr("0"),
				Properties: &armcompute.VirtualMachineScaleSetVMProperties{
					VMID: to.Ptr("22222222-2222-2222-2222-222222222222"),
					StorageProfile: &armcompute.StorageProfile{
						OSDisk: &armcompute.OSDisk{
							OSType: &linuxOS,
						},
					},
				},
				Tags: map[string]*string{
					"env":  to.Ptr("prod"),
					"team": to.Ptr("infra"),
				},
			},
			scaleSetName:   scaleSetName,
			resourceGroup:  resourceGroup,
			subscriptionID: subscriptionID,
			assertError:    require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, &VirtualMachine{
					ID:                          validResourceID,
					VMID:                        "22222222-2222-2222-2222-222222222222",
					Name:                        "vmss_0",
					Location:                    "eastus",
					Subscription:                subscriptionID,
					ResourceGroup:               resourceGroup,
					UniformScaleSetName:         scaleSetName,
					UniformScaleSetVMInstanceID: "0",
					Tags: map[string]string{
						"env":  "prod",
						"team": "infra",
					},
					operatingSystem: &linuxOS,
				}, vm)
				require.True(t, vm.IsLinuxOrUnknown(), "expected IsLinuxOrUnknown() to return true for Linux VM")
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			vm, err := virtualMachineFromARMComputeVirtualMachineScaleSetVM(tc.vm, tc.subscriptionID, tc.resourceGroup, tc.scaleSetName)
			tc.assertError(t, err)
			tc.assertVM(t, vm)
		})
	}
}

func TestVirtualMachineIsLinuxOrUnknown(t *testing.T) {
	t.Parallel()
	linuxOS := armcompute.OperatingSystemTypesLinux
	windowsOS := armcompute.OperatingSystemTypesWindows

	for _, tc := range []struct {
		desc     string
		vm       *VirtualMachine
		want     bool
		wantDesc string
	}{
		{
			desc: "Linux VM",
			vm: &VirtualMachine{
				operatingSystem: &linuxOS,
			},
			want:     true,
			wantDesc: "Linux",
		},
		{
			desc: "Windows VM",
			vm: &VirtualMachine{
				operatingSystem: &windowsOS,
			},
			want:     false,
			wantDesc: "Windows",
		},
		{
			desc: "Unknown OSType",
			vm: &VirtualMachine{
				operatingSystem: nil,
			},
			want:     true,
			wantDesc: "Unknown",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got := tc.vm.IsLinuxOrUnknown()
			require.Equal(t, tc.want, got, "expected IsLinuxOrUnknown() to return %v for %s VM, got %v", tc.want, tc.wantDesc, got)
		})
	}
}
