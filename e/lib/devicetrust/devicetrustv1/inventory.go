package devicetrustv1

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
)

type inventorySyncer struct {
	storage *storage.S

	createCallback,
	updateCallback,
	noopCallback,
	deleteCallback func(source *devicepb.DeviceSource, dev *devicepb.Device, err error)
}

type osTypeSet map[devicepb.OSType]struct{}

// SyncInventory executes its namesake stream.
//
// The `implicitSource` is the source acquired from an MDM Service certificate,
// if present. If absent, then the source supplied in the start message is used.
//
// Audit callbacks are invoked as appropriate, regardless of the outcome (`dev`
// may be nil and `err` may be non-nil). The exception are successful noop
// updates, which don't generate audit calls.
//
// Authorization checks are the responsibility of the caller.
func (s *inventorySyncer) SyncInventory(stream devicepb.DeviceTrustService_SyncInventoryServer) error {
	// Start step.
	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	startReq := req.GetStart()
	if startReq == nil {
		return trace.BadParameter("first message must be SyncInventoryStart")
	}

	// start: Validate source.
	source := startReq.Source
	if err := storage.ValidateDeviceSource(source); err != nil {
		return trace.Wrap(err, "start: source")
	}

	// start: Ack.
	if err := stream.Send(&devicepb.SyncInventoryResponse{
		Payload: &devicepb.SyncInventoryResponse_Ack{
			Ack: &devicepb.SyncInventoryAck{},
		},
	}); err != nil {
		return trace.Wrap(err)
	}

	ctx := stream.Context()

	// allowedOSTypes is the set of OS types this sync covers. An empty OsTypes
	// falls back to the "computer" OS types only for backwards compatibility.
	osTypes := startReq.OsTypes
	if len(osTypes) == 0 {
		osTypes = []devicepb.OSType{
			devicepb.OSType_OS_TYPE_MACOS,
			devicepb.OSType_OS_TYPE_WINDOWS,
			devicepb.OSType_OS_TYPE_LINUX,
		}
	}
	allowedOSTypes := make(osTypeSet, len(osTypes))
	for _, t := range osTypes {
		allowedOSTypes[t] = struct{}{}
	}

	// Tracking for "missing" devices.
	type deviceKey struct {
		osType   devicepb.OSType
		assetTag string
	}
	seenDevices := make(map[deviceKey]struct{})
	updateSeen := func(devs []*devicepb.Device) {
		if !startReq.TrackMissingDevices {
			return
		}

		for _, dev := range devs {
			if dev == nil || dev.OsType == devicepb.OSType_OS_TYPE_UNSPECIFIED || dev.AssetTag == "" {
				continue
			}

			seenDevices[deviceKey{
				osType:   dev.OsType,
				assetTag: dev.AssetTag,
			}] = struct{}{}
		}
	}

	// Devices step.
Devices:
	for {
		req, err = stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}

		var statuses []*devicepb.DeviceOrStatus
		var err error
		switch req := req.Payload.(type) {
		case *devicepb.SyncInventoryRequest_End:
			break Devices
		case *devicepb.SyncInventoryRequest_DevicesToUpsert:
			devs := req.DevicesToUpsert.GetDevices()
			statuses, err = s.upsertDevices(ctx, source, devs, allowedOSTypes)
			// err handled below.

			// Mark all devices as seen, regardless of outcome.
			// We don't want an Update failure to cause a device to be deleted.
			updateSeen(devs)
		case *devicepb.SyncInventoryRequest_DevicesToRemove:
			statuses, err = s.deleteDevices(ctx, source, req.DevicesToRemove.GetDevices(), allowedOSTypes)
			// err handled below.
		default:
			return trace.BadParameter("unexpected payload type %T during devices phase", req)
		}
		if err != nil {
			return trace.Wrap(err)
		}

		if err := stream.Send(&devicepb.SyncInventoryResponse{
			Payload: &devicepb.SyncInventoryResponse_Result{
				Result: &devicepb.SyncInventoryResult{
					Devices: statuses,
				},
			},
		}); err != nil {
			return trace.Wrap(err)
		}
	}

	// End step.
	// If we are not tracking missing devices, or saw zero devices, then simply
	// return.
	// This is a safeguard against client-side failures.
	if len(seenDevices) == 0 {
		return nil
	}

	// end: Find missing devices.
	const pageSize = 0 // use default
	const missingDevicesBatchSize = 200
	var pageToken string
	var missingDevs []*devicepb.Device
	for {
		// Use RESOURCE view so we get the Source back.
		devs, nextPageToken, err := s.storage.ListDevices(ctx, pageSize, pageToken, devicepb.DeviceView_DEVICE_VIEW_RESOURCE)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, dev := range devs {
			// Is the device managed by the source?
			if !sourcesMatch(dev.Source, source) {
				continue
			}

			// Is the device's OsType in the sync's declared set?
			if _, ok := allowedOSTypes[dev.OsType]; !ok {
				continue
			}

			// Did we see the device previously in the sync?
			key := deviceKey{
				osType:   dev.OsType,
				assetTag: dev.AssetTag,
			}
			if _, seen := seenDevices[key]; seen {
				continue
			}

			// Record missing device.
			// Control which fields are sent, so we have leeway to change the
			// implementation.
			missingDevs = append(missingDevs, &devicepb.Device{
				Id:       dev.Id,
				OsType:   dev.OsType,
				AssetTag: dev.AssetTag,
				Profile: &devicepb.DeviceProfile{
					ExternalId: dev.Profile.GetExternalId(),
				},
			})

			if len(missingDevs) >= missingDevicesBatchSize {
				if err := s.handleMissingDevices(ctx, stream, source, missingDevs, allowedOSTypes); err != nil {
					return trace.Wrap(err)
				}
				missingDevs = nil
			}
		}

		if nextPageToken == "" {
			break
		}
		pageToken = nextPageToken
	}

	// Handle remaining missing devices from above.
	return trace.Wrap(s.handleMissingDevices(ctx, stream, source, missingDevs, allowedOSTypes))
}

