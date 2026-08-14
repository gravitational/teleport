package service

import (
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
	sortByLastInventoryUpdateDateDesc = "lastInventoryUpdateDate:desc"
)

func (s *S) verifyMobileInventoryFilters(ctx context.Context, entry *types.JamfInventoryEntry) error {
	if _, err := s.jamf.GetMobileDevicesDetail(ctx, &jamf.GetMobileDevicesDetailRequest{
		Page:     0,
		PageSize: 1,
		Sort:     []string{sortByLastInventoryUpdateDateDesc},
		Filter:   entry.FilterRsql,
	}); err != nil {
		return trace.BadParameter("mobile device inventory query, filter=%q: %v", entry.FilterRsql, err)
	}
	return nil
}

// getMobilePage reads a single page of mobile device inventory from Jamf.
func (s *S) getMobilePage(
	ctx context.Context, spec RunSpec, pageNum int,
) (rawJamfPage, error) {
	req := &jamf.GetMobileDevicesDetailRequest{
		Section: []string{
			jamf.MobileDeviceSectionGeneral,
			jamf.MobileDeviceSectionHardware,
		},
		PageSize: spec.PageSize,
		Page:     pageNum,
		Sort:     []string{"mobileDeviceId:asc"},
		Filter:   spec.FilterRSQL,
	}

	if spec.Mode == mdmsync.SyncModePartial {
		req.Sort = []string{sortByLastInventoryUpdateDateDesc}
	}

	resp, err := s.jamf.GetMobileDevicesDetail(ctx, req)
	if err != nil {
		return rawJamfPage{}, trace.Wrap(err, "jamf mobile device read failed")
	}
	devices := make([]jamfDevice, len(resp.Results))
	for i, md := range resp.Results {
		devices[i] = mobileAdapter{md}
	}
	return rawJamfPage{devices: devices, totalCount: resp.TotalCount}, nil
}

// confirmMobile checks whether a mobile device still exists in Jamf and matches
// the expected OS type and serial number. Returns (true, device, nil) if the
// device is found and matches.
func (s *S) confirmMobile(ctx context.Context, dev *devicepb.Device) (matches bool, jamfDevForLogging any, err error) {
	md, err := s.jamf.GetMobileDeviceByID(ctx, &jamf.GetMobileDeviceByIDRequest{
		ID: dev.GetProfile().GetExternalId(),
	})
	if err != nil {
		return false, nil, err
	}
	matches = md.SerialNumber == dev.GetAssetTag() &&
		md.IOS != nil &&
		mobileToOSType(md.Type, md.IOS.ModelIdentifier) == dev.GetOsType()
	return matches, md, nil
}

func mobileToDevice(d *jamf.MobileDevice) (*devicepb.Device, error) {
	if d == nil {
		return nil, trace.BadParameter("mobile device is nil")
	}

	// Hardware has the SerialNumber and ModelIdentifier.
	// DeviceType + ModelIdentifier determine the OS type.
	// That's the bare minimum we need, everything else is DeviceProfile info.
	if d.Hardware == nil {
		return nil, trace.BadParameter("mobile device has no hardware section")
	}

	osType := mobileToOSType(d.DeviceType, d.Hardware.ModelIdentifier)
	if osType == devicepb.OSType_OS_TYPE_UNSPECIFIED {
		return nil, trace.BadParameter("unexpected deviceType=%q, hardware.modelIdentifier=%q", d.DeviceType, d.Hardware.ModelIdentifier)
	}

	profile := devicepb.DeviceProfile_builder{
		ModelIdentifier: d.Hardware.ModelIdentifier,
		ExternalId:      d.MobileDeviceID,
	}.Build()
	if d.General != nil {
		profile.SetOsVersion(d.General.OSVersion)
		profile.SetOsBuild(d.General.OSBuild)
		profile.SetOsBuildSupplemental(d.General.OSSupplementalBuildVersion)
	}

	return devicepb.Device_builder{
		OsType:   osType,
		AssetTag: d.Hardware.SerialNumber,
		Profile:  profile,
	}.Build(), nil
}

// mobileToOSType maps a Jamf mobile device's deviceType and modelIdentifier to
// a Teleport OSType. The API reports "iOS" for both iPhones and iPads, so we
// use the modelIdentifier prefix to distinguish them.
func mobileToOSType(deviceType, modelIdentifier string) devicepb.OSType {
	if !strings.EqualFold("iOS", deviceType) {
		return devicepb.OSType_OS_TYPE_UNSPECIFIED
	}

	modelIDLower := strings.ToLower(modelIdentifier)
	switch {
	case strings.HasPrefix(modelIDLower, "iphone"):
		return devicepb.OSType_OS_TYPE_IOS
	case strings.HasPrefix(modelIDLower, "ipad"):
		return devicepb.OSType_OS_TYPE_IPADOS
	default:
		return devicepb.OSType_OS_TYPE_UNSPECIFIED
	}
}

type mobileAdapter struct {
	md *jamf.MobileDevice
}

func (a mobileAdapter) toDevice() (*devicepb.Device, error) {
	return mobileToDevice(a.md)
}

func (a mobileAdapter) cutTime() time.Time {
	if a.md != nil && a.md.General != nil {
		return a.md.General.LastInventoryUpdateDate
	}
	return time.Time{}
}

func (a mobileAdapter) platform() string {
	return a.md.DeviceType
}

func (a mobileAdapter) serialNumber() string {
	if a.md.Hardware != nil {
		return a.md.Hardware.SerialNumber
	}
	return ""
}

func (a mobileAdapter) logSync(ctx context.Context, log logFunc, dev *devicepb.Device) {
	var serialNumber string
	if a.md.Hardware != nil {
		serialNumber = a.md.Hardware.SerialNumber
	}
	var lastInventoryUpdateDate time.Time
	if a.md.General != nil {
		lastInventoryUpdateDate = a.md.General.LastInventoryUpdateDate
	}
	log(ctx,
		"Syncing Jamf mobile device",
		"deviceType", a.md.DeviceType,
		"hardware.serialNumber", serialNumber,
		"mobileDeviceId", a.md.MobileDeviceID,
		"lastInventoryUpdateDate", lastInventoryUpdateDate,
		"profile", dev.GetProfile(),
	)
}
