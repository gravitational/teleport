/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package network

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/azure/client"
)

// TestInterfacesClientLive exercises the network interfaces client against a
// real Azure subscription. It is skipped unless TELEPORT_TEST_AZURE is set.
//
// Prerequisites:
//   - Authenticate so DefaultAzureCredential can find a credential, e.g. run
//     `az login`, or set AZURE_TENANT_ID / AZURE_CLIENT_ID / AZURE_CLIENT_SECRET.
//     The identity needs Microsoft.Network/networkInterfaces/read on the NIC.
//   - Set TELEPORT_TEST_AZURE_NIC_ID to a NIC resource ID, e.g.
//     /subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Network/networkInterfaces/<name>
//
// Run with:
//
//	TELEPORT_TEST_AZURE=1 \
//	TELEPORT_TEST_AZURE_NIC_ID=/subscriptions/.../networkInterfaces/my-nic \
//	go test ./lib/cloud/azure/network/ -run TestInterfacesClientLiveGet -v
func TestInterfacesClientLiveGet(t *testing.T) {
	if os.Getenv("TELEPORT_TEST_AZURE") == "" {
		t.Skip("Set TELEPORT_TEST_AZURE to run this test against a real Azure subscription.")
	}
	nicID := os.Getenv("TELEPORT_TEST_AZURE_NIC_ID")
	if nicID == "" {
		t.Skip("Set TELEPORT_TEST_AZURE_NIC_ID to the resource ID of a real Azure network interface.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	require.NoError(t, err, "failed to build a default Azure credential; have you run `az login`?")

	queryClient, err := client.NewClient(cred, client.WithRetryOnRateLimitErrors())
	require.NoError(t, err)

	nicClient := NewInterfacesClient(queryClient)

	nic, err := nicClient.Get(ctx, nicID)
	require.NoError(t, err)
	require.NotNil(t, nic)

	t.Logf("NIC %q: private IP=%q", nic.Name, nic.PrivateIP)

	require.Equal(t, nicID, nic.ID)
	require.NotEmpty(t, nic.PrivateIP, "expected the NIC to have a private IP")
}

// TestInterfacesClientLiveList exercises the network interfaces client against
// a real Azure subscription, listing all NICs. It is skipped unless
// TELEPORT_TEST_AZURE is set.
//
// Prerequisites:
//   - Authenticate so DefaultAzureCredential can find a credential, e.g. run
//     `az login`, or set AZURE_TENANT_ID / AZURE_CLIENT_ID / AZURE_CLIENT_SECRET.
//     The identity needs Microsoft.Network/networkInterfaces/read on the NICs.
//   - Set AZURE_SUBSCRIPTION_ID to a subscription ID that has at least one NIC.
//   - Optionally set AZURE_RESOURCE_GROUP to a resource group that has at least one NIC.
//
// Run with:
//
//		TELEPORT_TEST_AZURE=1 \
//		AZURE_SUBSCRIPTION_ID=<sub> \
//	  AZURE_RESOURCE_GROUP=<rg> \
//		go test ./lib/cloud/azure/network/ -run TestInterfacesClientLiveList -v
func TestInterfacesClientLiveList(t *testing.T) {
	if os.Getenv("TELEPORT_TEST_AZURE") == "" {
		t.Skip("Set TELEPORT_TEST_AZURE to run this test against a real Azure subscription.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	require.NoError(t, err, "failed to build a default Azure credential; have you run `az login`?")

	queryClient, err := client.NewClient(cred, client.WithRetryOnRateLimitErrors())
	require.NoError(t, err)

	nicClient := NewInterfacesClient(queryClient)

	rg := os.Getenv("AZURE_RESOURCE_GROUP")
	if rg == "" {
		rg = types.Wildcard
	}
	nicsByVM, err := nicClient.List(ctx, os.Getenv("AZURE_SUBSCRIPTION_ID"), rg)
	require.NoError(t, err)
	require.NotNil(t, nicsByVM)

	var uniform, other int
	for vmID, nics := range nicsByVM {
		if strings.Contains(strings.ToLower(vmID), "/virtualmachinescalesets/") {
			uniform++
		} else {
			other++
		}
		t.Logf("VM %q has %d NIC(s):", vmID, len(nics))
		for _, nic := range nics {
			t.Logf("  NIC %q: private IP=%q", nic.Name, nic.PrivateIP)
		}
	}
	t.Logf("nicsByVM: total=%d uniform-VMSS=%d standalone/flexible=%d", len(nicsByVM), uniform, other)
}

const testNICID = "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.Network/networkInterfaces/my-nic"
const token = "fake-token"

// fakeDoer is an HTTP doer that returns canned responses and records the request
// it received, so tests can run without contacting Azure.
type fakeDoer struct {
	resp       *http.Response
	repeatBody string
	respond    func(*http.Request) (*http.Response, error)
	err        error

	mu    sync.Mutex
	calls int
	last  *http.Request
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.calls++
	f.last = req
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	if f.respond != nil {
		return f.respond(req)
	}
	if f.repeatBody != "" {
		resp := jsonResponse(http.StatusOK, f.repeatBody)
		resp.Request = req
		return resp, nil
	}
	f.resp.Request = req
	return f.resp, nil
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// uniformSet is a single uniform VM Scale Set with one instance NIC.
type uniformSet struct {
	id, instance, nicName, privateIP string
}

// uniformScaleSets builds n uniform scale sets and their VM Scale Sets list
// entries (unwrapped JSON objects, ready to compose into a "value" array).
func uniformScaleSets(n int) ([]uniformSet, []string) {
	sets := make([]uniformSet, n)
	entries := make([]string, n)
	for i := range sets {
		name := fmt.Sprintf("uniform-ss-%d", i)
		id := "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/" + name
		sets[i] = uniformSet{
			id:        id,
			instance:  fmt.Sprintf("%s/virtualMachines/%d", id, i),
			nicName:   name + "-nic",
			privateIP: fmt.Sprintf("10.1.0.%d", i+1),
		}
		entries[i] = fmt.Sprintf(`{"id":%q,"name":%q,"properties":{"orchestrationMode":"Uniform"}}`, id, name)
	}
	return sets, entries
}

// setForPath returns the scale set that matches the given path, or false if
// none match.
func setForPath(sets []uniformSet, path string) (uniformSet, bool) {
	for _, s := range sets {
		if strings.HasPrefix(path, s.id+"/") {
			return s, true
		}
	}
	return uniformSet{}, false
}

// emptyList is an empty paged list response body.
const emptyList = `{"value":[]}`

func newTestClient(t *testing.T, doer *fakeDoer) InterfacesClient {
	t.Helper()
	cred := azure.NewStaticCredential(azcore.AccessToken{Token: token})
	queryClient, err := client.NewClient(cred, client.WithHTTPClient(doer))
	require.NoError(t, err)
	return NewInterfacesClient(queryClient)
}

func TestInterfacesClientGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		body          string
		wantName      string
		wantPrivateIP string
	}{
		{
			name:          "single primary ip config",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.0.4","primary":true}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "10.0.0.4",
		},
		{
			name:          "multiple ip configs, primary not first",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.0.5"}},{"properties":{"privateIPAddress":"10.0.0.6","primary":true}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "10.0.0.6",
		},
		{
			name:          "no primary flagged falls back to the only ip",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.0.7"}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "10.0.0.7",
		},
		{
			name:          "no primary flagged falls back to the first ip",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.0.9"}},{"properties":{"privateIPAddress":"10.0.0.10"}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "10.0.0.9",
		},
		{
			name:          "empty ip config is skipped",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":""}},{"properties":{"privateIPAddress":"10.0.0.8"}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "10.0.0.8",
		},
		{
			name:          "cidr secondary config is skipped in fallback",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"172.20.2.16/28"}},{"properties":{"privateIPAddress":"10.0.0.9"}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "10.0.0.9",
		},
		{
			name:          "only a cidr config yields no ip",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"172.20.2.16/28"}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "",
		},
		{
			name:          "no properties",
			body:          `{"name":"my-nic"}`,
			wantName:      "my-nic",
			wantPrivateIP: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			doer := &fakeDoer{resp: jsonResponse(http.StatusOK, test.body)}
			nic, err := newTestClient(t, doer).Get(t.Context(), testNICID)
			require.NoError(t, err)
			require.Equal(t, testNICID, nic.ID)
			require.Equal(t, test.wantName, nic.Name)
			require.Equal(t, test.wantPrivateIP, nic.PrivateIP)
		})
	}
}

