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
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
// it received, so tests can run without contacting Azure. It returns, in order
// of precedence: an error if set; whatever respond returns (route by request); a
// fresh response from repeatBody on every call (for pagination loops); otherwise
// resp.
type fakeDoer struct {
	resp       *http.Response
	repeatBody string
	respond    func(*http.Request) (*http.Response, error)
	err        error
	calls      int
	last       *http.Request
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.calls++
	f.last = req
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

// emptyList is an empty paged list response body.
const emptyList = `{"value":[]}`

// listRoute routes the requests List makes to canned response bodies: the flat
// networkInterfaces list, the VM Scale Sets list, and per-scale-set NIC lists
// (perScaleSetNICs is returned for any of those). pages maps a nextLink path to
// its body. Empty bodies default to an empty list; unmatched requests 404.
func listRoute(flatNICs, scaleSets, perScaleSetNICs string, pages map[string]string) func(*http.Request) (*http.Response, error) {
	if flatNICs == "" {
		flatNICs = emptyList
	}
	if scaleSets == "" {
		scaleSets = emptyList
	}
	if perScaleSetNICs == "" {
		perScaleSetNICs = emptyList
	}
	return func(req *http.Request) (*http.Response, error) {
		p := req.URL.Path
		if body, ok := pages[p]; ok {
			return jsonResponse(http.StatusOK, body), nil
		}
		switch {
		case strings.Contains(p, "/virtualMachineScaleSets/") && strings.HasSuffix(p, "/networkInterfaces"):
			return jsonResponse(http.StatusOK, perScaleSetNICs), nil
		case strings.HasSuffix(p, "/virtualMachineScaleSets"):
			return jsonResponse(http.StatusOK, scaleSets), nil
		case strings.HasSuffix(p, "/networkInterfaces"):
			return jsonResponse(http.StatusOK, flatNICs), nil
		}
		return jsonResponse(http.StatusNotFound, `{}`), nil
	}
}

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
			nic, err := newTestClient(t, doer).Get(context.Background(), testNICID)
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
	_, err := newTestClient(t, doer).Get(context.Background(), testNICID)
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
			_, err := newTestClient(t, doer).Get(context.Background(), testNICID)
			require.Error(t, err)
			require.True(t, test.assertErr(err), "unexpected error type: %v", err)
		})
	}
}

