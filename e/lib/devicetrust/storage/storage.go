package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/backend"
)

const (
	// MaxCollectedDataPerDevice is the maximum number of (non-enrollment)
	// collected data, for each device.
	MaxCollectedDataPerDevice = 10

	// DeviceEnrollTokenExpireDuration is the default expiration for enrollment
	// tokens.
	DeviceEnrollTokenExpireDuration = 1 * time.Hour
)

const (
	currentAPIVersion = "v1"

	// enrollmentDataID is the fixed ID used enrollment collected data.
	enrollmentDataID = "1"
)

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

func (s *S) nowUTC() time.Time {
	return s.backend().Clock().Now().UTC()
}

// BulkCreateDevices creates devices in bulk.
// Returns, for each device, a DeviceOrStatus with a non-empty ID in case of
// success, or a failure Status in case of error. The response is guaranteed to
// have the same ordering as the input.
func (s *S) BulkCreateDevices(ctx context.Context, devs []*devicepb.Device) []*devicepb.DeviceOrStatus {
	errToStatus := func(err error) *statuspb.Status {
		return status.Convert(trail.ToGRPC(err)).Proto()
	}

	resp := make([]*devicepb.DeviceOrStatus, len(devs)) // same order as devs
	seenTags := make(map[assetTagKey]struct{})
	for i, dev := range devs {
		resp[i] = &devicepb.DeviceOrStatus{}

		// Is the device valid?
		if err := validateForCreate(dev); err != nil {
			resp[i].Status = errToStatus(err)
			continue
		}

		// Is the tag repeated within devs?
		tag := assetTagKey{
			osType:   dev.OsType,
			assetTag: dev.AssetTag,
		}
		if _, ok := seenTags[tag]; ok {
			resp[i].Status = errToStatus(trace.AlreadyExists("asset tag already requested"))
			continue
		}
		seenTags[tag] = struct{}{}
	}

	// Group used mainly for an upper bound in the number of goroutines.
	var g errgroup.Group
	const maxCreateGoroutines = 4
	g.SetLimit(maxCreateGoroutines)

	// mu guards resp in the block below.
	var mu sync.Mutex
	for i, dev := range devs {
		mu.Lock()
		ok := resp[i].Status.GetCode() == int32(codes.OK)
		mu.Unlock()
		if !ok {
			continue // Errored on pre-validation.
		}

		i := i
		dev := dev
		g.Go(func() error {
			created, err := s.createDevice(ctx, dev)
			mu.Lock()
			resp[i].Status = errToStatus(err)
			resp[i].Id = created.GetId()
			mu.Unlock()
			return nil
		})
	}

	// Error swallowed on purpose, we record errors in the response itself.
	_ = g.Wait()

	return resp
}

// assetTagKey is used to detect duplicate tags.
type assetTagKey struct {
	osType   devicepb.OSType
	assetTag string
}