func TestInterfacesClientGetRequest(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{resp: jsonResponse(http.StatusOK, `{"name":"my-nic"}`)}
	_, err := newTestClient(t, doer).Get(t.Context(), testNICID)
	require.NoError(t, err)

	require.Equal(t, 1, doer.calls)
	req := doer.last
	require.NotNil(t, req)
	require.Equal(t, http.MethodGet, req.Method)
	require.Equal(t, "management.azure.com", req.URL.Host)
	require.Equal(t, testNICID, req.URL.Path)
	require.Equal(t, interfaceAPIVersion, req.URL.Query().Get("api-version"))
	require.Equal(t, "Bearer "+token, req.Header.Get("Authorization"))
}

func TestInterfacesClientGetErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    int
		assertErr func(error) bool
	}{
		{"not found", http.StatusNotFound, trace.IsNotFound},
		{"access denied", http.StatusForbidden, trace.IsAccessDenied},
		{"server error", http.StatusInternalServerError, func(err error) bool { return err != nil }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			doer := &fakeDoer{resp: jsonResponse(test.status, `{"error":{"code":"fail","message":"bye"}}`)}
			_, err := newTestClient(t, doer).Get(t.Context(), testNICID)
			require.Error(t, err)
			require.True(t, test.assertErr(err), "unexpected error type: %v", err)
		})
	}
}

