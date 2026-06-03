package service

import (
	"cmp"
	"context"
	"strings"
	"time"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/jamf"
	"github.com/gravitational/teleport/e/lib/mdmsync"
)

const (
	sortByReportDateDesc = "general.reportDate:desc"
)

func (s *S) verifyComputersInventoryFilters(ctx context.Context, entry *types.JamfInventoryEntry) error {
	if _, err := s.jamf.GetComputersInventory(ctx, &jamf.GetComputersInventoryRequest{
		Page:     0,
		PageSize: 1,
		Sort:     []string{sortByReportDateDesc},
		Filter:   entry.FilterRsql,
	}); err != nil {
		return trace.BadParameter("computer inventory query, filter=%q: %v", entry.FilterRsql, err)
	}
	return nil
}

// getComputersPage reads a single page of computer inventory from Jamf.
func (s *S) getComputersPage(
	ctx context.Context, spec RunSpec, pageNum int,
) (rawJamfPage, error) {
	req := &jamf.GetComputersInventoryRequest{
		Section: []string{
			jamf.SectionGeneral,
			jamf.SectionHardware,
			jamf.SectionLocalUserAccounts,
			jamf.SectionOperatingSystem,
		},
		PageSize: spec.PageSize,
		Page:     pageNum,
		Sort:     []string{"id:asc"}, // expected to be more "stable" than timestamps
		Filter:   spec.FilterRSQL,
	}

	// Sort by recent use on PARTIAL syncs.
	// Alternatively we could use an RSQL filter.
	if spec.Mode == mdmsync.SyncModePartial {
		req.Sort = []string{sortByReportDateDesc}
	}

	resp, err := s.jamf.GetComputersInventory(ctx, req)
	if err != nil {
		return rawJamfPage{}, trace.Wrap(err, "jamf computers read failed")
	}
	devices := make([]jamfDevice, len(resp.Results))
	for i, inv := range resp.Results {
		devices[i] = computerAdapter{inv}
	}
	return rawJamfPage{devices: devices, totalCount: resp.TotalCount}, nil
}

// confirmComputer checks whether a computer still exists in Jamf and matches
// the expected OS type and serial number. Returns (true, computer, nil) if the
// device is found and matches.
func (s *S) confirmComputer(ctx context.Context, dev *devicepb.Device) (matches bool, jamfDevForLogging any, err error) {
	computer, err := s.jamf.GetComputersInventoryByID(ctx, &jamf.GetComputersInventoryByIDRequest{
		ID: dev.Profile.GetExternalId(),
		Section: []string{
			jamf.SectionGeneral,  // for Platform
			jamf.SectionHardware, // for SerialNumber
		},
	})
	if err != nil {
		return false, nil, err
	}
	matches = computer.General != nil &&
		platformToOSType(computer.General.Platform) == dev.OsType &&
		computer.Hardware != nil &&
		computer.Hardware.SerialNumber == dev.AssetTag
	return matches, computer, nil
}

func computerInventoryToDevice(c *jamf.ComputerInventory) (*devicepb.Device, error) {
	if c == nil {
		// This is rather unexpected, but let's guard against it anyway.
		return nil, trace.BadParameter("computer inventory is nil")
	}

	// General has the Platform.
	// Hardware has the SerialNumber.
	// That's the bare minimum we need, everything else is DeviceProfile info.
	if c.General == nil || c.Hardware == nil {
		return nil, trace.BadParameter("computer inventory has no general or hardware section")
	}

	osType := platformToOSType(c.General.Platform)
	if osType == devicepb.OSType_OS_TYPE_UNSPECIFIED {
		return nil, trace.BadParameter("unexpected general.platform=%q", c.General.Platform)
	}

	usernames := make([]string, 0, len(c.LocalUserAccounts))
	for _, account := range c.LocalUserAccounts {
		if account != nil && account.Username != "" {
			usernames = append(usernames, account.Username)
		}
	}

	profile := &devicepb.DeviceProfile{
		ModelIdentifier:   c.Hardware.ModelIdentifier,
		OsUsernames:       usernames,
		JamfBinaryVersion: c.General.JamfBinaryVersion,
		ExternalId:        c.ID,
	}
	if c.OperatingSystem != nil {
		profile.OsVersion = c.OperatingSystem.Version
		profile.OsBuild = c.OperatingSystem.Build
		profile.OsBuildSupplemental = c.OperatingSystem.SupplementalBuildVersion
	}

	return &devicepb.Device{
		OsType:   osType,
		AssetTag: c.Hardware.SerialNumber,
		Profile:  profile,
	}, nil
}

func platformToOSType(platform string) devicepb.OSType {
	if strings.EqualFold("Mac", platform) {
		return devicepb.OSType_OS_TYPE_MACOS
	}
	return devicepb.OSType_OS_TYPE_UNSPECIFIED
}

type computerAdapter struct {
	inv *jamf.ComputerInventory
}

func (a computerAdapter) toDevice() (*devicepb.Device, error) {
	return computerInventoryToDevice(a.inv)
}

func (a computerAdapter) cutTime() time.Time {
	if a.inv != nil && a.inv.General != nil {
		return a.inv.General.ReportDate
	}
	return time.Time{}
}

func (a computerAdapter) platform() string {
	if a.inv.General != nil {
		return a.inv.General.Platform
	}
	return ""
}

func (a computerAdapter) serialNumber() string {
	if a.inv.Hardware != nil {
		return a.inv.Hardware.SerialNumber
	}
	return ""
}

func (a computerAdapter) logSync(ctx context.Context, log logFunc, dev *devicepb.Device) {
	osUsernames := dev.Profile.OsUsernames
	dev.Profile.OsUsernames = []string{"<REDACTED>"}
	general := cmp.Or(a.inv.General, &jamf.ComputerGeneralSection{})
	hardware := cmp.Or(a.inv.Hardware, &jamf.ComputerHardwareSection{})
	log(ctx,
		"Syncing Jamf computer",
		"general.platform", general.Platform,
		"hardware.serialNumber", hardware.SerialNumber,
		"id", a.inv.ID,
		"general.reportDate", general.ReportDate,
		"general.lastContactTime", general.LastContactTime,
		"general.lastEnrolledDate", general.LastEnrolledDate,
		"profile", dev.Profile,
	)
	dev.Profile.OsUsernames = osUsernames
}