// CreateDevice creates a new Device in storage and updates the necessary
// indexes (such as the asset tag index).
// Returns the stored device.
// Prefer using BulkCreateDevices if you want to create multiple devices
// concurrently.
func (s *S) CreateDevice(ctx context.Context, dev *devicepb.Device) (*devicepb.Device, error) {
	if err := validateForCreate(dev); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.createDevice(ctx, dev)
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

func (s *S) createDevice(ctx context.Context, dev *devicepb.Device) (*devicepb.Device, error) {
	// Marshal device to start, just in the extremely unlikely case that it fails.
	now := s.nowUTC()
	stored := &storedDevice{
		OSType:       int(dev.OsType),
		AssetTag:     dev.AssetTag,
		CreateTime:   now,
		UpdateTime:   now,
		EnrollStatus: int(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED),
	}
	storedJSON, err := json.Marshal(stored)
	if err != nil {
		return nil, trace.Wrap(err, "marshal device")
	}

	deviceID := uuid.NewString()

	// Create/update asset tag index.
	// It's OK to leave the asset tag mapping behind if writing the device fails.
	ref := &deviceRef{
		DeviceID: deviceID,
		OSType:   stored.OSType,
	}
	if err := s.updateAssetTagIndex(ctx, stored.AssetTag, ref); err != nil {
		return nil, trace.Wrap(err)
	}

	// Write device.
	if _, err := s.backend().Create(ctx, backend.Item{
		Key:   deviceKey(deviceID),
		Value: storedJSON,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return storedToDevice(deviceID, stored), nil
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
			retry, lastErr = s.createDeviceRef(ctx, assetTagKey, ref)
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
			return trace.Wrap(lastErr)
		}
	}

	return trace.Wrap(lastErr)
}

func (s *S) createDeviceRef(ctx context.Context, key []byte, ref *deviceRef) (retryable bool, err error) {
	val, err := json.Marshal(&devicesRef{
		Devices: []*deviceRef{ref},
	})
	if err != nil {
		return false, trace.Wrap(err, "marshal device reference")
	}

	if _, err := s.backend().Create(ctx, backend.Item{
		Key:   key,
		Value: val,
	}); err != nil {
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
		if existing.OSType == ref.OSType {
			// Does the device _really_ exist?
			// Let's not have a hanging mapping inutilize an asset tag.
			if _, getErr := s.backend().Get(ctx, deviceKey(existing.DeviceID)); getErr == nil {
				return false, trace.AlreadyExists("asset tag already registered")
			}

			// We either found a hanging mapping or there is a race on CreateDevice.
			// Let both tags be, admins can clear duplicate devices manually.
			s.logger.WithFields(log.Fields{
				"AssetTag":   deviceIDFromKey(current.Key),
				"ExistingID": existing.DeviceID,
				"NewID":      ref.DeviceID,
			}).Warn("Found possible duplicate on asset tag mapping")
		}
	}
	refs.Devices = append(refs.Devices, ref)

	val, err := json.Marshal(refs)
	if err != nil {
		return false, trace.Wrap(err, "marshal device references")
	}

	if _, err := s.backend().CompareAndSwap(ctx, *current, backend.Item{
		Key:   current.Key,
		Value: val,
	}); err != nil {
		return true, trace.Wrap(err)
	}
	return false, nil
}

// DeleteDevice hard-deletes a device from storage.
func (s *S) DeleteDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return trace.BadParameter("device ID required")
	}

	// Read the device first, we need the asset tag for the cleanup below.
	dev, err := s.GetDeviceByID(ctx, deviceID)
	if err != nil {
		return trace.Wrap(err)
	}

	// Delete the device.
	// If this succeeds the invocation is considered a success: the device key is
	// the source of truth for a device existing, the system can handle "hanging"
	// asset tags.
	if err := s.backend().Delete(ctx, deviceKey(deviceID)); err != nil {
		return trace.Wrap(err)
	}

	// Remove asset tag mapping.
	if err := s.removeFromAssetTagIndex(ctx, deviceID, dev.AssetTag); err != nil {
		s.logger.
			WithError(err).
			WithFields(log.Fields{
				"DeviceID": deviceID,
				"AssetTag": dev.AssetTag,
			}).
			Warn("Failed to remove asset tag mapping for device")
		// err swallowed on purpose.
	}

	// Remove enroll token, if present.
	if err := s.backend().Delete(ctx, deviceTokenKey(deviceID)); err != nil && !trace.IsNotFound(err) {
		s.logger.
			WithError(err).
			WithFields(log.Fields{
				"DeviceID": deviceID,
				"AssetTag": dev.AssetTag,
			}).
			Warn("Failed to remove enroll token for device")
		// err swallowed on purpose.
	}

	// Remove collected data.
	cdStart := collectedDataKeyStart(deviceID)
	cdEnd := backend.RangeEnd(cdStart)
	if err := s.backend().DeleteRange(ctx, cdStart, cdEnd); err != nil {
		s.logger.
			WithError(err).
			WithFields(log.Fields{
				"DeviceID": deviceID,
				"AssetTag": dev.AssetTag,
			}).
			Warn("Failed to remove collected data for device")
		// err swallowed on purpose.
	}

	return nil
}