func TestInterfacesClientGetInvalidResourceID(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{resp: jsonResponse(http.StatusOK, `{}`)}
	_, err := newTestClient(t, doer).Get(context.Background(), "not-a-valid-resource-id")
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

func TestInterfacesClientList(t *testing.T) {
	t.Parallel()

	// Flat-list page 1 has a NIC for vmA and an orphan NIC (no virtualMachine);
	// page 2 has a NIC for vmB. The orphan must be omitted from the result.
	page1 := `{"value":[` + nicJSON("nicA", testVMA, "10.0.0.4") + `,` + nicJSON("orphan", "", "10.0.0.5") + `],"nextLink":"https://management.azure.com/flat-page-2"}`
	page2 := `{"value":[` + nicJSON("nicB", testVMB, "10.0.0.6") + `]}`

	doer := &fakeDoer{respond: listRoute(page1, "", "", map[string]string{"/flat-page-2": page2})}

	nicsByVM, err := newTestClient(t, doer).List(context.Background(), "sub", "rg")
	require.NoError(t, err)

	require.Len(t, nicsByVM, 2, "orphan NIC without a VM must be omitted")

	a := nicsByVM[strings.ToLower(testVMA)]
	require.Len(t, a, 1)
	require.Equal(t, "10.0.0.4", a[0].PrivateIP)

	b := nicsByVM[strings.ToLower(testVMB)]
	require.Len(t, b, 1)
	require.Equal(t, "10.0.0.6", b[0].PrivateIP)
}

func TestInterfacesClientListGroupsMultipleNICs(t *testing.T) {
	t.Parallel()

	// A single VM with two attached NICs should collect both.
	body := `{"value":[` + nicJSON("nic1", testVMA, "10.0.0.4") + `,` + nicJSON("nic2", testVMA, "10.0.0.7") + `]}`
	doer := &fakeDoer{respond: listRoute(body, "", "", nil)}

	nicsByVM, err := newTestClient(t, doer).List(context.Background(), "sub", "rg")
	require.NoError(t, err)

	nics := nicsByVM[strings.ToLower(testVMA)]
	require.Len(t, nics, 2)
	ips := []string{nics[0].PrivateIP, nics[1].PrivateIP}
	require.ElementsMatch(t, []string{"10.0.0.4", "10.0.0.7"}, ips)
}

func TestInterfacesClientListMaxPages(t *testing.T) {
	t.Parallel()

	// Every response points to a next page, which would loop forever without the
	// page cap. Both the flat-list and scale-set enumerations hit it.
	doer := &fakeDoer{repeatBody: `{"value":[],"nextLink":"https://management.azure.com/next"}`}

	_, err := newTestClient(t, doer).List(context.Background(), "sub", "rg")
	require.Error(t, err)
	require.ErrorContains(t, err, "maximum")
}

func TestInterfacesClientListRequest(t *testing.T) {
	t.Parallel()

	// check lists with the given resource group and asserts both the flat NIC
	// request and the VM Scale Sets request use the expected path and api-version.
	check := func(t *testing.T, resourceGroup, wantNICPath, wantScaleSetPath string) {
		t.Helper()
		var reqs []*url.URL
		doer := &fakeDoer{respond: func(req *http.Request) (*http.Response, error) {
			reqs = append(reqs, req.URL)
			return jsonResponse(http.StatusOK, emptyList), nil
		}}
		_, err := newTestClient(t, doer).List(context.Background(), "sub", resourceGroup)
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

		require.NotNil(t, nicURL, "expected a flat network interfaces request")
		require.Equal(t, wantNICPath, nicURL.Path)
		require.Equal(t, interfaceAPIVersion, nicURL.Query().Get("api-version"))

		require.NotNil(t, scaleSetURL, "expected a VM Scale Sets request")
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

func TestInterfacesClientListIncludesUniformVMSS(t *testing.T) {
	t.Parallel()

	const (
		uniformSSID     = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/uniform-ss"
		flexSSID        = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/flex-ss"
		uniformInstance = uniformSSID + "/virtualMachines/0"
	)

	flat := `{"value":[` + nicJSON("standalone-nic", testVMA, "10.0.0.4") + `]}`
	scaleSets := `{"value":[` +
		`{"id":"` + uniformSSID + `","name":"uniform-ss","properties":{"orchestrationMode":"Uniform"}},` +
		`{"id":"` + flexSSID + `","name":"flex-ss","properties":{"orchestrationMode":"Flexible"}}` +
		`]}`
	uniformNICs := `{"value":[` + nicJSON("uniform-nic", uniformInstance, "10.0.0.5") + `]}`

	var requestedPaths []string
	doer := &fakeDoer{respond: func(req *http.Request) (*http.Response, error) {
		p := req.URL.Path
		requestedPaths = append(requestedPaths, p)
		switch {
		case strings.Contains(p, "/virtualMachineScaleSets/") && strings.HasSuffix(p, "/networkInterfaces"):
			return jsonResponse(http.StatusOK, uniformNICs), nil
		case strings.HasSuffix(p, "/virtualMachineScaleSets"):
			return jsonResponse(http.StatusOK, scaleSets), nil
		case strings.HasSuffix(p, "/networkInterfaces"):
			return jsonResponse(http.StatusOK, flat), nil
		}
		return jsonResponse(http.StatusNotFound, `{}`), nil
	}}

	nicsByVM, err := newTestClient(t, doer).List(context.Background(), "sub", "rg")
	require.NoError(t, err)

	// The standalone VM comes from the flat list; the uniform VMSS instance comes
	// from the per-scale-set list.
	require.Len(t, nicsByVM, 2)
	require.Equal(t, "10.0.0.4", nicsByVM[strings.ToLower(testVMA)][0].PrivateIP)
	require.Equal(t, "10.0.0.5", nicsByVM[strings.ToLower(uniformInstance)][0].PrivateIP)

	// The flexible scale set must NOT be queried for NICs; its instances are
	// already covered by the flat list.
	for _, p := range requestedPaths {
		require.NotContains(t, p, "flex-ss/networkInterfaces")
	}
}