func TestInterfacesClientGetInvalidResourceID(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{resp: jsonResponse(http.StatusOK, `{}`)}
	_, err := newTestClient(t, doer).Get(t.Context(), "not-a-valid-resource-id")
	require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	require.Zero(t, doer.calls, "transport must not be called for an invalid resource ID")
}

const (
	testVMA = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vmA"
	testVMB = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vmB"
)

func nicJSON(name, vmID, privateIP string) string {
	vm := ""
	if vmID != "" {
		vm = `"virtualMachine":{"id":"` + vmID + `"},`
	}
	return `{"id":"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/networkInterfaces/` + name + `",` +
		`"name":"` + name + `","properties":{` + vm +
		`"ipConfigurations":[{"properties":{"privateIPAddress":"` + privateIP + `","primary":true}}]}}`
}

// TestInterfacesClientList checks that standalone, flexible-VMSS, and
// uniform-VMSS NICs are all listed, grouped per VM, with orphan NICs dropped.
func TestInterfacesClientList(t *testing.T) {
	t.Parallel()

	const (
		flexSSID     = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/flex-ss"
		flexInstance = flexSSID + "/virtualMachines/0"
	)

	uniform, uniformEntries := uniformScaleSets(3)

	// The VM Scale Sets list enumerates the uniform sets plus a flexible set. The
	// flexible set's instances are already in the flat list, so it must not be
	// queried per-set.
	scaleSets := `{"value":[` + strings.Join(append(uniformEntries,
		`{"id":"`+flexSSID+`","name":"flex-ss","properties":{"orchestrationMode":"Flexible"}}`), ",") + `]}`

	// Two pages of flat NICs: standalone vmA (one NIC per page, so grouping spans
	// pages) and vmB, the flexible-VMSS instance, and an orphan NIC to be dropped.
	flatPage1 := `{"value":[` + nicJSON("nicA1", testVMA, "10.0.0.4") + `,` + nicJSON("orphan", "", "10.0.0.5") + `],"nextLink":"https://management.azure.com/flat-page-2"}`
	flatPage2 := `{"value":[` + nicJSON("nicA2", testVMA, "10.0.0.7") + `,` + nicJSON("nicB", testVMB, "10.0.0.6") + `,` + nicJSON("flex-nic", flexInstance, "10.0.0.8") + `]}`

	var mu sync.Mutex
	var requestedPaths []string
	doer := &fakeDoer{respond: func(req *http.Request) (*http.Response, error) {
		p := req.URL.Path
		mu.Lock()
		requestedPaths = append(requestedPaths, p)
		mu.Unlock()
		switch {
		case p == "/flat-page-2":
			return jsonResponse(http.StatusOK, flatPage2), nil
		case strings.Contains(p, "/virtualMachineScaleSets/") && strings.HasSuffix(p, "/networkInterfaces"):
			if s, ok := setForPath(uniform, p); ok {
				return jsonResponse(http.StatusOK, `{"value":[`+nicJSON(s.nicName, s.instance, s.privateIP)+`]}`), nil
			}
			return jsonResponse(http.StatusNotFound, `{}`), nil
		case strings.HasSuffix(p, "/virtualMachineScaleSets"):
			return jsonResponse(http.StatusOK, scaleSets), nil
		case strings.HasSuffix(p, "/networkInterfaces"):
			return jsonResponse(http.StatusOK, flatPage1), nil
		}
		return jsonResponse(http.StatusNotFound, `{}`), nil
	}}

	nicsByVM, err := newTestClient(t, doer).List(t.Context(), "sub", "rg")
	require.NoError(t, err)

	// VMs with NICs: standalone vmA and vmB and the flexible-VMSS instance (all
	// from the flat list), plus one instance per uniform scale set. The orphan NIC
	// is dropped.
	require.Len(t, nicsByVM, 3+len(uniform))

	// vmA's two NICs are grouped together, collected across both flat pages.
	vmA := nicsByVM[strings.ToLower(testVMA)]
	require.Len(t, vmA, 2)
	require.ElementsMatch(t, []string{"10.0.0.4", "10.0.0.7"}, []string{vmA[0].PrivateIP, vmA[1].PrivateIP})

	// Standalone vmB and the flexible-VMSS instance each have one NIC.
	require.Equal(t, "10.0.0.6", nicsByVM[strings.ToLower(testVMB)][0].PrivateIP)
	require.Equal(t, "10.0.0.8", nicsByVM[strings.ToLower(flexInstance)][0].PrivateIP)

	// Each uniform scale set's instance NIC is fetched per-set and merged in.
	for _, s := range uniform {
		nics := nicsByVM[strings.ToLower(s.instance)]
		require.Len(t, nics, 1, "expected exactly one NIC for %s", s.instance)
		require.Equal(t, s.privateIP, nics[0].PrivateIP)
	}

	// The flexible scale set shouldn't be requested directly through the Compute
	// API. Instead it should have been found via the Network API.
	for _, p := range requestedPaths {
		require.NotContains(t, p, "flex-ss/networkInterfaces")
	}
}