func (s *S) removeFromAssetTagIndex(ctx context.Context, deviceID, assetTag string) error {
	item, err := s.backend().Get(ctx, devicesByAssetTagKey(assetTag))
	if err != nil {
		return trace.Wrap(err, "reading asset tag mapping")
	}

	refs := &devicesRef{}
	if err := json.Unmarshal(item.Value, refs); err != nil {
		return trace.Wrap(err, "unmarshal asset tag mapping")
	}

	// Is the device within the references?
	// It should be, but let's go light in the assumptions.
	found := false
	devs := refs.Devices
	for i := 0; i < len(devs); i++ {
		if devs[i].DeviceID != deviceID {
			continue
		}

		// Swap with last and cut from slice.
		last := len(devs) - 1
		devs[i], devs[last] = devs[last], nil
		devs = devs[:last]
		i--

		// Do not break here in case the device appears multiple times.
		// It shouldn't happen, but no harm in checking.
		found = true
	}
	if !found {
		return nil
	}
	refs.Devices = devs

	val, err := json.Marshal(refs)
	if err != nil {
		return trace.Wrap(err, "marshal asset tag mapping")
	}

	if _, err := s.backend().CompareAndSwap(ctx, *item, backend.Item{
		Key:   item.Key,
		Value: val,
	}); err != nil {
		return trace.Wrap(err, "writing asset tag mapping")
	}

	return nil
}

// GetDeviceByID reads a device by ID.
// Returns the stored device or trace.NotFound.
func (s *S) GetDeviceByID(ctx context.Context, deviceID string) (*devicepb.Device, error) {
	// Fetch collected data for the device asynchronously.
	cdCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cdC := make(chan []*devicepb.DeviceCollectedData)
	go func() {
		cd, err := s.getDeviceCollectedData(cdCtx, deviceID)
		if err != nil {
			s.logger.
				WithError(err).
				WithField("DeviceID", deviceID).
				Warn("Failed to fetch collected data for device")
		}
		cdC <- cd
	}()

	// Fetch the device.
	dev, _, _, err := s.getDeviceByID(ctx, deviceID)
	if err != nil {
		cancel() // Stop and wait for goroutine.
		<-cdC
		return nil, trace.Wrap(err)
	}

	// Add collected data to it.
	dev.CollectedData = <-cdC

	return dev, nil
}

// getDeviceByID is the internal version of GetDeviceByID.
// It returns all internal data structures along with the device.
func (s *S) getDeviceByID(ctx context.Context, deviceID string) (*devicepb.Device, *storedDevice, *backend.Item, error) {
	if deviceID == "" {
		return nil, nil, nil, trace.BadParameter("device ID required")
	}

	item, err := s.backend().Get(ctx, deviceKey(deviceID))
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	stored := &storedDevice{}
	if err := json.Unmarshal(item.Value, stored); err != nil {
		return nil, nil, nil, trace.Wrap(err, "unmarshal device")
	}

	dev := storedToDevice(deviceIDFromKey(item.Key), stored)
	return dev, stored, item, nil
}

