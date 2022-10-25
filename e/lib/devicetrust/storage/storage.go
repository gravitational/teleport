package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	log "github.com/sirupsen/logrus"
)

const currentAPIVersion = "v1"

// GetBackendFunc is a function that returns a backend.Backend implementation.
type GetBackendFunc func() backend.Backend

// S implements the Device Trust storage, backed by a backend.Backend.
type S struct {
	logger  *log.Entry
	backend GetBackendFunc
}

// New returns a new Device Trust storage instance.
func New(getBackend GetBackendFunc) (*S, error) {
	if getBackend == nil {
		return nil, trace.BadParameter("getBackend required")
	}

	return &S{
		logger:  log.WithField(trace.Component, "devicetrust.storage"),
		backend: getBackend,
	}, nil
}

// CreateDevice creates a new Device in storage and updates the necessary
// indexes (such as the asset tag index).
// Returns the stored device.
func (s *S) CreateDevice(ctx context.Context, dev *devicepb.Device) (*devicepb.Device, error) {
	if err := validateForCreate(dev); err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal device to start, just in the extremely unlikely case that it fails.
	now := time.Now().UTC()
	stored := &storedDevice{
		OSType:     int(dev.OsType),
		AssetTag:   dev.AssetTag,
		CreateTime: now,
		UpdateTime: now,
	}
	storedJSON, err := json.Marshal(stored)
	if err != nil {
		return nil, trace.Wrap(err, "marshal device")
	}

	// Disallow duplicate asset tags for the same OS. This naturally ignores "hanging"
	// asset tag mappings while providing a modicum of consistency - ideally we'd
	// have a true unique constraint.
	sameTagDevs, err := s.GetDevicesByAssetTag(ctx, stored.AssetTag)
	if err != nil {
		return nil, trace.Wrap(err, "verifying asset tag uniqueness")
	}
	for _, other := range sameTagDevs {
		if other.OsType == dev.OsType {
			return nil, trace.AlreadyExists("asset tag already registered")
		}
	}

	deviceID := uuid.NewString()

	// Create/update asset tag index.
	// It's OK to leave the asset tag mapping behind if writing the device fails.
	if err := s.updateAssetTagIndex(ctx, stored.AssetTag, &deviceRef{
		DeviceID: deviceID,
		OSType:   stored.OSType,
	}); err != nil {
		return nil, trace.Wrap(err, "update asset tag index")
	}

	// Write device.
	if _, err := s.backend().Put(ctx, backend.Item{
		Key:   deviceKey(deviceID),
		Value: storedJSON,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return storedToDevice(deviceID, stored), nil
}

func validateForCreate(d *devicepb.Device) error {
	switch {
	case d == nil:
		return trace.BadParameter("device required")
	case d.OsType == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return trace.BadParameter("unknown or invalid os_type")
	case d.AssetTag == "":
		return trace.BadParameter("asset_tag required")
	}
	return nil
}

func (s *S) updateAssetTagIndex(ctx context.Context, assetTag string, ref *deviceRef) error {
	logger := s.logger.WithFields(log.Fields{
		"DeviceID": ref.DeviceID,
		"OSType":   ref.OSType,
		"AssetTag": assetTag,
	})

	assetTagKey := devicesByAssetTagKey(assetTag)
	var lastErr error
	const maxAttempts = 3 // arbitrary
	for i := 0; i < maxAttempts; i++ {
		var retry bool
		current, getErr := s.backend().Get(ctx, assetTagKey)
		switch {
		case trace.IsNotFound(getErr): // New asset tag
			retry, lastErr = s.putDeviceRef(ctx, assetTagKey, ref)
			if lastErr != nil {
				logger.WithError(lastErr).Debug("Failed to write new asset tag mapping, retrying")
			}

		case getErr == nil: // Existing asset tag
			retry, lastErr = s.appendDeviceRef(ctx, current, ref)
			if lastErr != nil {
				logger.WithError(getErr).Debug("Failed to append to asset tag mapping, retrying")
			}

		default: // getErr != nil
			logger.WithError(getErr).Warn("Unexpected error reading asset tag mapping, retrying")
			retry = true
			lastErr = getErr
		}

		switch {
		case lastErr == nil: // Update OK
			return nil
		case !retry:
			return trace.Wrap(getErr)
		}
	}

	return trace.Wrap(lastErr)
}

func (s *S) putDeviceRef(ctx context.Context, key []byte, ref *deviceRef) (retryable bool, err error) {
	val, err := json.Marshal(&devicesRef{
		Devices: []*deviceRef{ref},
	})
	if err != nil {
		return false, trace.Wrap(err, "marshal device reference")
	}

	_, err = s.backend().Put(ctx, backend.Item{
		Key:   key,
		Value: val,
	})
	if err != nil {
		return true, trace.Wrap(err)
	}
	return false, nil
}

func (s *S) appendDeviceRef(ctx context.Context, current *backend.Item, ref *deviceRef) (retryable bool, err error) {
	refs := &devicesRef{}
	if err := json.Unmarshal(current.Value, refs); err != nil {
		return false, trace.Wrap(err, "unmarshal device references")
	}

	// Is the device already mapped? Nothing to do in that case.
	for _, existing := range refs.Devices {
		if existing.DeviceID == ref.DeviceID {
			return false, nil
		}
	}
	refs.Devices = append(refs.Devices, ref)

	val, err := json.Marshal(refs)
	if err != nil {
		return false, trace.Wrap(err, "marshal device references")
	}

	_, err = s.backend().CompareAndSwap(ctx, *current, backend.Item{
		Key:   current.Key,
		Value: val,
	})
	if err != nil {
		return true, trace.Wrap(err)
	}
	return false, nil
}

// GetDeviceByID reads a device by ID.
// Returns the stored device or trace.NotFound.
func (s *S) GetDeviceByID(ctx context.Context, deviceID string) (*devicepb.Device, error) {
	item, err := s.backend().Get(ctx, deviceKey(deviceID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stored := &storedDevice{}
	if err := json.Unmarshal(item.Value, stored); err != nil {
		return nil, trace.Wrap(err, "unmarshal device")
	}

	return storedToDevice(deviceIDFromKey(item.Key), stored), nil
}

// GetDevicesByAssetTag reads devices by asset tag.
// Returns an empty slice if no devices are found.
func (s *S) GetDevicesByAssetTag(ctx context.Context, assetTag string) ([]*devicepb.Device, error) {
	item, err := s.backend().Get(ctx, devicesByAssetTagKey(assetTag))
	switch {
	case trace.IsNotFound(err):
		return nil, nil
	case err != nil:
		return nil, trace.Wrap(err)
	}

	refs := &devicesRef{}
	if err := json.Unmarshal(item.Value, refs); err != nil {
		return nil, trace.Wrap(err, "unmarshal device references")
	}

	// In practice devices with the same asset tag are expected to be rare (eg,
	// one per OS), but let's set a limit to be safe.
	maxActiveGoroutines := len(devicepb.OSType_name)

	// Fetch devices in parallel.
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxActiveGoroutines)
	devicesC := make(chan *devicepb.Device, len(refs.Devices))
	queried := make(map[string]struct{})
	for _, ref := range refs.Devices {
		ref := ref

		// Skip duplicate devices, in case the index has them.
		if _, ok := queried[ref.DeviceID]; ok {
			continue
		}
		queried[ref.DeviceID] = struct{}{}

		g.Go(func() error {
			dev, err := s.GetDeviceByID(ctx, ref.DeviceID)
			if trace.IsNotFound(err) {
				// OK, ignore "hanging" asset tag mappings.
				// It can happen in some CreateDevice failure scenarios.
				// TODO(codingllama): Attempt to clean "hanging" asset tag mappings.
				err = nil
			}
			devicesC <- dev
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	close(devicesC) // Safe, all goroutines returned by this point.
	res := make([]*devicepb.Device, 0, len(refs.Devices))
	for dev := range devicesC {
		res = append(res, dev)
	}

	return res, nil
}

func deviceIDFromKey(key []byte) string {
	idx := bytes.LastIndexByte(key, backend.Separator)
	return string(key[idx+1:])
}

func storedToDevice(deviceID string, sd *storedDevice) *devicepb.Device {
	var cred *devicepb.DeviceCredential
	if c := sd.Credential; c != nil {
		cred = &devicepb.DeviceCredential{
			Id:           c.ID,
			PublicKeyDer: c.PublicKeyDER,
		}
	}
	return &devicepb.Device{
		ApiVersion:   currentAPIVersion,
		Id:           deviceID,
		OsType:       devicepb.OSType(sd.OSType),
		AssetTag:     sd.AssetTag,
		CreateTime:   timestamppb.New(sd.CreateTime),
		UpdateTime:   timestamppb.New(sd.UpdateTime),
		EnrollStatus: devicepb.DeviceEnrollStatus(sd.EnrollStatus),
		Credential:   cred,
	}
}

func deviceKey(deviceID string) []byte {
	return backend.Key("devices", "id", deviceID)
}

func devicesByAssetTagKey(assetTag string) []byte {
	return backend.Key("devices", "byTag", assetTag)
}