func TestInterfacesClientListMaxPages(t *testing.T) {
	t.Parallel()

	// Every response points to a next page, which would loop forever without the
	// page cap.
	doer := &fakeDoer{repeatBody: `{"value":[],"nextLink":"https://management.azure.com/next"}`}

	_, err := newTestClient(t, doer).List(t.Context(), "sub", "rg")
	require.Error(t, err)
	require.ErrorIs(t, err, trace.LimitExceeded("listing Azure network interfaces exceeded the maximum of %d pages", maxListPages))
}

func TestInterfacesClientListRequest(t *testing.T) {
	t.Parallel()

	// check lists with the given resource group and asserts both the Network API
	// request and the VM Scale Sets Compute API request use the expected path and
	// api-version.
	check := func(t *testing.T, resourceGroup, wantNICPath, wantScaleSetPath string) {
		t.Helper()
		var reqs []*url.URL
		doer := &fakeDoer{respond: func(req *http.Request) (*http.Response, error) {
			reqs = append(reqs, req.URL)
			return jsonResponse(http.StatusOK, emptyList), nil
		}}
		_, err := newTestClient(t, doer).List(t.Context(), "sub", resourceGroup)
		require.NoError(t, err)

		var nicURL, scaleSetURL *url.URL
		for _, u := range reqs {
			switch {
			case strings.HasSuffix(u.Path, "/providers/Microsoft.Network/networkInterfaces"):
				nicURL = u
			case strings.HasSuffix(u.Path, "/providers/Microsoft.Compute/virtualMachineScaleSets"):
				scaleSetURL = u
			}
		}

		require.NotNil(t, nicURL, "expected a network API interfaces request")
		require.Equal(t, wantNICPath, nicURL.Path)
		require.Equal(t, interfaceAPIVersion, nicURL.Query().Get("api-version"))

		require.NotNil(t, scaleSetURL, "expected a compute API request")
		require.Equal(t, wantScaleSetPath, scaleSetURL.Path)
		require.Equal(t, computeAPIVersion, scaleSetURL.Query().Get("api-version"))
	}

	t.Run("scoped to resource group", func(t *testing.T) {
		t.Parallel()
		check(t, "rg",
			"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/networkInterfaces",
			"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets")
	})

	t.Run("wildcard lists whole subscription", func(t *testing.T) {
		t.Parallel()
		check(t, types.Wildcard,
			"/subscriptions/sub/providers/Microsoft.Network/networkInterfaces",
			"/subscriptions/sub/providers/Microsoft.Compute/virtualMachineScaleSets")
	})
}