func (s *S) getDeviceCollectedData(ctx context.Context, deviceID string) ([]*devicepb.DeviceCollectedData, error) {
	start := collectedDataKeyStart(deviceID)
	end := backend.RangeEnd(start)
	limit := MaxCollectedDataPerDevice * 2 // Give the search some leeway.
	res, err := s.backend().GetRange(ctx, start, end, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cd := make([]*devicepb.DeviceCollectedData, len(res.Items))
	for i, item := range res.Items {
		stored := &storedCollectedData{}
		if err := json.Unmarshal(item.Value, stored); err != nil {
			return nil, trace.Wrap(err)
		}
		cd[i] = &devicepb.DeviceCollectedData{
			CollectTime:  timestamppb.New(stored.CollectTime),
			RecordTime:   timestamppb.New(stored.RecordTime),
			OsType:       devicepb.OSType(stored.OSType),
			SerialNumber: stored.SerialNumber,
		}
	}

	// Sort by ascending RecordTime.
	sort.Slice(cd, func(i, j int) bool {
		d1 := cd[i]
		d2 := cd[j]
		if d1.RecordTime.Seconds == d2.RecordTime.Seconds {
			return d1.RecordTime.Nanos < d2.RecordTime.Nanos
		}
		return d1.RecordTime.Seconds < d2.RecordTime.Seconds
	})

	return cd, nil
}

// GetDevicesByAssetTag reads devices by asset tag.
// Returns an empty slice if no devices are found.
func (s *S) GetDevicesByAssetTag(ctx context.Context, assetTag string) ([]*devicepb.Device, error) {
	if assetTag == "" {
		return nil, trace.BadParameter("asset tag required")
	}

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
		// A hanging asset tag mapping could lead to a nil from device coming from
		// the channel.
		if dev != nil {
			res = append(res, dev)
		}
	}

	return res, nil
}

// ListDevices is a paginated search of devices. It returns the found devices
// and the page token for the next call.
//
// Use an empty pageToken to start the search and the returned nextPageToken for
// subsequent calls. A non-empty nextPageToken means that a next page is
// available (but may be empty), an empty nextPageToken signifies the end of the
// search. Callers are not expected to change other parameters in the same
// series of invocations.
//
// The requested pageSize is not guaranteed, as the server may change it at its
// discretion.
func (s *S) ListDevices(ctx context.Context, pageSize int, pageToken string, view devicepb.DeviceView) (devices []*devicepb.Device, nextPageToken string, err error) {
	if view == devicepb.DeviceView_DEVICE_VIEW_UNSPECIFIED {
		return nil, "", trace.BadParameter("view required")
	}

	startKey := deviceKeyStart()
	endKey := backend.RangeEnd(startKey)

	// The pageToken contains the ID of the last device returned. If it's present,
	// we start the search from it and skip that device in the results.
	var lastID string
	if pageToken != "" {
		var err error
		lastID, err = deviceIDFromPageToken(pageToken)
		if err != nil {
			return nil, "", trace.BadParameter("invalid page token")
		}
		startKey = deviceKey(lastID)
	}

	// Adjust page size, so it can't be too large.
	const maxPageSize = 200
	if pageSize <= 0 || pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	// Increment pageSize to allow for the extra item represented by lastID.
	// We skip this item in the results below.
	if lastID != "" {
		pageSize++
	}

	res, err := s.backend().GetRange(ctx, startKey, endKey, pageSize)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	for _, item := range res.Items {
		// The devices collection could shift between calls, so don't assume
		// anything about the position of lastID.
		deviceID := deviceIDFromKey(item.Key)
		if deviceID == lastID {
			continue
		}

		stored := &storedDevice{}
		if err := json.Unmarshal(item.Value, stored); err != nil {
			return nil, "", trace.Wrap(err)
		}
		devices = append(devices, storedToDeviceView(deviceID, stored, view))
	}

	// There can only be a next page if we got as many devices as we requested.
	if len(res.Items) == pageSize {
		lastDev := devices[len(devices)-1]
		nextPageToken, err = deviceIDToPageToken(lastDev.Id)
		if err != nil {
			return nil, "", trace.Wrap(err, "generating next page token")
		}
	}

	// Fetch collected data for all devices.
	if view == devicepb.DeviceView_DEVICE_VIEW_RESOURCE {
		const maxActiveGoroutines = 10
		var g errgroup.Group
		g.SetLimit(maxActiveGoroutines)

		for _, dev := range devices {
			dev := dev
			g.Go(func() error {
				cd, err := s.getDeviceCollectedData(ctx, dev.Id)
				if err != nil {
					s.logger.
						WithError(err).
						WithField("deviceID", dev.Id).
						Warn("Failed to fetch collected data for device")
					return nil // err swallowed on purpose
				}

				dev.CollectedData = cd
				return nil
			})
		}

		// Wait() should never error, as the goroutines don't themselves.
		_ = g.Wait()
	}

	return devices, nextPageToken, nil
}