func (s *inventorySyncer) upsertDevices(
	ctx context.Context,
	source *devicepb.DeviceSource,
	devs []*devicepb.Device,
	allowedOSTypes osTypeSet,
) ([]*devicepb.DeviceOrStatus, error) {
	if len(devs) == 0 {
		return nil, nil
	}

	// Allow for some parallelism when writing devices.
	// Enough to make a difference, but not enough to negatively impact the
	// backend.
	const upsertLimit = 4
	g := &errgroup.Group{}
	g.SetLimit(upsertLimit)

	var mu sync.Mutex // mu guards statuses
	statuses := make([]*devicepb.DeviceOrStatus, len(devs))
	setStatus := func(i int, s *devicepb.DeviceOrStatus) {
		mu.Lock()
		statuses[i] = s
		mu.Unlock()
	}

	for i, dev := range devs {
		if err := validateOSType(allowedOSTypes, dev.OsType); err != nil {
			setStatus(i, &devicepb.DeviceOrStatus{Status: errToStatus(err)})
			continue
		}

		// Avoid querying clearly-invalid devices.
		const createAsResource = false
		if err := storage.ValidateDeviceForCreate(dev, createAsResource); err != nil {
			setStatus(i, &devicepb.DeviceOrStatus{
				Status: errToStatus(err),
			})
			continue
		}

		// Synced devices always use the "global" source.
		dev.Source = source

		i := i
		dev := dev
		g.Go(func() error {
			// Find if the device exists.
			// A non-empty ID is not a guarantee that the device exists, as indexes
			// can have leftover data, but it's a strong sign that it does.
			deviceID, err := s.storage.GetDeviceIDByOSTag(ctx, dev.OsType, dev.AssetTag, false /* verifyExistence */)
			// err handled below.

			// Attempt Update first.
			var stored *devicepb.Device
			if err == nil {
				var prevUpdateTime *timestamppb.Timestamp
				stored, err = s.storage.UpdateDevice(ctx, deviceID, func(stored *devicepb.Device) *devicepb.Device {
					prevUpdateTime = stored.UpdateTime

					// Copy system-managed fields and fields that sync can't, by
					// definition, change.
					// Everything else we take from the sync device.
					dev.ApiVersion = stored.ApiVersion
					dev.Id = stored.Id
					dev.CreateTime = stored.CreateTime
					dev.UpdateTime = stored.UpdateTime
					dev.EnrollStatus = stored.EnrollStatus
					dev.Credential = stored.Credential
					dev.Owner = stored.Owner
					return dev
				})
				// err handled below

				// Notify noops separately from updates, they are needless noise for
				// audit but interesting for metrics.
				if err == nil && proto.Equal(prevUpdateTime, stored.GetUpdateTime()) {
					s.noopCallback(source, stored, err)
				} else {
					s.updateCallback(source, stored, err)
				}
			}
			// Attempt Create if either GetDeviceIDByOSTag or UpdateDevice failed with
			// not found.
			if trace.IsNotFound(err) {
				stored, err = s.storage.CreateDevice(ctx, dev, createAsResource)
				s.createCallback(source, stored, err)
				// err handled below.
			}

			setStatus(i, &devicepb.DeviceOrStatus{
				Status: errToStatus(err),
				Id:     stored.GetId(), // only present on success.
			})

			return nil
		})
	}

	_ = g.Wait() // Inner goroutines never error.

	return statuses, nil
}

