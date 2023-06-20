package jamf_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/gravitational/teleport/e/lib/jamf"
	"github.com/gravitational/teleport/e/lib/jamf/testenv"
)

func TestClient_GetComputersInventory(t *testing.T) {
	env := testenv.MustNew(nil /* opts */)
	defer env.Close()

	api := env.API
	client := env.Client
	ctx := context.Background()

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
			got, err := client.GetComputersInventory(ctx, test.req)
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
	ctx := context.Background()

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
			got, err := client.GetComputersInventoryByID(ctx, test.req)
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