type devicePageToken struct {
	ID string `json:"id"`
}

func deviceIDToPageToken(deviceID string) (string, error) {
	jsonToken, err := json.Marshal(&devicePageToken{
		ID: deviceID,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Encode resulting token to discourage external fiddling.
	return base64.StdEncoding.EncodeToString(jsonToken), nil
}

func deviceIDFromPageToken(pageToken string) (string, error) {
	decodedToken, err := base64.StdEncoding.DecodeString(pageToken)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var t devicePageToken
	if err := json.Unmarshal(decodedToken, &t); err != nil {
		return "", trace.Wrap(err)
	}
	return t.ID, nil
}

// EnrollDevice updates an existing device in storage, marking it as enrolled.
// Both device credential and collected data are required for the update.
// Returns the updated device, without collected data.
func (s *S) EnrollDevice(
	ctx context.Context,
	deviceID string, cred *devicepb.DeviceCredential, cd *devicepb.DeviceCollectedData) (*devicepb.Device, error) {
	if _, err := ValidateDeviceCredential(cred); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ValidateCollectedData(cd); err != nil {
		return nil, trace.Wrap(err)
	}

	dev, stored, item, err := s.getDeviceByID(ctx, deviceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ValidateCollectedDataAgainstDevice(cd, dev); err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal new device first, so we can exit in the off chance it errors.
	now := s.nowUTC()
	stored.UpdateTime = now
	stored.EnrollStatus = int(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED)
	stored.Credential = &storedDeviceCredential{
		ID:           cred.Id,
		PublicKeyDER: cred.PublicKeyDer,
	}
	val, err := json.Marshal(stored)
	if err != nil {
		return nil, trace.Wrap(err, "marshal device")
	}

	// Marshal and write collected data.
	if err := s.recordCollectedData(ctx, deviceID, cd, originEnrollment, now); err != nil {
		return nil, trace.Wrap(err)
	}

	// Update device.
	if _, err := s.backend().CompareAndSwap(ctx, *item, backend.Item{
		Key:   item.Key,
		Value: val,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return storedToDevice(deviceID, stored), nil
}

// RecordDeviceAuthnData records collected data gathered during device
// authentication.
// Authentication data is limited per-device; once the limit is reached, older
// data is discarded in favor of new data.
func (s *S) RecordDeviceAuthnData(ctx context.Context, deviceID string, cd *devicepb.DeviceCollectedData) error {
	if err := ValidateCollectedData(cd); err != nil {
		return trace.Wrap(err)
	}

	// Fetch device and verify data against it.
	dev, _, _, err := s.getDeviceByID(ctx, deviceID)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := ValidateCollectedDataAgainstDevice(cd, dev); err != nil {
		return trace.Wrap(err)
	}

	err = s.recordCollectedData(ctx, deviceID, cd, originAuthentication, s.nowUTC())
	return trace.Wrap(err)
}

func (s *S) recordCollectedData(ctx context.Context, deviceID string, cd *devicepb.DeviceCollectedData, origin collectedDataOrigin, recordTime time.Time) error {
	storedCD := &storedCollectedData{
		Origin:       origin,
		CollectTime:  cd.CollectTime.AsTime(),
		RecordTime:   recordTime,
		OSType:       int(cd.OsType),
		SerialNumber: cd.SerialNumber,
	}
	val, err := json.Marshal(storedCD)
	if err != nil {
		return trace.Wrap(err, "marshal collected data")
	}

	var cdID string
	if origin == originEnrollment {
		cdID = enrollmentDataID
	} else {
		cdID = uuid.NewString()
	}

	if _, err := s.backend().Put(ctx, backend.Item{
		Key:   collectedDataKey(deviceID, cdID),
		Value: val,
	}); err != nil {
		return trace.Wrap(err)
	}

	// Enrollment data doesn't count for the max data limit.
	if origin == originEnrollment {
		return nil
	}

	if err := s.clearCollectedDataIfNeeded(ctx, deviceID); err != nil {
		s.logger.
			WithError(err).
			WithField("deviceID", deviceID).
			Warn("Failed to clear collected data for device")
		// err swallowed on purpose, new data is already written.
	}

	return nil
}

// simplifiedCollectedData is used to decide which collected data entries to
// delete.
type simplifiedCollectedData struct {
	Key        []byte    `json:"-"`
	RecordTime time.Time `json:"record_time"`
}

func (s *S) clearCollectedDataIfNeeded(ctx context.Context, deviceID string) error {
	start := collectedDataKeyStart(deviceID)
	end := backend.RangeEnd(start)
	limit := MaxCollectedDataPerDevice * 2 // Give the search some leeway.
	res, err := s.backend().GetRange(ctx, start, end, limit)
	if err != nil {
		return trace.Wrap(err)
	}

	// Are we above the limit, ignoring enrollment data?
	if len(res.Items) <= MaxCollectedDataPerDevice+1 {
		return nil
	}

	cd := make([]simplifiedCollectedData, 0, len(res.Items))
	for _, item := range res.Items {
		if dataID := deviceIDFromKey(item.Key); dataID == enrollmentDataID {
			continue
		}

		cd = append(cd, simplifiedCollectedData{
			Key: item.Key,
		})
		scd := &(cd[len(cd)-1])
		if err := json.Unmarshal(item.Value, scd); err != nil {
			return trace.Wrap(err)
		}
	}
	sort.Slice(cd, func(i, j int) bool {
		d1 := cd[i]
		d2 := cd[j]
		return d1.RecordTime.Before(d2.RecordTime)
	})

	// From older to newer, delete data until we hit the size limit.
	for len(cd) > MaxCollectedDataPerDevice {
		if err := s.backend().Delete(ctx, cd[0].Key); err != nil {
			return trace.Wrap(err)
		}
		cd = cd[1:]
	}

	return nil
}

// CreateDeviceEnrollToken creates or replaces the existing enrollment token for
// a device. Only one enrollment token is allowed at a time.
//
// Enrollment tokens are tied to a particular device and expire in a reasonably
// short (for a human) amount of time.
//
// The plain token is not stored, instead it is meant to be sent out-of-band (as
// in outside of Teleport) to the person responsible for enrolling the device.
// SpendDeviceEnrollToken spends the token for the enrollment ceremony.
func (s *S) CreateDeviceEnrollToken(ctx context.Context, deviceID string) (*devicepb.DeviceEnrollToken, error) {
	if deviceID == "" {
		return nil, trace.BadParameter("device ID required")
	}

	// Device must exist, the easiest way to check is to read the key.
	if _, err := s.backend().Get(ctx, deviceKey(deviceID)); err != nil {
		return nil, trace.Wrap(err)
	}

	// Draw a few random bytes, base64 encode into a valid string and use the
	// resulting string as the password.
	// tokenPlain is sent to the client.
	// tokenHashed is written to storage.
	const tokenLen = 32
	tokenRaw := make([]byte, tokenLen)
	if _, err := rand.Read(tokenRaw); err != nil {
		return nil, trace.Wrap(err, "generating a new enrollment token")
	}
	tokenPlain := base64.RawStdEncoding.EncodeToString(tokenRaw)
	tokenHashed, err := bcrypt.GenerateFromPassword([]byte(tokenPlain), bcrypt.DefaultCost)
	if err != nil {
		return nil, trace.Wrap(err, "hashing enrollment token as a password")
	}

	val, err := json.Marshal(&storedEnrollToken{
		HashedToken: tokenHashed,
	})
	if err != nil {
		return nil, trace.Wrap(err, "marshal enrollment token")
	}

	if _, err := s.backend().Put(ctx, backend.Item{
		Key:     deviceTokenKey(deviceID),
		Value:   val,
		Expires: s.nowUTC().Add(DeviceEnrollTokenExpireDuration),
	}); err != nil {
		return nil, trace.Wrap(err, "writing enrollment token")
	}

	return &devicepb.DeviceEnrollToken{
		Token: tokenPlain,
	}, nil
}

// SpendDeviceEnrollToken spends an existing enrollment token, allowing the
// enrollment ceremony to proceed.
// The token is immediately spent in a positive match.
// Callers are encouraged to "erase" the resulting errors with a constant
// type/message, as to avoid leaking information about storage state.
func (s *S) SpendDeviceEnrollToken(ctx context.Context, deviceID, token string) error {
	switch {
	case deviceID == "":
		return trace.BadParameter("device ID required")
	case token == "":
		return trace.BadParameter("token required")
	}

	// Device must exist, the easiest way to check is to read the key.
	if _, err := s.backend().Get(ctx, deviceKey(deviceID)); err != nil {
		return trace.Wrap(err)
	}

	key := deviceTokenKey(deviceID)
	item, err := s.backend().Get(ctx, key)
	if err != nil {
		return trace.Wrap(err)
	}
	stored := &storedEnrollToken{}
	if err := json.Unmarshal(item.Value, stored); err != nil {
		return trace.Wrap(err, "unmarshal enrollment token")
	}

	if err := bcrypt.CompareHashAndPassword(stored.HashedToken, []byte(token)); err != nil {
		return trace.BadParameter("invalid token")
	}
	if err := s.backend().Delete(ctx, key); err != nil {
		return trace.Wrap(err, "failed to spend enrollment token")
	}

	return nil
}

func deviceIDFromKey(key []byte) string {
	idx := bytes.LastIndexByte(key, backend.Separator)
	return string(key[idx+1:])
}

func storedToDeviceView(deviceID string, sd *storedDevice, view devicepb.DeviceView) *devicepb.Device {
	// If "list" provide only basic device information.
	// Suitable for viewing multiple devices at once, as in "tctl devices ls".
	if view == devicepb.DeviceView_DEVICE_VIEW_LIST {
		return &devicepb.Device{
			ApiVersion:   currentAPIVersion,
			Id:           deviceID,
			OsType:       devicepb.OSType(sd.OSType),
			AssetTag:     sd.AssetTag,
			CreateTime:   timestamppb.New(sd.CreateTime),
			UpdateTime:   timestamppb.New(sd.UpdateTime),
			EnrollStatus: devicepb.DeviceEnrollStatus(sd.EnrollStatus),
		}
	}

	// Full device information.
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

func storedToDevice(deviceID string, sd *storedDevice) *devicepb.Device {
	return storedToDeviceView(deviceID, sd, devicepb.DeviceView_DEVICE_VIEW_RESOURCE)
}

func deviceKeyStart() []byte {
	return backend.Key("devices", "id")
}

func deviceKey(deviceID string) []byte {
	return backend.Key("devices", "id", deviceID)
}

func devicesByAssetTagKey(assetTag string) []byte {
	return backend.Key("devices", "byTag", assetTag)
}

func deviceTokenKey(deviceID string) []byte {
	return backend.Key("devices", "enroll_token", deviceID)
}

func collectedDataKey(deviceID, cdID string) []byte {
	return backend.Key("devices", "collected_data", deviceID, cdID)
}

func collectedDataKeyStart(deviceID string) []byte {
	return backend.Key("devices", "collected_data", deviceID)
}
