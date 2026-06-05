package web

import (
	"encoding/json"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestListDevices_byAssetTag(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t, withModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust: {Enabled: true},
			},
		},
	}))
	webPack := s.newAuthWebPack(t, "foo")
	endpoint := webPack.clt.Endpoint("enterprise", "devices")
	authClient := s.newAdminAuthClient(s.ctx, t)

	// create test devices
	_, err := authClient.DevicesClient().BulkCreateDevices(s.ctx, devicepb.BulkCreateDevicesRequest_builder{
		Devices: []*devicepb.Device{
			devicepb.Device_builder{
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				AssetTag:     "device2",
				EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
			}.Build(),
			devicepb.Device_builder{
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				AssetTag:     "device4",
				EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
			}.Build(),
		},
	}.Build())
	require.NoError(t, err)

	var testCases = []struct {
		name, assetTag string
		expected       *ui.ListDevicesResponse
		wantEmpty      bool
	}{
		{
			name:     "search with correct asset tag",
			assetTag: "device2",
			expected: &ui.ListDevicesResponse{
				Items:    []ui.Device{{AssetTag: "device2"}},
				StartKey: "",
			},
		},
		{
			name:      "search with invalid asset tag",
			assetTag:  "device2invalid",
			wantEmpty: true,
			expected: &ui.ListDevicesResponse{
				Items:    []ui.Device{},
				StartKey: "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{
				// limit value should be ignored when search is not empty
				"limit":    []string{"50"},
				"startKey": []string{""},
				"search":   []string{tc.assetTag},
			})
			require.NoError(t, err)

			deviceResp := unmarshalWebResponse(t, resp.Bytes())

			if tc.wantEmpty {
				require.Empty(t, deviceResp.Items)
			} else {
				require.Len(t, deviceResp.Items, 1)
				require.Equal(t, tc.expected.Items[0].AssetTag, deviceResp.Items[0].AssetTag)
			}
		})
	}
}

func TestListDevices_paginated(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t, withModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust: {Enabled: true},
			},
		},
	}))
	webPack := s.newAuthWebPack(t, "foo")
	endpoint := webPack.clt.Endpoint("enterprise", "devices")
	authClient := s.newAdminAuthClient(s.ctx, t)

	testDevices := []*devicepb.Device{
		devicepb.Device_builder{
			OsType:       devicepb.OSType_OS_TYPE_MACOS,
			AssetTag:     "device1",
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
		}.Build(),
		devicepb.Device_builder{
			OsType:       devicepb.OSType_OS_TYPE_MACOS,
			AssetTag:     "device2",
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
		}.Build(),
		devicepb.Device_builder{
			OsType:       devicepb.OSType_OS_TYPE_MACOS,
			AssetTag:     "device3",
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
		}.Build(),

		devicepb.Device_builder{
			OsType:       devicepb.OSType_OS_TYPE_MACOS,
			AssetTag:     "device4",
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
		}.Build(),
	}

	// create test devices
	_, err := authClient.DevicesClient().BulkCreateDevices(s.ctx, devicepb.BulkCreateDevicesRequest_builder{
		Devices: testDevices,
	}.Build())
	require.NoError(t, err)

	t.Run("paginated query", func(t *testing.T) {
		// totalDevices stores devices retreived till the last page.
		respTotalDevices := make([]ui.Device, 0)

		// Page1: Test page with limit. This should return 2 elements and a startKey
		resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{
			"limit":    []string{"2"},
			"startKey": []string{""},
		})
		require.NoError(t, err)

		respDevices := unmarshalWebResponse(t, resp.Bytes())
		require.Len(t, respDevices.Items, 2)
		require.NotEmpty(t, respDevices.StartKey)

		respTotalDevices = append(respTotalDevices, respDevices.Items...)

		// Page2: Query next 1 element and using startKey received in Page1
		resp, err = webPack.clt.Get(s.ctx, endpoint, url.Values{
			"limit":    []string{"1"},
			"startKey": []string{respDevices.StartKey},
		})
		require.NoError(t, err)

		respDevices = unmarshalWebResponse(t, resp.Bytes())
		require.Len(t, respDevices.Items, 1)
		require.NotEmpty(t, respDevices.StartKey)

		respTotalDevices = append(respTotalDevices, respDevices.Items...)

		// Page3: Query next 1 element and using startKey received in Page2
		resp, err = webPack.clt.Get(s.ctx, endpoint, url.Values{
			"limit":    []string{"1"},
			"startKey": []string{respDevices.StartKey},
		})
		require.NoError(t, err)

		respDevices = unmarshalWebResponse(t, resp.Bytes())
		require.Len(t, respDevices.Items, 1)
		require.NotEmpty(t, respDevices.StartKey)

		respTotalDevices = append(respTotalDevices, respDevices.Items...)

		// Page4: It should return 0 element as we've already reached last item.
		resp, err = webPack.clt.Get(s.ctx, endpoint, url.Values{
			"limit":    []string{"1"},
			"startKey": []string{respDevices.StartKey},
		})
		require.NoError(t, err)

		respDevices = unmarshalWebResponse(t, resp.Bytes())
		require.Empty(t, respDevices.Items)

		// Test if totalDevices length is 4 (total number of devices available in the storage)
		require.Len(t, respTotalDevices, 4)

		// finally test if expected devices are in resposne
		requireAssetTagsEqual(t, toUIDevices(testDevices), respTotalDevices)
	})
}

func requireAssetTagsEqual(t *testing.T, want, got []ui.Device) {
	slices.SortFunc(want, func(a, b ui.Device) int {
		return strings.Compare(a.AssetTag, b.AssetTag)
	})
	slices.SortFunc(got, func(a, b ui.Device) int {
		return strings.Compare(a.AssetTag, b.AssetTag)
	})
	for i, w := range want {
		require.Equal(t, w.AssetTag, got[i].AssetTag)
	}
}

func unmarshalWebResponse(t *testing.T, resp []byte) *ui.ListDevicesResponse {
	var deviceResp *ui.ListDevicesResponse

	err := json.Unmarshal(resp, &deviceResp)
	require.NoError(t, err)

	return deviceResp
}

func TestListDevices_errors(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t, withModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust: {Enabled: true},
			},
		},
	}))
	webPack := s.newAuthWebPack(t, "foo")
	endpoint := webPack.clt.Endpoint("enterprise", "devices")

	var testCases = []struct {
		name           string
		url            url.Values
		badParamErrMsg string
		expected       string
	}{
		{
			name: "test negative limit",
			url: url.Values{
				"limit":    []string{"-2"},
				"startKey": []string{""},
			},
			// empty response
			expected: "{\"items\":[]}",
		},
		{
			name: "test empty limit",
			url: url.Values{
				"limit":    []string{""},
				"startKey": []string{""},
			},
			//  empty string
			badParamErrMsg: " ",
		},
		{
			name: "invalid limit value",
			url: url.Values{
				"limit":    []string{"12invalid"},
				"startKey": []string{""},
			},
			badParamErrMsg: "12invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := webPack.clt.Get(s.ctx, endpoint, tc.url)
			if tc.badParamErrMsg != "" {
				require.ErrorContains(t, err, tc.badParamErrMsg)
			} else {
				require.NoError(t, err)
				require.Contains(t, string(resp.Bytes()), tc.expected)
			}
		})
	}
}