func (s *inventorySyncer) deleteDevices(
	ctx context.Context,
	source *devicepb.DeviceSource,
	devs []*devicepb.Device,
	allowedOSTypes osTypeSet,
) ([]*devicepb.DeviceOrStatus, error) {
	statuses := make([]*devicepb.DeviceOrStatus, len(devs))
	for i, dev := range devs {
		st := &devicepb.DeviceOrStatus{}
		statuses[i] = st

		if err := validateOSType(allowedOSTypes, dev.OsType); err != nil {
			st.Status = errToStatus(err)
			continue
		}

		// Either Id or (OsType,AssetTag) must be present for the device to be
		// identified.
		hasID := dev.Id != ""
		hasOSTag := dev.OsType != devicepb.OSType_OS_TYPE_UNSPECIFIED && dev.AssetTag != ""
		if !hasID && !hasOSTag {
			st.Status = &spb.Status{
				Code:    int32(codes.InvalidArgument),
				Message: "device has no identifiers (id or os_type+asset_tag)",
			}
			continue
		}

		// Query device ID?
		// Assign the queried ID to the device itself, it's useful for audit below.
		if dev.Id == "" {
			var err error
			dev.Id, err = s.storage.GetDeviceIDByOSTag(ctx, dev.OsType, dev.AssetTag, false /* verifyExistence */)
			if err != nil {
				st.Status = errToStatus(err)
				continue
			}
		}

		// Delete.
		err := s.storage.DeleteDevicePredicate(ctx, dev.Id, func(stored *devicepb.Device) error {
			switch {
			// If multiple identifiers are provided, make sure all of them match.
			case dev.AssetTag != "" && dev.AssetTag != stored.AssetTag,
				dev.OsType != devicepb.OSType_OS_TYPE_UNSPECIFIED && dev.OsType != stored.OsType:
				return trace.BadParameter("device identifiers don't match the same device (id vs os_type+asset_tag)")
			// Source must match.
			case !sourcesMatch(stored.Source, source):
				return trace.BadParameter("device is owned by another source")
			default:
				return nil
			}
		})
		s.deleteCallback(source, dev, err)
		if err == nil {
			st.Id = dev.Id
			st.Deleted = true
		} else {
			st.Status = errToStatus(err)
		}
	}
	return statuses, nil
}

// handleMissingDevices sends the missing devices `devs` to the client, handles
// the remove response, and reports the result of the removals.
func (s *inventorySyncer) handleMissingDevices(
	ctx context.Context,
	stream devicepb.DeviceTrustService_SyncInventoryServer,
	source *devicepb.DeviceSource,
	missingDevs []*devicepb.Device,
	allowedOSTypes osTypeSet,
) error {
	if len(missingDevs) == 0 {
		return nil
	}

	// Notify missing devices.
	if err := stream.Send(&devicepb.SyncInventoryResponse{
		Payload: &devicepb.SyncInventoryResponse_MissingDevices{
			MissingDevices: &devicepb.SyncInventoryMissingDevices{
				Devices: missingDevs,
			},
		},
	}); err != nil {
		return trace.Wrap(err)
	}

	// Deletion requests (may be empty).
	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	removeReq := req.GetDevicesToRemove()
	if removeReq == nil {
		return trace.BadParameter(
			"unexpected payload type %T during missing devices phase, expected devices_to_remove",
			req.Payload)
	}

	// Verify that deletion requests have an ID and are contained in the set of
	// missing devices.
	// In principle we don't want this step to be used for arbitrary deletions.
	missingDevIDs := make(map[string]struct{})
	for _, missing := range missingDevs {
		missingDevIDs[missing.Id] = struct{}{}
	}
	devicesToRemove := removeReq.GetDevices()
	for _, dev := range devicesToRemove {
		_, ok := missingDevIDs[dev.Id]
		if dev.Id == "" || !ok {
			// We could fail this particular device, instead of failing the entire
			// stream, but this is likely a programming error and failing the stream
			// will make that much clearer.
			return trace.BadParameter("device %+v is not in the current set of missing devices", dev)
		}
	}

	statuses, err := s.deleteDevices(ctx, source, removeReq.GetDevices(), allowedOSTypes)
	if err != nil {
		return trace.Wrap(err)
	}

	// Report deletion results.
	return trace.Wrap(stream.Send(&devicepb.SyncInventoryResponse{
		Payload: &devicepb.SyncInventoryResponse_Result{
			Result: &devicepb.SyncInventoryResult{
				Devices: statuses,
			},
		},
	}))
}

func errToStatus(err error) *spb.Status {
	return status.Convert(trail.ToGRPC(err)).Proto()
}

// validateOSType returns an error if osType is set and not in allowed.
// An UNSPECIFIED osType is accepted so that downstream validation can
// produce its specific "missing identifiers" error rather than being
// masked by an os_types rejection.
func validateOSType(allowed osTypeSet, osType devicepb.OSType) error {
	if osType == devicepb.OSType_OS_TYPE_UNSPECIFIED {
		return nil
	}
	if _, ok := allowed[osType]; ok {
		return nil
	}
	return trace.BadParameter("device os_type %v not in declared os_types", osType)
}

func sourcesMatch(s1, s2 *devicepb.DeviceSource) bool {
	if s1 == nil || s2 == nil {
		return s1 == s2
	}
	return s1.Name == s2.Name && s1.Origin == s2.Origin
}
