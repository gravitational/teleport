package jamf_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/jamf"
	"github.com/gravitational/teleport/e/lib/jamf/testenv"
)

func TestClient_GetComputersInventory(t *testing.T) {
	env := testenv.MustNew(nil /* opts */)
	defer env.Close()

	api := env.API
	client := env.Client

	dev1 := &jamf.ComputerInventory{
		ID:   "1",
		UDID: "11",
		General: &jamf.ComputerGeneralSection{
			Name:            "llama",
			LastContactTime: time.Unix(1685469902, 0), // 2023-05-30T18:05:02+00:00
		},
		Hardware: &jamf.ComputerHardwareSection{
			ModelIdentifier: "MacBookPro9,2",
		},
		LocalUserAccounts: []*jamf.LocalUserAccount{
			{
				UID:      "501",
				Username: "llama",
			},
		},
		OperatingSystem: &jamf.ComputerOperatingSystemSection{
			Name:    "Mac OS X",
			Version: "13.4",
		},
	}
	dev2 := &jamf.ComputerInventory{
		ID:   "2",
		UDID: "22",
		General: &jamf.ComputerGeneralSection{
			Name:            "alpaca",
			LastContactTime: time.Unix(1685468902, 0), // 2023-05-30T17:48:02+00:00
		},
		Hardware: &jamf.ComputerHardwareSection{
			ModelIdentifier: "MacBookPro10,1",
		},
		LocalUserAccounts: []*jamf.LocalUserAccount{
			{
				UID:      "501",
				Username: "alpaca",
			},
		},
		OperatingSystem: &jamf.ComputerOperatingSystemSection{
			Name:    "Mac OS X",
			Version: "13.3.1",
		},
	}

	api.SetInventory([]*jamf.ComputerInventory{
		dev1,
		dev2,
	})

	const totalCount = 2
	wantAllDefault := &jamf.GetComputersInventoryResponse{
		TotalCount: totalCount,
		Results: []*jamf.ComputerInventory{
			// dev2 comes before dev1 in the default order (general.name:asc).
			{
				ID:      dev2.ID,
				UDID:    dev2.UDID,
				General: dev2.General,
			},
			{
				ID:      dev1.ID,
				UDID:    dev1.UDID,
				General: dev1.General,
			},
		},
	}

	allSections := []string{
		jamf.SectionGeneral,
		jamf.SectionHardware,
		jamf.SectionLocalUserAccounts,
		jamf.SectionOperatingSystem,
	}
	wantAllFull := &jamf.GetComputersInventoryResponse{
		TotalCount: totalCount,
		Results: []*jamf.ComputerInventory{
			// Default sort ("general.name:asc").
			dev2, dev1,
		},
	}

	wantEmpty := &jamf.GetComputersInventoryResponse{
		TotalCount: totalCount,
	}

	tests := []struct {
		name string
		req  *jamf.GetComputersInventoryRequest
		want *jamf.GetComputersInventoryResponse
	}{
		{
			name: "default query",
			req:  &jamf.GetComputersInventoryRequest{},
			want: wantAllDefault,
		},
		{
			name: "sections",
			req: &jamf.GetComputersInventoryRequest{
				Section: allSections,
			},
			want: wantAllFull,
		},
		{
			name: "sort",
			req: &jamf.GetComputersInventoryRequest{
				Section: allSections, // easier to craft response
				Sort:    []string{"general.lastContactTime:desc"},
			},
			want: &jamf.GetComputersInventoryResponse{
				TotalCount: totalCount,
				Results: []*jamf.ComputerInventory{
					dev1, dev2, // dev1.General.LastContactTime > dev2.General.LastContactTime
				},
			},
		},
		{
			name: "page",
			req: &jamf.GetComputersInventoryRequest{
				Page: 2, // there's no page 2
			},
			want: wantEmpty,
		},
		{
			name: "pagination 1/2",
			req: &jamf.GetComputersInventoryRequest{
				Section:  allSections,
				Page:     0, // 1st page
				PageSize: 1,
			},
			want: &jamf.GetComputersInventoryResponse{
				TotalCount: totalCount,
				Results:    []*jamf.ComputerInventory{dev2},
			},
		},
		{
			name: "pagination 2/2",
			req: &jamf.GetComputersInventoryRequest{
				Section:  allSections,
				Page:     1, // 2nd page
				PageSize: 1,
			},
			want: &jamf.GetComputersInventoryResponse{
				TotalCount: totalCount,
				Results:    []*jamf.ComputerInventory{dev1},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := client.GetComputersInventory(t.Context(), test.req)
			if err != nil {
				t.Fatalf("GetComputersInventory failed: %v", err)
			}
			if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("GetComputersInventory mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestClient_GetComputersInventoryID(t *testing.T) {
	env := testenv.MustNew(nil /* opts */)
	defer env.Close()

	api := env.API
	client := env.Client

	t.Run("invalid ID", func(t *testing.T) {
		for _, id := range []string{
			"../scripts",
			"..",
			"foo/bar",
			"with space",
		} {
			_, err := client.GetComputersInventoryByID(t.Context(), &jamf.GetComputersInventoryByIDRequest{ID: id})
			require.Error(t, err, "ID %q", id)
			require.True(t, trace.IsBadParameter(err), "ID %q", id)
		}
	})

	inv := []*jamf.ComputerInventory{
		{
			ID: "2",
			General: &jamf.ComputerGeneralSection{
				Name:     "llama's macbook",
				Platform: "Mac",
			},
			Hardware: &jamf.ComputerHardwareSection{
				ModelIdentifier: "MacBookPro9,2",
				SerialNumber:    "CXXXXXXXXXX1",
			},
		},
		{
			ID: "3",
			General: &jamf.ComputerGeneralSection{
				Name:     "alpaca's macbook",
				Platform: "Mac",
			},
			Hardware: &jamf.ComputerHardwareSection{
				ModelIdentifier: "MacBookPro9,2",
				SerialNumber:    "CXXXXXXXXXX2",
			},
		},
	}
	llamaComputer := inv[0]
	alpacaComputer := inv[1]
	api.SetInventory(inv)

	tests := []struct {
		name         string
		req          *jamf.GetComputersInventoryByIDRequest
		wantStatus   int // only for errors
		wantComputer *jamf.ComputerInventory
	}{
		{
			name: "ok with default sections",
			req: &jamf.GetComputersInventoryByIDRequest{
				ID: llamaComputer.ID,
			},
			wantComputer: &jamf.ComputerInventory{
				ID:      llamaComputer.ID,
				General: llamaComputer.General,
			},
		},
		{
			name: "ok with custom sections",
			req: &jamf.GetComputersInventoryByIDRequest{
				ID: alpacaComputer.ID,
				Section: []string{
					jamf.SectionGeneral,
					jamf.SectionHardware,
				},
			},
			wantComputer: alpacaComputer,
		},
		{
			name: "not found",
			req: &jamf.GetComputersInventoryByIDRequest{
				ID: "404",
			},
			wantStatus: 404,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := client.GetComputersInventoryByID(t.Context(), test.req)
			if test.wantStatus > 0 {
				apiErr := &jamf.APIError{}
				if !errors.As(err, &apiErr) || apiErr.StatusCode != test.wantStatus {
					t.Errorf("GetComputersInventoryByID returned err=%q, want error with status=%v", err, test.wantStatus)
				}
				return
			}

			if diff := cmp.Diff(test.wantComputer, got); diff != "" {
				t.Errorf("GetComputersInventoryID mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestClient_ComputersInventoryV1(t *testing.T) {
	t.Parallel()

	env := testenv.MustNew(nil /* opts */)
	t.Cleanup(func() { env.Close() })

	api := env.API
	client := env.Client
	ctx := t.Context()

	verifyNotFound := func(t *testing.T, err error, endpoint string) {
		t.Helper()

		var target *jamf.APIError
		require.ErrorAs(t, err, &target, "%s: error type mismatch", endpoint)
		require.Equal(t, http.StatusNotFound, target.StatusCode, "%s: unexpected status code", endpoint)
	}

	// Sanity check: initially "v2" is supported, then it isn't.
	{
		_, err := client.GetV2ComputersInventory(ctx, &jamf.GetComputersInventoryRequest{})
		require.NoError(t, err, "client.GetV2ComputersInventory() errored unexpectedly")

		api.SetDisableComputersInventoryV2(true)
		_, err = client.GetV2ComputersInventory(ctx, &jamf.GetComputersInventoryRequest{})
		require.Error(t, err, "client.GetV2ComputersInventory() succeeded unexpectedly")
		verifyNotFound(t, err, "GET /v2/computers-inventory")
	}

	inv := []*jamf.ComputerInventory{
		{
			ID: "2",
			General: &jamf.ComputerGeneralSection{
				Name:     "llama's macbook",
				Platform: "Mac",
			},
		},
		{
			ID: "3",
			General: &jamf.ComputerGeneralSection{
				Name:     "alpaca's macbook",
				Platform: "Mac",
			},
		},
	}
	api.SetInventory(inv)

	// Sanity check: GetV2ByID also breaks.
	{
		_, err := client.GetV2ComputersInventoryByID(ctx, &jamf.GetComputersInventoryByIDRequest{
			ID:      inv[0].ID,
			Section: []string{jamf.SectionGeneral},
		})
		require.Error(t, err, "client.GetV2ComputersInventoryByID() succeeded unexpectedly")
		verifyNotFound(t, err, "GET /v2/computers-inventory/{id}")
	}

	// Re-create the client so the bootstrap marks "v2" as unavailable.
	client = env.MustNewClient()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		resp, err := client.GetComputersInventory(ctx, &jamf.GetComputersInventoryRequest{})
		require.NoError(t, err, "client.GetComputersInventory() errored")
		if diff := cmp.Diff(inv, resp.Results); diff != "" {
			t.Errorf("client.GetComputersInventory() mismatch (-want +got)\n%s", diff)
		}
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		want := inv[0]
		got, err := client.GetComputersInventoryByID(ctx, &jamf.GetComputersInventoryByIDRequest{
			ID:      want.ID,
			Section: []string{jamf.SectionGeneral},
		})
		require.NoError(t, err, "client.GetComputersInventoryByID() errored")
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("client.GetComputersInventoryByID() mismatch (-want +got)\n%s", diff)
		}
	})
}

func TestClient_GetMobileDevicesDetail(t *testing.T) {
	env := testenv.MustNew(nil /* opts */)
	defer env.Close()

	api := env.API
	client := env.Client

	dev1 := &jamf.MobileDevice{
		MobileDeviceID: "1",
		DeviceType:     "iOS",
		General: &jamf.MobileDeviceGeneralSection{
			OSVersion:               "26.3.1",
			OSBuild:                 "23D8133",
			LastInventoryUpdateDate: time.Unix(1685468902, 0), // 2023-05-30T17:48:02+00:00
			LastEnrolledDate:        time.Unix(1685468902, 0),
		},
		Hardware: &jamf.MobileDeviceHardwareSection{
			ModelIdentifier: "iPad15,7",
			SerialNumber:    "CXXXXXXXXXX3",
		},
	}
	dev2 := &jamf.MobileDevice{
		MobileDeviceID: "2",
		DeviceType:     "iOS",
		General: &jamf.MobileDeviceGeneralSection{
			OSVersion:               "26.4",
			OSBuild:                 "24C2234",
			LastInventoryUpdateDate: time.Unix(1685469902, 0), // 2023-05-30T18:05:02+00:00
			LastEnrolledDate:        time.Unix(1685469902, 0),
		},
		Hardware: &jamf.MobileDeviceHardwareSection{
			ModelIdentifier: "iPhone15,2",
			SerialNumber:    "CXXXXXXXXXX4",
		},
	}

	api.SetMobileDeviceInventory([]*jamf.MobileDevice{dev2, dev1})

	const totalCount = 2
	allSections := []string{
		jamf.MobileDeviceSectionGeneral,
		jamf.MobileDeviceSectionHardware,
	}

	wantAllDefault := &jamf.GetMobileDevicesDetailResponse{
		TotalCount: totalCount,
		Results: []*jamf.MobileDevice{
			// Default sort in the fake API is by mobileDeviceId:asc.
			// Default section is GENERAL only.
			{
				MobileDeviceID: dev1.MobileDeviceID,
				DeviceType:     dev1.DeviceType,
				General:        dev1.General,
			},
			{
				MobileDeviceID: dev2.MobileDeviceID,
				DeviceType:     dev2.DeviceType,
				General:        dev2.General,
			},
		},
	}

	wantAllFull := &jamf.GetMobileDevicesDetailResponse{
		TotalCount: totalCount,
		Results:    []*jamf.MobileDevice{dev1, dev2},
	}

	wantEmpty := &jamf.GetMobileDevicesDetailResponse{
		TotalCount: totalCount,
	}

	tests := []struct {
		name string
		req  *jamf.GetMobileDevicesDetailRequest
		want *jamf.GetMobileDevicesDetailResponse
	}{
		{
			name: "default query",
			req:  &jamf.GetMobileDevicesDetailRequest{},
			want: wantAllDefault,
		},
		{
			name: "sections",
			req: &jamf.GetMobileDevicesDetailRequest{
				Section: allSections,
			},
			want: wantAllFull,
		},
		{
			name: "sort by lastInventoryUpdateDate desc",
			req: &jamf.GetMobileDevicesDetailRequest{
				Section: allSections,
				Sort:    []string{"lastInventoryUpdateDate:desc"},
			},
			want: &jamf.GetMobileDevicesDetailResponse{
				TotalCount: totalCount,
				Results:    []*jamf.MobileDevice{dev2, dev1},
			},
		},
		{
			name: "page out of range",
			req: &jamf.GetMobileDevicesDetailRequest{
				Page: 2, // there's no page 2
			},
			want: wantEmpty,
		},
		{
			name: "pagination 1/2",
			req: &jamf.GetMobileDevicesDetailRequest{
				Section:  allSections,
				Page:     0,
				PageSize: 1,
			},
			want: &jamf.GetMobileDevicesDetailResponse{
				TotalCount: totalCount,
				Results:    []*jamf.MobileDevice{dev1},
			},
		},
		{
			name: "pagination 2/2",
			req: &jamf.GetMobileDevicesDetailRequest{
				Section:  allSections,
				Page:     1,
				PageSize: 1,
			},
			want: &jamf.GetMobileDevicesDetailResponse{
				TotalCount: totalCount,
				Results:    []*jamf.MobileDevice{dev2},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := client.GetMobileDevicesDetail(t.Context(), test.req)
			if err != nil {
				t.Fatalf("GetMobileDevicesDetail failed: %v", err)
			}
			if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("GetMobileDevicesDetail mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestClient_GetMobileDeviceByID(t *testing.T) {
	env := testenv.MustNew(nil /* opts */)
	defer env.Close()

	api := env.API
	client := env.Client

	inv := []*jamf.MobileDevice{
		{
			MobileDeviceID: "1",
			DeviceType:     "iOS",
			Hardware: &jamf.MobileDeviceHardwareSection{
				ModelIdentifier: "iPad15,7",
				SerialNumber:    "CXXXXXXXXXX3",
			},
		},
		{
			MobileDeviceID: "2",
			DeviceType:     "iOS",
			Hardware: &jamf.MobileDeviceHardwareSection{
				ModelIdentifier: "iPhone15,2",
				SerialNumber:    "CXXXXXXXXXX4",
			},
		},
	}
	dev2 := inv[1]
	api.SetMobileDeviceInventory(inv)

	tests := []struct {
		name       string
		req        *jamf.GetMobileDeviceByIDRequest
		wantStatus int
		want       *jamf.MobileDeviceDetails
	}{
		{
			name: "ok",
			req:  &jamf.GetMobileDeviceByIDRequest{ID: dev2.MobileDeviceID},
			want: &jamf.MobileDeviceDetails{
				ID:           dev2.MobileDeviceID,
				SerialNumber: dev2.Hardware.SerialNumber,
				Type:         "ios",
				IOS: &jamf.MobileDeviceDetailsIOS{
					ModelIdentifier: dev2.Hardware.ModelIdentifier,
				},
			},
		},
		{
			name:       "not found",
			req:        &jamf.GetMobileDeviceByIDRequest{ID: "404"},
			wantStatus: 404,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := client.GetMobileDeviceByID(t.Context(), test.req)
			if test.wantStatus > 0 {
				apiErr := &jamf.APIError{}
				if !errors.As(err, &apiErr) || apiErr.StatusCode != test.wantStatus {
					t.Errorf("GetMobileDeviceByID returned err=%q, want error with status=%v", err, test.wantStatus)
				}
				return
			}

			require.NoError(t, err)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("GetMobileDeviceByID mismatch (-want +got)\n%s", diff)
			}
		})
	}
}
