package api_test

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/intune/api"
	"github.com/gravitational/teleport/e/lib/intune/testenv"
)

func TestClient_ListManagedDevices(t *testing.T) {
	clock := clockwork.NewFakeClock()
	env := testenv.MustNew(t, &testenv.Config{
		Clock: clock,
	})
	fakeAPI := env.API

	dev1 := &api.ManagedDevice{
		ID:               "1",
		LastSyncDateTime: time.Unix(1685468902, 0), // 2023-05-30T17:48:02+00:00
		SerialNumber:     "Parallels-1C2F1D1B421942123F4A82ACA583A5DA",
		Model:            "Parallels ARM Virtual Machine",
		OperatingSystem:  "Windows",
		OSVersion:        "10.0.26100.4351",
	}
	dev2 := &api.ManagedDevice{
		ID:               "2",
		LastSyncDateTime: time.Unix(1685469902, 0), // 2023-05-30T18:05:02+00:00
		SerialNumber:     "F4GQP2RTMD6N",
		Model:            "MacBookPro9,2",
		OperatingSystem:  "macOS",
		OSVersion:        "15.5 (24F74)",
	}
	dev3 := &api.ManagedDevice{
		ID:               "3",
		LastSyncDateTime: time.Unix(1685468902, 0), // 2023-05-30T17:48:02+00:00
		SerialNumber:     "",
		Model:            "",
		OperatingSystem:  "Linux (ubuntu)",
		OSVersion:        "24.04",
	}

	fakeAPI.SetManagedDevices([]*api.ManagedDevice{dev1, dev2, dev3})

	client := env.MustNewClient(t)

	t.Run("list and filter by lastSyncDateTime", func(t *testing.T) {
		resp, err := client.ListManagedDevices(t.Context(), &api.ListManagedDevicesRequest{})
		require.NoError(t, err)
		require.Len(t, resp.ManagedDevices, 3)
		var lastSyncedDevice *api.ManagedDevice
		for _, device := range resp.ManagedDevices {
			if lastSyncedDevice == nil || device.LastSyncDateTime.After(lastSyncedDevice.LastSyncDateTime) {
				lastSyncedDevice = device
			}
		}
		require.Equal(t, dev2.ID, lastSyncedDevice.ID)

		resp2, err := client.ListManagedDevices(t.Context(), &api.ListManagedDevicesRequest{
			LastSyncDateTime: lastSyncedDevice.LastSyncDateTime.Add(-time.Second)})
		require.NoError(t, err)
		require.Len(t, resp2.ManagedDevices, 1, "Filtering by LastSyncDateTime doesn't work")
		require.Equal(t, lastSyncedDevice.ID, resp2.ManagedDevices[0].ID)
	})

	t.Run("limit number of results", func(t *testing.T) {
		resp, err := client.ListManagedDevices(t.Context(), &api.ListManagedDevicesRequest{
			Top: 1})
		require.NoError(t, err)
		require.Len(t, resp.ManagedDevices, 1)

		resp2, err := client.ListManagedDevices(t.Context(), &api.ListManagedDevicesRequest{
			Top: 2})
		require.NoError(t, err)
		require.Len(t, resp2.ManagedDevices, 2)
	})

	t.Run("pagination", func(t *testing.T) {
		resp, err := client.ListManagedDevices(t.Context(), &api.ListManagedDevicesRequest{
			Top: 1})
		require.NoError(t, err)
		require.Len(t, resp.ManagedDevices, 1)
		require.Equal(t, dev1.ID, resp.ManagedDevices[0].ID)
		require.NotEmpty(t, resp.NextLink)

		resp2, err := client.ListManagedDevices(t.Context(), &api.ListManagedDevicesRequest{
			NextLink: resp.NextLink})
		require.NoError(t, err)
		require.Len(t, resp2.ManagedDevices, 1)
		require.Equal(t, dev2.ID, resp2.ManagedDevices[0].ID)
		require.NotEmpty(t, resp2.NextLink)

		resp3, err := client.ListManagedDevices(t.Context(), &api.ListManagedDevicesRequest{
			NextLink: resp2.NextLink})
		require.NoError(t, err)
		require.Len(t, resp3.ManagedDevices, 1)
		require.Equal(t, dev3.ID, resp3.ManagedDevices[0].ID)
		require.Empty(t, resp3.NextLink)
	})
}
