package storage

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/modules"
)

const (
	// MaxCollectedDataPerDevice is the maximum number of (non-enrollment)
	// collected data, for each device.
	MaxCollectedDataPerDevice = 10

	// DeviceEnrollTokenExpireDuration is the default expiration for enrollment
	// tokens.
	DeviceEnrollTokenExpireDuration = 1 * time.Hour

	// deviceWebAuthnAttemptExpireDuration is the default expiration for newly
	// created device web authentication attempts.
	// Failed attempts are deleted on the spot, regardless of the operation's
	// outcome.
	deviceWebAuthnAttemptExpireDuration = 5 * time.Minute
)

// ErrEnrolledDeviceLimit is returned when device enrollment is restricted due to license limit.
var ErrEnrolledDeviceLimit = &trace.AccessDeniedError{Message: "cluster has reached its enrolled trusted device limit, please contact the cluster administrator"}

const (
	currentAPIVersion = "v1"

	// enrollmentDataID is the fixed ID used enrollment collected data.
	enrollmentDataID = "1"
)

// UsersService represents the subset of [services.UsersService] used by [S].
type UsersService interface {
	// UpdateAndSwapUser reads and updates a user.
	UpdateAndSwapUser(ctx context.Context, user string, withSecrets bool, fn func(u types.User) (changed bool, err error)) (types.User, error)
}

// Params are creational params for [S].
type Params struct {
	Logger       *slog.Logger
	Backend      backend.Backend
	UsersService UsersService
	Modules      modules.Modules
	// BCryptCostOverride allows overriding the default bcrypt cost.
	BCryptCostOverride int
}

// S implements the Device Trust storage, backed by a backend.Backend.
type S struct {
	logger     *slog.Logger
	backend    backend.Backend
	users      UsersService
	bcryptCost int
	modules    modules.Modules
}

// New returns a new Device Trust storage instance.
func New(params Params) (*S, error) {
	switch {
	case params.Backend == nil:
		return nil, trace.BadParameter("param Backend required")
	case params.UsersService == nil:
		return nil, trace.BadParameter("param UsersService required")
	case params.Modules == nil:
		return nil, trace.BadParameter("param Modules required")
	}

	cost := bcrypt.DefaultCost
	if params.BCryptCostOverride > 0 {
		cost = params.BCryptCostOverride
	}

	baseLogger := params.Logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}

	return &S{
		logger:     baseLogger.With(teleport.ComponentKey, "devicetrust.storage"),
		backend:    params.Backend,
		users:      params.UsersService,
		bcryptCost: cost,
		modules:    params.Modules,
	}, nil
}

func (s *S) nowUTC() time.Time {
	return s.backend.Clock().Now().UTC()
}

// BulkCreateDevices creates devices in bulk.
// If `createAsResource` is `true`, readonly and system-managed fields that are
// present are copied as-is to storage. This is to allow "device" tctl resources
// to behave alike other resources. Prefer non-resource creation if possible.
// Returns, for each device, a DeviceOrStatus with a non-empty ID in case of
// success, or a failure Status in case of error. The response is guaranteed to
// have the same ordering as the input.
func (s *S) BulkCreateDevices(ctx context.Context, devs []*devicepb.Device, createAsResource bool) []*devicepb.DeviceOrStatus {
	errToStatus := func(err error) *statuspb.Status {
		return status.Convert(trail.ToGRPC(err)).Proto()
	}

	resp := make([]*devicepb.DeviceOrStatus, len(devs)) // same order as devs
	seenTags := make(map[assetTagKey]struct{})
	for i, dev := range devs {
		resp[i] = &devicepb.DeviceOrStatus{}

		// Is the device valid?
		if err := ValidateDeviceForCreate(dev, createAsResource); err != nil {
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
			created, err := s.createDevice(ctx, dev, createAsResource)
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
// If `createAsResource` is `true`, readonly and system-managed fields that are
// present are copied as-is to storage. This is to allow "device" tctl resources
// to behave alike other resources. Prefer non-resource creation if possible.
// Returns the stored device.
// Prefer using [BulkCreateDevices] if you want to create multiple devices
// concurrently.
func (s *S) CreateDevice(ctx context.Context, dev *devicepb.Device, createAsResource bool) (*devicepb.Device, error) {
	if err := ValidateDeviceForCreate(dev, createAsResource); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.createDevice(ctx, dev, createAsResource)
}

func (s *S) createDevice(ctx context.Context, dev *devicepb.Device, createAsResource bool) (*devicepb.Device, error) {
	// There are two use-cases for resource-like creates:
	//
	// 1. Brand-new insertions, similarly to non-resource creation
	// 2. Backup-like insertions, i.e., the equivalent of
	//    `tctl rm devices/X | tctl create`
	//
	// There's no clear indication of what we're looking at, except that fields
	// that are usually system-managed are present, so we do our best to honor
	// those and insert as-is to storage.

	deviceID, storedDev := deviceToStored(dev, s.nowUTC(), createAsResource)

	// Marshal device before writes, just in the extremely unlikely case that it
	// fails.
	storedJSON, err := json.Marshal(storedDev)
	if err != nil {
		return nil, trace.Wrap(err, "marshal device")
	}

	// Create/update asset tag index.
	// It's OK to leave the asset tag mapping behind if writing the device fails.
	ref := &deviceRef{
		DeviceID: deviceID,
		OSType:   storedDev.OSType,
	}
	if err := s.updateAssetTagIndex(ctx, storedDev.AssetTag, ref); err != nil {
		return nil, trace.Wrap(err)
	}

	// Write device.
	if _, err := s.backend.Create(ctx, backend.Item{
		Key:   deviceKey(deviceID),
		Value: storedJSON,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// Note: we don't write collected data on pure Create or Update methods -
	// all collected data writes should be predicated on a successful device
	// challenge.
	// The DeviceProfile is the best way to constrain the device by various
	// characteristics (for versions newer than v12).

	return storedToDevice(deviceID, storedDev), nil
}

func deviceToStored(d *devicepb.Device, now time.Time, createAsResource bool) (deviceID string, storedDev *storedDevice) {
	var storedSource *storedDeviceSource
	if d.Source != nil {
		storedSource = &storedDeviceSource{
			Name:   d.Source.Name,
			Origin: int(d.Source.Origin),
		}
	}

	// Profiles have no required fields, but there's no point saving an empty
	// profile so let's avoid that.
	var storedProfile *storedDeviceProfile
	if (createAsResource && d.Profile != nil) || !isDeviceProfileEmpty(d.Profile) {
		storedProfile = &storedDeviceProfile{
			UpdateTime:          now,
			ModelIdentifier:     d.Profile.ModelIdentifier,
			OSVersion:           d.Profile.OsVersion,
			OSBuild:             d.Profile.OsBuild,
			OSBuildSupplemental: d.Profile.OsBuildSupplemental,
			OSUsernames:         d.Profile.OsUsernames,
			JamfBinaryVersion:   d.Profile.JamfBinaryVersion,
			ExternalID:          d.Profile.ExternalId,
			OSID:                d.Profile.OsId,
		}
	}

	storedDev = &storedDevice{
		OSType:       int(d.OsType),
		AssetTag:     d.AssetTag,
		CreateTime:   now,
		UpdateTime:   now,
		EnrollStatus: int(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED),
		Source:       storedSource,
		Profile:      storedProfile,
	}

	// Non-resource writes don't assign readonly or system-managed fields.
	if !createAsResource {
		return uuid.NewString(), storedDev
	}

	// ID.
	deviceID = d.Id
	if deviceID == "" {
		deviceID = uuid.NewString()
	}

	// CreateTime and UpdateTime.
	if d.CreateTime != nil {
		storedDev.CreateTime = d.CreateTime.AsTime()
	}
	if d.UpdateTime != nil {
		storedDev.UpdateTime = d.UpdateTime.AsTime()
	}

	// EnrollToken: enrollment tokens cannot be backed-up via `tctl get`; they are
	// considered ephemeral by the system and never returned on reads.
	// That seems to be for the best.

	// EnrollStatus.
	if d.EnrollStatus != devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED {
		storedDev.EnrollStatus = int(d.EnrollStatus)
	}

	// DeviceCredential.
	if cred := d.Credential; cred != nil {
		storedDev.Credential = &storedDeviceCredential{
			ID:                    cred.Id,
			PublicKeyDER:          cred.PublicKeyDer,
			DeviceAttestationType: int(cred.DeviceAttestationType),
			TPMEKCertSerial:       cred.TpmEkcertSerial,
			TPMAKPublic:           cred.TpmAkPublic,
		}
	}

	// Profile.
	if updateTime := d.Profile.GetUpdateTime(); updateTime != nil {
		storedDev.Profile.UpdateTime = updateTime.AsTime()
	}

	// Owner (normally set on enroll or authn).
	storedDev.Owner = d.Owner

	return deviceID, storedDev
}

// isDeviceProfileEmpty returns true if the [devicepb.DeviceProfile] is
// considered empty.
// A profile that lacks any data other than the UpdateTime, which is a
// system-managed field, is considered empty.
func isDeviceProfileEmpty(p *devicepb.DeviceProfile) bool {
	return p == nil || proto.Equal(p, &devicepb.DeviceProfile{UpdateTime: p.UpdateTime})
}

func (s *S) updateAssetTagIndex(ctx context.Context, assetTag string, ref *deviceRef) error {
	return trace.Wrap(s.updateDeviceRefsIndex(ctx, devicesByAssetTagKey(assetTag), ref, true /* dedupMatchingOS */))
}

// updateDeviceRefsIndex creates or updates a hand-written devicesRef index.
func (s *S) updateDeviceRefsIndex(
	ctx context.Context,
	key backend.Key,
	ref *deviceRef,
	dedupMatchingOS bool,
) error {
	logger := s.logger.With(
		"key", key.String(),
		"device_id", ref.DeviceID,
		"os_type", ref.OSType,
	)

	var lastErr error
	const maxAttempts = 3 // arbitrary
	for range maxAttempts {
		var retry bool
		current, getErr := s.backend.Get(ctx, key)
		switch {
		case trace.IsNotFound(getErr): // New devices ref
			retry, lastErr = s.createDeviceRef(ctx, key, ref)
			if lastErr != nil {
				logger.DebugContext(ctx,
					"Failed to write new devices ref entry, retrying",
					"error", lastErr,
				)
			}

		case getErr == nil: // Existing devices ref
			retry, lastErr = s.appendDeviceRef(ctx, current, ref, dedupMatchingOS)
			if lastErr != nil {
				logger.DebugContext(ctx,
					"Failed to append to devices ref entry, retrying",
					"error", lastErr,
				)
			}

		default: // getErr != nil
			logger.WarnContext(ctx,
				"Unexpected error reading devices ref entry, retrying",
				"error", getErr,
			)

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

func (s *S) createDeviceRef(ctx context.Context, key backend.Key, ref *deviceRef) (retryable bool, err error) {
	val, err := json.Marshal(&devicesRef{
		Devices: []*deviceRef{ref},
	})
	if err != nil {
		return false, trace.Wrap(err, "marshal devices reference")
	}

	if _, err := s.backend.Create(ctx, backend.Item{
		Key:   key,
		Value: val,
	}); err != nil {
		return true, trace.Wrap(err)
	}
	return false, nil
}

func (s *S) appendDeviceRef(
	ctx context.Context,
	current *backend.Item,
	ref *deviceRef,
	dedupMatchingOS bool, // byAssetTag only
) (retryable bool, err error) {
	refs := &devicesRef{}
	if err := json.Unmarshal(current.Value, refs); err != nil {
		return false, trace.Wrap(err, "unmarshal devices reference")
	}

	// Is the device already mapped? Nothing to do in that case.
	for _, existing := range refs.Devices {
		if existing.DeviceID == ref.DeviceID {
			return false, nil
		}
		if dedupMatchingOS && existing.OSType == ref.OSType {
			// Does the device _really_ exist?
			// Let's not have a hanging mapping inutilize an asset tag.
			if _, getErr := s.backend.Get(ctx, deviceKey(existing.DeviceID)); getErr == nil {
				return false, trace.AlreadyExists("asset tag already registered")
			}

			// We either found a hanging mapping or there is a race on CreateDevice.
			// Let both tags be, admins can clear duplicate devices manually.
			s.logger.WarnContext(ctx,
				"Found possible duplicate on asset tag mapping",
				"asset_tag", deviceIDFromKey(current.Key),
				"existing_id", existing.DeviceID,
				"new_id", ref.DeviceID,
			)
		}
	}
	refs.Devices = append(refs.Devices, ref)

	val, err := json.Marshal(refs)
	if err != nil {
		return false, trace.Wrap(err, "marshal devices reference")
	}

	if _, err := s.backend.CompareAndSwap(ctx, *current, backend.Item{
		Key:   current.Key,
		Value: val,
	}); err != nil {
		return true, trace.Wrap(err)
	}
	return false, nil
}

// UpdateDevice updates an existing device in storage.
// Only fields considered mutable are updated, attempts to update readonly
// fields result in errors.
// Transient fields like EnrollToken and CollectedData are ignored during
// updates.
func (s *S) UpdateDevice(
	ctx context.Context,
	deviceID string, updateFn func(stored *devicepb.Device) *devicepb.Device) (*devicepb.Device, error) {
	if updateFn == nil {
		return nil, trace.BadParameter("updateFunc required")
	}

	stored, _, item, err := s.getDeviceByID(ctx, deviceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Modify fields.
	updated := updateFn(proto.Clone(stored).(*devicepb.Device))

	// Ignore transient fields.
	updated.EnrollToken = nil   // Safe to nil, saved to deviceTokenKey.
	updated.CollectedData = nil // Safe to nil, saved to collectedDataKey.

	if isDeviceProfileEmpty(updated.Profile) {
		// Null "empty" profiles.
		updated.Profile = nil
	} else if updated.Profile != nil {
		// Ignore Profile.UpdateTime.
		updated.Profile.UpdateTime = stored.GetProfile().GetUpdateTime()
	}

	// Has the device changed?
	if proto.Equal(updated, stored) {
		return updated, nil
	}

	// Validate changes.
	if err := validateDeviceForUpdate(updated, stored); err != nil {
		return nil, trace.Wrap(err)
	}

	// System-managed: update time.
	now := s.nowUTC()
	updated.UpdateTime = timestamppb.New(now)

	// System-managed: profile update time, if changed.
	if updated.Profile != nil && !proto.Equal(stored.Profile, updated.Profile) {
		updated.Profile.UpdateTime = timestamppb.New(now)
	}

	// System-managed: perform unenrollment, if necessary:
	// * Erase device credential
	// * Erase device owner
	// * Erase collected data
	if stored.EnrollStatus == devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED &&
		updated.EnrollStatus == devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED {
		updated.Credential = nil // Written below.
		updated.Owner = ""       // Written below.

		if err := s.deleteCollectedData(ctx, deviceID); err != nil {
			return nil, trace.Wrap(err, "deleting collected data on unenroll")
		}

		owner := stored.Owner
		if err := s.unassignDeviceFromUser(ctx, owner, deviceID); err != nil {
			return nil, trace.Wrap(err, "unassigning device from user")
		}
	}

	// Convert updated dev to storage.
	_, storedU := deviceToStored(updated, now, true /* createAsResource */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal and update.
	val, err := json.Marshal(storedU)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := s.backend.CompareAndSwap(ctx, *item, backend.Item{
		Key:   item.Key,
		Value: val,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return storedToDevice(deviceID, storedU), nil
}

// AssignDeviceOwner assigns an owner to the device.
// The device must not be owned by another user.
func (s *S) AssignDeviceOwner(ctx context.Context, deviceID, owner string) (*devicepb.Device, error) {
	if owner == "" {
		return nil, trace.BadParameter("owner required")
	}

	dev, err := s.assignDeviceOwner(ctx, deviceID, owner)
	if err != nil {
		return nil, trace.Wrap(err, "assigning owner to device")
	}

	if err := s.assignDeviceToUser(ctx, owner, deviceID); err != nil {
		return nil, trace.Wrap(err, "assigning device to user")
	}

	return dev, nil
}

func (s *S) assignDeviceOwner(ctx context.Context, deviceID, owner string) (*devicepb.Device, error) {
	dev, stored, item, err := s.getDeviceByID(ctx, deviceID)
	switch {
	case err != nil:
		return nil, trace.Wrap(err)
	case stored.Owner == owner:
		return dev, nil // Nothing to do.
	case stored.Owner != "":
		return nil, trace.BadParameter("device already has an owner")
	}

	stored.UpdateTime = s.nowUTC()
	stored.Owner = owner
	val, err := json.Marshal(stored)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := s.backend.CompareAndSwap(ctx, *item, backend.Item{
		Key:   item.Key,
		Value: val,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return storedToDevice(deviceID, stored), nil
}

func (s *S) assignDeviceToUser(ctx context.Context, user, deviceID string) error {
	if user == "" {
		return nil
	}

	g, gCtx := errgroup.WithContext(ctx)

	// Assign devices to User.TrustedDeviceIDs.
	// Transient for SSO, but useful informational field for local users.
	g.Go(func() error {
		_, err := s.users.UpdateAndSwapUser(gCtx, user, false /* withSecrets */, func(u types.User) (changed bool, err error) {
			ids := u.GetTrustedDeviceIDs()
			if slices.Contains(ids, deviceID) {
				return false, nil // Nothing to do.
			}

			u.SetTrustedDeviceIDs(append(ids, deviceID))
			return true, nil
		})
		return trace.Wrap(err)
	})

	// Update hand-written user->devices index.
	g.Go(func() error {
		return trace.Wrap(s.updateUserDevicesIndex(gCtx, user, deviceID))
	})

	return trace.Wrap(g.Wait())
}

func (s *S) updateUserDevicesIndex(ctx context.Context, user, deviceID string) error {
	ref := &deviceRef{
		DeviceID: deviceID,
		// OSType not used by this index.
	}
	return trace.Wrap(s.updateDeviceRefsIndex(ctx, devicesByUserKey(user), ref, false /* dedupMatchingOS */))
}

func (s *S) unassignDeviceFromUser(ctx context.Context, user, deviceID string) error {
	if user == "" {
		return nil // Nothing to do. May happen for legacy devices.
	}

	g, gCtx := errgroup.WithContext(ctx)

	// Remove device from User.TrustedDeviceIDs.
	g.Go(func() error {
		_, err := s.users.UpdateAndSwapUser(gCtx, user, false /* withSecrets */, func(u types.User) (changed bool, err error) {
			ids := u.GetTrustedDeviceIDs()
			if !slices.Contains(ids, deviceID) {
				return false, nil // Nothing to do
			}

			for i := 0; i < len(ids); i++ {
				if ids[i] == deviceID {
					ids = slices.Delete(ids, i, i+1)
					i--
				}
			}
			u.SetTrustedDeviceIDs(ids)
			return true, nil
		})
		if trace.IsNotFound(err) {
			return nil // Nothing to do in this case.
		}
		return trace.Wrap(err)
	})

	// Remove from hand-written user->devices index.
	g.Go(func() error {
		return trace.Wrap(s.removeFromUserDevicesIndex(gCtx, user, deviceID))
	})

	return trace.Wrap(g.Wait())
}

func (s *S) removeFromUserDevicesIndex(ctx context.Context, user, deviceID string) error {
	return trace.Wrap(s.removeFromDeviceRefsIndex(ctx, devicesByUserKey(user), deviceID))
}

type deviceToDelete struct {
	ID       string `json:"-"`
	AssetTag string `json:"asset_tag"`
	Owner    string `json:"owner"`
}

// DeleteDevicePredicate hard-deletes the specified device from storage if it
// matches the predicate.
func (s *S) DeleteDevicePredicate(ctx context.Context, deviceID string, p func(d *devicepb.Device) error) error {
	dev, stored, _, err := s.getDeviceByID(ctx, deviceID)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := p(dev); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.deleteDevice(ctx, &deviceToDelete{
		ID:       deviceID,
		AssetTag: stored.AssetTag, // Don't copy from `dev` in case `p` does something funky.
		Owner:    stored.Owner,
	}))
}

// DeleteDevice hard-deletes a device from storage.
func (s *S) DeleteDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return trace.BadParameter("device ID required")
	}

	item, err := s.backend.Get(ctx, deviceKey(deviceID))
	if err != nil {
		return trace.Wrap(err)
	}

	// Unmarshal as little as we can from the device, this makes deletion a
	// possible form of recovery from storage problems.
	dev := &deviceToDelete{}
	if err := json.Unmarshal(item.Value, dev); err != nil {
		return trace.Wrap(err)
	}
	dev.ID = deviceID

	return trace.Wrap(s.deleteDevice(ctx, dev))
}

func (s *S) deleteDevice(ctx context.Context, dev *deviceToDelete) error {
	// Unassign device from user.
	if err := s.unassignDeviceFromUser(ctx, dev.Owner, dev.ID); err != nil {
		return trace.Wrap(err, "unassigning device from user")
	}

	// Delete the device.
	// If this succeeds the invocation is considered a success: the device key is
	// the source of truth for a device existing, the system can handle "hanging"
	// asset tags.
	if err := s.backend.Delete(ctx, deviceKey(dev.ID)); err != nil {
		return trace.Wrap(err)
	}

	logger := func() *slog.Logger {
		return s.logger.With(
			"device_id", dev.ID,
			"asset_tag", dev.AssetTag,
		)
	}

	// Remove asset tag mapping.
	if err := s.removeFromAssetTagIndex(ctx, dev.ID, dev.AssetTag); err != nil {
		logger().WarnContext(ctx,
			"Failed to remove asset tag mapping for device",
			"error", err,
		)
		// err swallowed on purpose.
	}

	// Remove enroll token, if present.
	if err := s.backend.Delete(ctx, deviceTokenKey(dev.ID)); err != nil && !trace.IsNotFound(err) {
		logger().WarnContext(ctx,
			"Failed to remove enroll token for device",
			"error", err,
		)
		// err swallowed on purpose.
	}

	// Remove collected data.
	if err := s.deleteCollectedData(ctx, dev.ID); err != nil {
		logger().WarnContext(ctx,
			"Failed to remove collected data for device",
			"error", err,
		)
		// err swallowed on purpose.
	}

	return nil
}

func (s *S) removeFromAssetTagIndex(ctx context.Context, deviceID, assetTag string) error {
	return trace.Wrap(s.removeFromDeviceRefsIndex(ctx, devicesByAssetTagKey(assetTag), deviceID))
}

// removeFromDeviceRefsIndex removes a deviceRef from a hand-written devicesRef
// index.
func (s *S) removeFromDeviceRefsIndex(
	ctx context.Context,
	key backend.Key,
	deviceID string,
) error {
	item, err := s.backend.Get(ctx, key)
	switch {
	case trace.IsNotFound(err):
		return nil // Nothing to unassign.
	case err != nil:
		return trace.Wrap(err, "reading devices ref index")
	}

	refs := &devicesRef{}
	if err := json.Unmarshal(item.Value, refs); err != nil {
		return trace.Wrap(err, "unmarshal devices ref index")
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
		return trace.Wrap(err, "marshal devices ref")
	}

	if _, err := s.backend.CompareAndSwap(ctx, *item, backend.Item{
		Key:   item.Key,
		Value: val,
	}); err != nil {
		return trace.Wrap(err, "writing devices ref index")
	}

	return nil
}

func (s *S) deleteCollectedData(ctx context.Context, deviceID string) error {
	cdStart := collectedDataKeyStart(deviceID)
	cdEnd := backend.RangeEnd(cdStart)
	return trace.Wrap(s.backend.DeleteRange(ctx, cdStart, cdEnd))
}

// GetDeviceByID reads a device by ID.
// Returns the stored device or trace.NotFound.
func (s *S) GetDeviceByID(ctx context.Context, deviceID string) (*devicepb.Device, error) {
	type collectedDataResp struct {
		cd  []*devicepb.DeviceCollectedData
		err error
	}

	// Fetch collected data for the device asynchronously.
	cdCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cdC := make(chan collectedDataResp)
	go func() {
		cd, err := s.getDeviceCollectedData(cdCtx, deviceID)
		cdC <- collectedDataResp{
			cd:  cd,
			err: err,
		}
	}()

	// Fetch the device.
	dev, _, _, err := s.getDeviceByID(ctx, deviceID)
	if err != nil {
		cancel() // Stop and wait for goroutine.
		<-cdC
		return nil, trace.Wrap(err)
	}

	// Add collected data to it.
	resp := <-cdC
	if resp.err != nil {
		s.logger.WarnContext(ctx,
			"Failed to fetch collected data for device",
			"error", err,
			"device_id", deviceID,
		)
		// err swallowed on purpose, in keeping with legacy behavior
	}
	dev.CollectedData = resp.cd // Always safe to do.

	return dev, nil
}

// getDeviceByID is the internal version of GetDeviceByID.
// It returns all internal data structures along with the device.
func (s *S) getDeviceByID(ctx context.Context, deviceID string) (*devicepb.Device, *storedDevice, *backend.Item, error) {
	if deviceID == "" {
		return nil, nil, nil, trace.BadParameter("device ID required")
	}

	item, err := s.backend.Get(ctx, deviceKey(deviceID))
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
	res, err := s.backend.GetRange(ctx, start, end, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cd := make([]*devicepb.DeviceCollectedData, len(res.Items))
	for i, item := range res.Items {
		stored := &storedCollectedData{}
		if err := json.Unmarshal(item.Value, stored); err != nil {
			return nil, trace.Wrap(err)
		}
		cd[i] = storedToCollectedData(stored)
	}

	// Sort by ascending RecordTime.
	slices.SortFunc(cd, func(a, b *devicepb.DeviceCollectedData) int {
		return a.RecordTime.AsTime().Compare(b.RecordTime.AsTime())
	})

	return cd, nil
}

// GetDeviceIDByOSTag returns a device ID from an {osType,assetTag} pair.
//
// This is a lower-level method than [GetDeviceByID], which most callers should
// prefer.
//
// The `verifyExistence` flag can be set so that the ID is checked against the
// device key, instead of only against the asset tag mapping. Setting it to
// `false` can cause [GetDeviceIDByOSTag] and [GetDeviceByID] to disagree on
// whether a device exists, with the latter being the more accurate.
// Be careful when setting it to `false`.
func (s *S) GetDeviceIDByOSTag(ctx context.Context, osType devicepb.OSType, assetTag string, verifyExistence bool) (string, error) {
	switch {
	case osType == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return "", trace.BadParameter("os type required")
	case assetTag == "":
		return "", trace.BadParameter("asset tag required")
	}

	refs, err := s.getDeviceRefsByTag(ctx, assetTag)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var deviceID string
	for _, dev := range refs.Devices {
		if dev.OSType == int(osType) {
			deviceID = dev.DeviceID
			break
		}
	}
	if deviceID == "" {
		return "", trace.NotFound("device not found")
	}
	if !verifyExistence {
		return deviceID, nil
	}

	// Make sure the device actually exists.
	// We don't need the backend.Item.Value here, but there's no way to check
	// that a key exists without fetching it.
	// At least this saves an unmarshal and other reads that GetDeviceByID
	// typically performs.
	if _, err := s.backend.Get(ctx, deviceKey(deviceID)); err != nil {
		return "", trace.Wrap(err)
	}
	return deviceID, nil
}

// GetDevicesByAssetTag reads devices by asset tag.
// Returns an empty slice if no devices are found.
func (s *S) GetDevicesByAssetTag(ctx context.Context, assetTag string) ([]*devicepb.Device, error) {
	if assetTag == "" {
		return nil, trace.BadParameter("asset tag required")
	}

	refs, err := s.getDeviceRefsByTag(ctx, assetTag)
	switch {
	case trace.IsNotFound(err):
		return nil, nil
	case err != nil:
		return nil, trace.Wrap(err)
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

func (s *S) getDeviceRefsByTag(ctx context.Context, assetTag string) (*devicesRef, error) {
	return s.getDeviceRefs(ctx, devicesByAssetTagKey(assetTag))
}

func (s *S) getDeviceRefs(ctx context.Context, key backend.Key) (*devicesRef, error) {
	item, err := s.backend.Get(ctx, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	refs := &devicesRef{}
	return refs, trace.Wrap(json.Unmarshal(item.Value, refs))
}

// GetUserTrustedDeviceIDs reads the devices associated to a user from the
// "devicesByUser" index.
// User-device associations are created by [S.EnrollDevice] or
// [S.AssignDeviceOwner] and may be removed during device updates or deletes.
func (s *S) GetUserTrustedDeviceIDs(ctx context.Context, user string) ([]string, error) {
	refs, err := s.getDeviceRefs(ctx, devicesByUserKey(user))
	switch {
	case trace.IsNotFound(err): // Unknown user
		return nil, nil
	case err != nil:
		return nil, trace.Wrap(err)
	}

	deviceIDs := make([]string, len(refs.Devices))
	for i, ref := range refs.Devices {
		deviceIDs[i] = ref.DeviceID
	}
	return deviceIDs, nil
}

// ListDevicesByUser is a paginated search of devices for the user. It returns the found devices
// and the page token for the next call.
//
// Use an empty pageToken to start the search and the returned nextPageToken for
// subsequent calls. An empty nextPageToken signifies the end of the search.
// Callers are not expected to change other parameters in the same
// series of invocations.
//
// The requested pageSize is not guaranteed, as the server may change it at its
// discretion.
func (s *S) ListDevicesByUser(ctx context.Context, pageSize int, pageToken string, user string) (devices []*devicepb.Device, nextPageToken string, err error) {
	// maxPageSize is the max size a page can be when fetching devices.
	const maxPageSize = 200

	devIDs, err := s.GetUserTrustedDeviceIDs(ctx, user)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// sort the slice so we can binary search for the nextPageToken
	slices.Sort(devIDs)
	startIndex, _ := slices.BinarySearch(devIDs, pageToken)

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	const maxActiveGoroutines = 2
	var g errgroup.Group
	g.SetLimit(maxActiveGoroutines)

	// Calculate the end index for slicing
	endIndex := startIndex + pageSize
	if l := len(devIDs); endIndex >= l {
		endIndex = len(devIDs)
	} else {
		// Get nextPageToken before slicing.
		nextPageToken = devIDs[endIndex]
	}

	devIDs = devIDs[startIndex:endIndex]

	var mu sync.Mutex // guards devices
	for _, id := range devIDs {
		g.Go(func() error {
			dev, err := s.GetDeviceByID(ctx, id)
			if err != nil {
				s.logger.ErrorContext(ctx,
					"Failed to get device",
					"id", id,
					"error", err,
				)
				return nil
			}
			if dev.Owner != user {
				return nil
			}

			mu.Lock()
			defer mu.Unlock()
			devices = append(devices, dev)
			return nil
		})

	}

	// Wait() should never error, as the goroutines don't themselves.
	_ = g.Wait()

	slices.SortFunc(devices, func(a, b *devicepb.Device) int {
		return strings.Compare(a.Id, b.Id)
	})

	return devices, nextPageToken, nil
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

	res, err := s.backend.GetRange(ctx, startKey, endKey, pageSize)
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
			// Be resilient against JSON failures, otherwise it's impossible to list
			// any devices.
			s.logger.ErrorContext(ctx,
				"Failed to unmarshal device, stored value may be invalid or corrupted",
				"error", err,
				"key", item.Key.String(),
			)
			continue
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
			g.Go(func() error {
				cd, err := s.getDeviceCollectedData(ctx, dev.Id)
				if err != nil {
					s.logger.WarnContext(ctx,
						"Failed to fetch collected data for device",
						"error", err,
						"device_id", dev.Id,
					)
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
	deviceID string, cred *devicepb.DeviceCredential, cd *devicepb.DeviceCollectedData,
	owner string,
) (*devicepb.Device, error) {
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
	if _, err := ValidateDeviceCredential(cred, dev.OsType); err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal new device first, so we can exit in the off chance it errors.
	now := s.nowUTC()
	stored.UpdateTime = now
	stored.EnrollStatus = int(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED)
	stored.Credential = &storedDeviceCredential{
		ID:                    cred.Id,
		PublicKeyDER:          cred.PublicKeyDer,
		DeviceAttestationType: int(cred.DeviceAttestationType),
		TPMEKCertSerial:       cred.TpmEkcertSerial,
		TPMAKPublic:           cred.TpmAkPublic,
	}
	prevOwner := stored.Owner // Save so we can unassign the device.
	stored.Owner = owner
	val, err := json.Marshal(stored)
	if err != nil {
		return nil, trace.Wrap(err, "marshal device")
	}

	// Clear previous collected data.
	// A newly-enrolled device is a blank slate.
	cdKeyStart := collectedDataKeyStart(dev.Id)
	if err := s.backend.DeleteRange(ctx, cdKeyStart, backend.RangeEnd(cdKeyStart)); err != nil {
		const msg = "" +
			"Failed to clear device collected data during enrollment. " +
			"This could lead to difficulties in device authentication, if that happens try enrolling the device again. " +
			"Proceeding."
		s.logger.WarnContext(ctx,
			msg,
			"error", err,
			"device_id", dev.Id,
			"asset_tag", dev.AssetTag,
		)
	}

	// Marshal and write collected data.
	if err := s.recordCollectedData(ctx, deviceID, cd, originEnrollment, now); err != nil {
		return nil, trace.Wrap(err)
	}

	updateDevice := func() error {
		_, err := s.backend.CompareAndSwap(ctx, *item, backend.Item{
			Key:   item.Key,
			Value: val,
		})
		return trace.Wrap(err)
	}

	// Unassign device from previous owner, if any.
	// The service layer will redo the assignment if any following updates fail.
	if prevOwner != "" && prevOwner != owner {
		if err := s.unassignDeviceFromUser(ctx, prevOwner, deviceID); err != nil {
			return nil, trace.Wrap(err, "unassigning device from user")
		}
	}

	// Verify limits if the account is usage-based, otherwise just update.
	var completeEnrollFn func() error
	if f := s.modules.Features(); f.IsUsageBasedBilling {
		completeEnrollFn = func() error {
			return backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
				LockConfiguration: backend.LockConfiguration{
					Backend:            s.backend,
					LockNameComponents: []string{"devicesEnrollLock"},
					TTL:                5 * time.Second,
					RetryInterval:      100 * time.Millisecond,
				},
			}, func(ctx context.Context) error {
				if err := s.VerifyEnrolledDevicesLimit(ctx); err != nil {
					return trace.Wrap(err)
				}
				return trace.Wrap(updateDevice())
			})
		}
	} else {
		completeEnrollFn = updateDevice
	}

	if err := completeEnrollFn(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Assign trusted device to user.
	// This comes after enrollment proper because that's when we check for limits.
	// Since enrollment already happened, any errors here are swallowed.
	// The service layer will redo the assignment if this fails.
	if err := s.assignDeviceToUser(ctx, owner, deviceID); err != nil {
		s.logger.WarnContext(ctx,
			"Failed to assign device to user",
			"error", err,
			"device_id", deviceID,
			"asset_tag", dev.AssetTag,
			"user", owner,
		)
		// err swallowed on purpose.
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
	if err := s.validateCollectedDataDrift(ctx, dev, cd); err != nil {
		return trace.Wrap(err)
	}

	err = s.recordCollectedData(ctx, deviceID, cd, originAuthentication, s.nowUTC())
	return trace.Wrap(err)
}

func (s *S) validateCollectedDataDrift(ctx context.Context, dev *devicepb.Device, cd *devicepb.DeviceCollectedData) error {
	stored, err := s.getDeviceCollectedData(ctx, dev.Id)
	if err != nil {
		return trace.Wrap(err)
	}

	return validateCollectedDataDriftQueried(s.logger, dev, stored, cd)

}

// validateCollectedDataDriftQueried runs data drift validation on `cd` using an
// already queried slice of collected data.
// It can do some nice logging using `dev` too.
func validateCollectedDataDriftQueried(logger *slog.Logger, dev *devicepb.Device, stored []*devicepb.DeviceCollectedData, cd *devicepb.DeviceCollectedData) error {
	l := len(stored)
	if l == 0 {
		logger.WarnContext(context.Background(),
			"Found no collected data entries for device. Skipping collected data drift validation.",
			"device_id", dev.Id,
			"asset_tag", dev.AssetTag,
		)
		return nil
	}

	// Comparing `cd` against the last entry should be enough to guarantee no
	// drift, as data can only drift forward.
	// Note that storedCD is already sorted by RecordTime DESC.
	source := stored[l-1]
	if err := validateCollectedDataDrift(cd, source); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *S) recordCollectedData(ctx context.Context, deviceID string, cd *devicepb.DeviceCollectedData, origin collectedDataOrigin, recordTime time.Time) error {
	storedCD := collectedDataToStored(cd, origin, recordTime, false /* createAsResource */)

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

	if _, err := s.backend.Put(ctx, backend.Item{
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
		s.logger.WarnContext(ctx,
			"Failed to clear collected data for device",
			"error", err,
			"device_id", deviceID,
		)
		// err swallowed on purpose, new data is already written.
	}

	return nil
}

// simplifiedCollectedData is used to decide which collected data entries to
// delete.
type simplifiedCollectedData struct {
	Key        backend.Key `json:"-"`
	RecordTime time.Time   `json:"record_time"`
}

func (s *S) clearCollectedDataIfNeeded(ctx context.Context, deviceID string) error {
	start := collectedDataKeyStart(deviceID)
	end := backend.RangeEnd(start)
	limit := MaxCollectedDataPerDevice * 2 // Give the search some leeway.
	res, err := s.backend.GetRange(ctx, start, end, limit)
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
	slices.SortFunc(cd, func(a, b simplifiedCollectedData) int {
		return a.RecordTime.Compare(b.RecordTime)
	})

	// From older to newer, delete data until we hit the size limit.
	for len(cd) > MaxCollectedDataPerDevice {
		if err := s.backend.Delete(ctx, cd[0].Key); err != nil {
			return trace.Wrap(err)
		}
		cd = cd[1:]
	}

	return nil
}

// CreateDeviceEnrollTokenUsingData creates a [DeviceEnrollToken] using `cd` as an
// input.
// The collected data must pass a strict set of validations for token creation
// to be allowed.
// It returns the corresponding device, as long as queried successfully, and
// either an error or the assigned [DeviceEnrollToken] in the device.
func (s *S) CreateDeviceEnrollTokenUsingData(ctx context.Context, cd *devicepb.DeviceCollectedData) (*devicepb.Device, error) {
	if err := ValidateCollectedData(cd); err != nil {
		return nil, trace.Wrap(err)
	}

	devs, err := s.GetDevicesByAssetTag(ctx, cd.SerialNumber)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(devs) == 0 {
		return nil, trace.NotFound("device not found")
	}

	// Find the one specific device we are looking for, or otherwise error.
	var targetDev *devicepb.Device
	for _, dev := range devs {
		if dev.OsType == cd.OsType {
			// Sanity check: we should get exactly 0 or 1 match, but let's
			// double-check to be safe.
			if targetDev != nil {
				return nil, trace.BadParameter("collected data matches more than one device, aborting")
			}
			targetDev = dev
			break
		}
	}
	if targetDev == nil {
		return nil, trace.NotFound("device not found")
	}
	// From this point onwards return `targetDev`, it allows for richer logging
	// in the outer layers.

	if targetDev.EnrollStatus != devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED {
		return targetDev, trace.BadParameter("device is already enrolled")
	}

	// Run strict validation.
	// An unenrolled device is expected to have zero (or obsolete) collected data,
	// so we validate solely against the device and profile.
	if err := validateCollectedDataAgainstDeviceStrict(cd, targetDev); err != nil {
		return targetDev, trace.Wrap(err)
	}

	// Note: We don't record collected data here - the device is presently
	// unenrolled and the client did not pass a device challenge to get here.

	defaultExpire := time.Time{}
	token, err := s.createDeviceEnrollToken(
		ctx, targetDev.Id, defaultExpire, true /* createdByAutoEnroll */)
	if err != nil {
		return targetDev, trace.Wrap(err)
	}

	targetDev.EnrollToken = token
	return targetDev, trace.Wrap(err)
}

// CreateDeviceEnrollToken creates or replaces the existing enrollment token for
// a device. Only one enrollment token is allowed at a time.
//
// Enrollment tokens are tied to a particular device. If `expiresAt` is the zero
// time, tokens are set to expire after a default, reasonably short (for a
// human) amount of time.
//
// The plain token is not stored, instead it is meant to be sent out-of-band (as
// in outside of Teleport) to the person responsible for enrolling the device.
// [SpendDeviceEnrollToken] spends the token for the enrollment ceremony.
func (s *S) CreateDeviceEnrollToken(
	ctx context.Context, deviceID string, expiresAt time.Time) (*devicepb.DeviceEnrollToken, error) {
	if deviceID == "" {
		return nil, trace.BadParameter("device ID required")
	}

	// Device must exist, the easiest way to check is to read the key.
	if _, err := s.backend.Get(ctx, deviceKey(deviceID)); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.createDeviceEnrollToken(ctx, deviceID, expiresAt, false /* createdByAutoEnroll */)
}

func (s *S) createDeviceEnrollToken(
	ctx context.Context,
	deviceID string,
	expiresAt time.Time,
	createdByAutoEnroll bool,
) (*devicepb.DeviceEnrollToken, error) {
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
	tokenHashed, err := bcrypt.GenerateFromPassword([]byte(tokenPlain), s.bcryptCost)
	if err != nil {
		return nil, trace.Wrap(err, "hashing enrollment token as a password")
	}

	val, err := json.Marshal(&storedEnrollToken{
		HashedToken:         tokenHashed,
		CreatedByAutoEnroll: createdByAutoEnroll,
	})
	if err != nil {
		return nil, trace.Wrap(err, "marshal enrollment token")
	}

	// TODO(codingllama): Enforce a max expiration time for tokens?
	if expiresAt.IsZero() {
		expiresAt = s.nowUTC().Add(DeviceEnrollTokenExpireDuration)
	} else {
		expiresAt = expiresAt.UTC()
	}
	if _, err := s.backend.Put(ctx, backend.Item{
		Key:     deviceTokenKey(deviceID),
		Value:   val,
		Expires: expiresAt,
	}); err != nil {
		return nil, trace.Wrap(err, "writing enrollment token")
	}

	return &devicepb.DeviceEnrollToken{
		Token:      tokenPlain,
		ExpireTime: timestamppb.New(expiresAt),
	}, nil
}

// DeviceEnrollTokenData holds internal data about a spent DeviceEnrollToken.
type DeviceEnrollTokenData struct {
	CreatedByAutoEnroll bool
}

// SpendDeviceEnrollToken spends an existing enrollment token, allowing the
// enrollment ceremony to proceed.
// The token is immediately spent in a positive match.
// Callers are encouraged to "erase" the resulting errors with a constant
// type/message, as to avoid leaking information about storage state.
func (s *S) SpendDeviceEnrollToken(ctx context.Context, deviceID, token string) (*DeviceEnrollTokenData, error) {
	switch {
	case deviceID == "":
		return nil, trace.BadParameter("device ID required")
	case token == "":
		return nil, trace.BadParameter("token required")
	}

	// Device must exist, the easiest way to check is to read the key.
	if _, err := s.backend.Get(ctx, deviceKey(deviceID)); err != nil {
		return nil, trace.Wrap(err)
	}

	key := deviceTokenKey(deviceID)
	item, err := s.backend.Get(ctx, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stored := &storedEnrollToken{}
	if err := json.Unmarshal(item.Value, stored); err != nil {
		return nil, trace.Wrap(err, "unmarshal enrollment token")
	}

	if err := bcrypt.CompareHashAndPassword(stored.HashedToken, []byte(token)); err != nil {
		return nil, trace.BadParameter("invalid token")
	}
	if err := s.backend.Delete(ctx, key); err != nil {
		return nil, trace.Wrap(err, "failed to spend enrollment token")
	}

	return &DeviceEnrollTokenData{
		CreatedByAutoEnroll: stored.CreatedByAutoEnroll,
	}, nil
}

// GetDevicesUsage returns the current usage numbers for Device Trust.
// Meant for usage-based accounts.
func (s *S) GetDevicesUsage(ctx context.Context) (*DevicesUsage, error) {
	return s.getDevicesUsage(ctx, -1 /* limit */)
}

func (s *S) getDevicesUsage(ctx context.Context, limit int) (*DevicesUsage, error) {
	numEnrolled := 0

	const pageSize = 0
	var pageToken string
	for {
		devs, nextPageToken, err := s.ListDevices(ctx, pageSize, pageToken, devicepb.DeviceView_DEVICE_VIEW_LIST)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, dev := range devs {
			if dev.EnrollStatus == devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED {
				numEnrolled++
			}
		}
		if limit > -1 && numEnrolled >= limit {
			break
		}
		if nextPageToken == "" {
			break
		}
		pageToken = nextPageToken
	}

	return &DevicesUsage{
		NumEnrolled: numEnrolled,
	}, nil
}

// VerifyEnrolledDevicesLimit returns an error if the current account is
// usage-based and has reached its enrollment limits, otherwise it returns nil.
// [S.EnrollDevice] will check limits before allowing new enrollments, but this
// method is exposed so we can avoid starting a costly enrollment ceremony if
// the limits are already reached.
func (s *S) VerifyEnrolledDevicesLimit(ctx context.Context) error {
	f := s.modules.Features()
	deviceEntitlement := f.GetEntitlement(entitlements.DeviceTrust)
	if deviceEntitlement.Limit == 0 {
		return nil // unlimited
	}

	limit := int(deviceEntitlement.Limit)
	if limit <= 0 {
		return trace.Wrap(ErrEnrolledDeviceLimit)
	}

	usage, err := s.getDevicesUsage(ctx, limit)
	if err != nil {
		return trace.Wrap(err)
	}
	if usage.NumEnrolled >= limit {
		return trace.Wrap(ErrEnrolledDeviceLimit)
	}

	return nil
}

// CreateDeviceWebToken writes webToken to storage, as part of a new device
// authentication attempt.
//
// Requires all non system-generated fields to be set, including User and
// ExpectedDeviceIDs.
//
// Returns a token with only the fields required to spend it set.
func (s *S) CreateDeviceWebToken(ctx context.Context, webToken *devicepb.DeviceWebToken) (*devicepb.DeviceWebToken, error) {
	if err := validateDeviceWebToken(webToken); err != nil {
		return nil, trace.Wrap(err, "device web token validation")
	}

	token, err := createDeviceToken()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stored := &storedWebAuthenticationAttempt{
		State:             webAuthenticationAttemptCreated,
		HashedWebToken:    token.HashedToken,
		WebSessionID:      webToken.WebSessionId,
		User:              webToken.User,
		BrowserUserAgent:  webToken.BrowserUserAgent,
		BrowserIP:         webToken.BrowserIp,
		ExpectedDeviceIDs: webToken.ExpectedDeviceIds,
	}
	val, err := json.Marshal(stored)
	if err != nil {
		return nil, trace.Wrap(err, "marshal device authentication attempt")
	}

	id := uuid.NewString()
	if _, err := s.backend.Create(ctx, backend.Item{
		Key:     deviceWebAuthenticationAttemptKey(id),
		Value:   val,
		Expires: s.nowUTC().Add(deviceWebAuthnAttemptExpireDuration),
	}); err != nil {
		return nil, trace.Wrap(err, "writing device authentication attempt")
	}

	return &devicepb.DeviceWebToken{
		Id:    id,
		Token: token.SafePlainToken,
	}, nil
}

// SpendDeviceWebToken spends a device web token, returning the spent token on
// success. Only the token itself is verified, further validations are the
// responsibility of the caller.
//
// It expects a token returned by [CreateDeviceWebToken] as input. This method
// optimistically transitions the underlying authentication attempt to the
// Confirm state and issues a DeviceConfirmationToken.
//
// On failures the underlying authentication attempt is deleted.
//
// Returns the stored web token, minus the plaintext token itself, and the
// confirmation token.
func (s *S) SpendDeviceWebToken(
	ctx context.Context,
	webToken *devicepb.DeviceWebToken,
	authenticatedDeviceID string,
) (*devicepb.DeviceWebToken, *devicepb.DeviceConfirmationToken, error) {
	attemptID := webToken.GetId()
	switch {
	case attemptID == "":
		return nil, nil, trace.BadParameter("web token ID required")
	case authenticatedDeviceID == "":
		return nil, nil, trace.BadParameter("authenticated device ID required")
	}

	item, attempt, err := s.getWebAuthnAttempt(ctx, attemptID, webAuthenticationAttemptCreated, func(attempt *storedWebAuthenticationAttempt) error {
		err := matchDeviceToken(webToken.Token, attempt.HashedWebToken)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Prepare the confirmation token.
	confirmToken, err := createDeviceToken()
	if err != nil {
		s.logger.WarnContext(ctx,
			"Failed to issue DeviceConfirmationToken, deleting authentication attempt",
			"error", err,
		)
		return nil, nil, trace.Wrap(err)
	}

	// Transition authentication attempt.
	attempt.State = webAuthenticationAttemptConfirm
	attempt.HashedWebToken = nil
	attempt.HashedConfirmToken = confirmToken.HashedToken
	attempt.AuthenticatedDeviceID = authenticatedDeviceID
	val, err := json.Marshal(attempt)
	if err != nil {
		return nil, nil, trace.Wrap(err, "marshal device authentication attempt")
	}
	if _, err := s.backend.ConditionalUpdate(ctx, backend.Item{
		Key:      item.Key,
		Value:    val,
		Expires:  item.Expires, // Keep original expiration.
		Revision: item.Revision,
	}); err != nil {
		return nil, nil, trace.Wrap(err, "update device authentication attempt")
	}

	storedWebToken := &devicepb.DeviceWebToken{
		Id:                attemptID,
		WebSessionId:      attempt.WebSessionID,
		BrowserUserAgent:  attempt.BrowserUserAgent,
		BrowserIp:         attempt.BrowserIP,
		User:              attempt.User,
		ExpectedDeviceIds: attempt.ExpectedDeviceIDs,
	}

	storedConfirmToken := &devicepb.DeviceConfirmationToken{
		Id:    attemptID,
		Token: confirmToken.SafePlainToken,
	}

	return storedWebToken, storedConfirmToken, nil
}

// DeviceConfirmationTokenData represents data associated to a stored
// DeviceConfirmationToken.
type DeviceConfirmationTokenData struct {
	// WebSessionID is the WebSession identifier.
	WebSessionID string
	// User is the owner of the session and authenticated device.
	User string
	// BrowserIP is the IP of the browser that started device web authentication.
	BrowserIP string
	// AuthenticatedDeviceID is the ID of the authenticated device.
	AuthenticatedDeviceID string
}

// SpendDeviceConfirmationToken spends a DeviceConfirmationToken issued by
// [SpendDeviceWebToken].
//
// A successfully spent token is a pre-requisite for issuing augmented
// certificates for the WebSession.
//
// The caller must inspect the [DeviceConfirmationTokenData] and validate it
// against the request before issuing augmented certificates.
func (s *S) SpendDeviceConfirmationToken(ctx context.Context, confirmToken *devicepb.DeviceConfirmationToken) (*DeviceConfirmationTokenData, error) {
	attemptID := confirmToken.GetId()
	if attemptID == "" {
		return nil, trace.BadParameter("confirmation token ID required")
	}

	// Read/validate the token.
	item, attempt, err := s.getWebAuthnAttempt(ctx, attemptID, webAuthenticationAttemptConfirm, func(attempt *storedWebAuthenticationAttempt) error {
		err := matchDeviceToken(confirmToken.Token, attempt.HashedConfirmToken)
		return trace.Wrap(err)
	})
	// err handled below.

	var tokenData *DeviceConfirmationTokenData
	if attempt != nil {
		tokenData = &DeviceConfirmationTokenData{
			WebSessionID:          attempt.WebSessionID,
			User:                  attempt.User,
			BrowserIP:             attempt.BrowserIP,
			AuthenticatedDeviceID: attempt.AuthenticatedDeviceID,
		}
	}
	// Return tokenData from here onwards, it helps with audit.

	if err != nil {
		return tokenData, trace.Wrap(err)
	}

	// "Spend" it.
	err = s.backend.Delete(ctx, item.Key)
	return tokenData, trace.Wrap(err, "spend device confirmation token")
}

// DeleteDeviceWebAuthenticationAttempt deletes the device web authentication
// attempt that underlies a DeviceWebToken or DeviceConfirmationToken.
//
// The deletion is unconditional.
func (s *S) DeleteDeviceWebAuthenticationAttempt(ctx context.Context, attemptID string) error {
	if attemptID == "" {
		return trace.BadParameter("attempt ID required")
	}

	err := s.backend.Delete(ctx, deviceWebAuthenticationAttemptKey(attemptID))
	return trace.Wrap(err)
}

func (s *S) getWebAuthnAttempt(
	ctx context.Context,
	attemptID string,
	expectedState webAuthenticationAttemptState,
	validate func(*storedWebAuthenticationAttempt) error,
) (*backend.Item, *storedWebAuthenticationAttempt, error) {
	// Read and unmarshal the attempt.
	key := deviceWebAuthenticationAttemptKey(attemptID)
	item, err := s.backend.Get(ctx, key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var attempt storedWebAuthenticationAttempt
	if err := json.Unmarshal(item.Value, &attempt); err != nil {
		return nil, nil, trace.Wrap(err, "unmarshal device authentication attempt")
	}

	silentDeleteAttempt := func() {
		if err := s.backend.Delete(ctx, key); err != nil {
			s.logger.WarnContext(ctx,
				"Failed to delete device authentication attempt",
				"error", err,
				"attempt_id", attemptID,
			)
			// err swallowed on purpose.
		}
	}

	// Verify the attempt state.
	if attempt.State != expectedState {
		// A state mismatch here is likely a double-spend attempt.
		silentDeleteAttempt()
		return nil, nil, trace.BadParameter("device authentication attempt state mismatch")
	}

	// Validate the attempt.
	if err := validate(&attempt); err != nil {
		silentDeleteAttempt()
		// Return item and attempt for audit purposes.
		return item, &attempt, trace.Wrap(err)
	}

	return item, &attempt, nil
}

func deviceIDFromKey(key backend.Key) string {
	components := key.Components()
	return components[len(components)-1]
}

func storedToDeviceView(deviceID string, sd *storedDevice, view devicepb.DeviceView) *devicepb.Device {
	var source *devicepb.DeviceSource
	if sd.Source != nil {
		source = &devicepb.DeviceSource{
			Name:   sd.Source.Name,
			Origin: devicepb.DeviceOrigin(sd.Source.Origin),
		}
	}

	// If "list" provide only basic device information.
	// Suitable for viewing multiple devices at once, as in "tctl devices ls".
	if view == devicepb.DeviceView_DEVICE_VIEW_LIST {
		return &devicepb.Device{
			ApiVersion:   currentAPIVersion,
			Id:           deviceID,
			OsType:       devicepb.OSType(sd.OSType),
			Owner:        sd.Owner,
			AssetTag:     sd.AssetTag,
			CreateTime:   timestamppb.New(sd.CreateTime),
			UpdateTime:   timestamppb.New(sd.UpdateTime),
			EnrollStatus: devicepb.DeviceEnrollStatus(sd.EnrollStatus),
			Source:       source,
		}
	}

	// Full device information.
	var cred *devicepb.DeviceCredential
	if c := sd.Credential; c != nil {
		cred = &devicepb.DeviceCredential{
			Id:                    c.ID,
			PublicKeyDer:          c.PublicKeyDER,
			DeviceAttestationType: devicepb.DeviceAttestationType(c.DeviceAttestationType),
			TpmEkcertSerial:       c.TPMEKCertSerial,
			TpmAkPublic:           c.TPMAKPublic,
		}
	}

	var profile *devicepb.DeviceProfile
	if sd.Profile != nil {
		profile = &devicepb.DeviceProfile{
			UpdateTime:          timestamppb.New(sd.Profile.UpdateTime),
			ModelIdentifier:     sd.Profile.ModelIdentifier,
			OsVersion:           sd.Profile.OSVersion,
			OsBuild:             sd.Profile.OSBuild,
			OsBuildSupplemental: sd.Profile.OSBuildSupplemental,
			OsUsernames:         sd.Profile.OSUsernames,
			JamfBinaryVersion:   sd.Profile.JamfBinaryVersion,
			ExternalId:          sd.Profile.ExternalID,
			OsId:                sd.Profile.OSID,
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
		Source:       source,
		Profile:      profile,
		Owner:        sd.Owner,
	}
}

func storedToDevice(deviceID string, sd *storedDevice) *devicepb.Device {
	return storedToDeviceView(deviceID, sd, devicepb.DeviceView_DEVICE_VIEW_RESOURCE)
}

func collectedDataToStored(cd *devicepb.DeviceCollectedData, origin collectedDataOrigin, recordTime time.Time, createAsResource bool) *storedCollectedData {
	storedCD := &storedCollectedData{
		Origin:                  origin,
		CollectTime:             cd.CollectTime.AsTime(),
		RecordTime:              recordTime,
		OSType:                  int(cd.OsType),
		SerialNumber:            cd.SerialNumber,
		ModelIdentifier:         cd.ModelIdentifier,
		OSVersion:               cd.OsVersion,
		OSBuild:                 cd.OsBuild,
		OSUsername:              cd.OsUsername,
		OSLoginUser:             cd.OsLoginUser,
		JamfBinaryVersion:       cd.JamfBinaryVersion,
		MacOSEnrollmentProfiles: cd.MacosEnrollmentProfiles,
		ReportedAssetTag:        cd.ReportedAssetTag,
		SystemSerialNumber:      cd.SystemSerialNumber,
		BaseBoardSerialNumber:   cd.BaseBoardSerialNumber,
		TPMPlatformAttestation:  tpmPlatformAttestationToStored(cd.TpmPlatformAttestation),
		OSID:                    cd.OsId,
	}

	if !createAsResource {
		return storedCD
	}

	if cd.RecordTime != nil {
		storedCD.RecordTime = cd.RecordTime.AsTime()
	}

	return storedCD
}

func storedToCollectedData(stored *storedCollectedData) *devicepb.DeviceCollectedData {
	return &devicepb.DeviceCollectedData{
		CollectTime:             timestamppb.New(stored.CollectTime),
		RecordTime:              timestamppb.New(stored.RecordTime),
		OsType:                  devicepb.OSType(stored.OSType),
		SerialNumber:            stored.SerialNumber,
		ModelIdentifier:         stored.ModelIdentifier,
		OsVersion:               stored.OSVersion,
		OsBuild:                 stored.OSBuild,
		OsUsername:              stored.OSUsername,
		OsLoginUser:             stored.OSLoginUser,
		JamfBinaryVersion:       stored.JamfBinaryVersion,
		MacosEnrollmentProfiles: stored.MacOSEnrollmentProfiles,
		ReportedAssetTag:        stored.ReportedAssetTag,
		SystemSerialNumber:      stored.SystemSerialNumber,
		BaseBoardSerialNumber:   stored.BaseBoardSerialNumber,
		TpmPlatformAttestation:  tpmPlatformAttestationFromStored(stored.TPMPlatformAttestation),
		OsId:                    stored.OSID,
	}
}

func tpmPlatformAttestationToStored(pa *devicepb.TPMPlatformAttestation) *tpmPlatformAttestation {
	if pa == nil {
		return nil
	}

	var pp *tpmPlatformParameters
	if pa.PlatformParameters != nil {
		var quotes []tpmQuote
		for _, q := range pa.PlatformParameters.Quotes {
			quotes = append(quotes, tpmQuote{
				Quote:     q.Quote,
				Signature: q.Signature,
			})
		}
		var pcrs []tpmPCR
		for _, pcr := range pa.PlatformParameters.Pcrs {
			pcrs = append(pcrs, tpmPCR{
				Index:     pcr.Index,
				Digest:    pcr.Digest,
				DigestAlg: pcr.DigestAlg,
			})
		}

		pp = &tpmPlatformParameters{
			Quotes:   quotes,
			PCRs:     pcrs,
			EventLog: pa.PlatformParameters.EventLog,
		}
	}

	return &tpmPlatformAttestation{
		Nonce:              pa.Nonce,
		PlatformParameters: pp,
	}
}

func tpmPlatformAttestationFromStored(stored *tpmPlatformAttestation) *devicepb.TPMPlatformAttestation {
	if stored == nil {
		return nil
	}

	var pp *devicepb.TPMPlatformParameters
	if stored.PlatformParameters != nil {
		var quotes []*devicepb.TPMQuote
		for _, q := range stored.PlatformParameters.Quotes {
			quotes = append(quotes, &devicepb.TPMQuote{
				Quote:     q.Quote,
				Signature: q.Signature,
			})
		}
		var pcrs []*devicepb.TPMPCR
		for _, pcr := range stored.PlatformParameters.PCRs {
			pcrs = append(pcrs, &devicepb.TPMPCR{
				Index:     pcr.Index,
				Digest:    pcr.Digest,
				DigestAlg: pcr.DigestAlg,
			})
		}

		pp = &devicepb.TPMPlatformParameters{
			Quotes:   quotes,
			Pcrs:     pcrs,
			EventLog: stored.PlatformParameters.EventLog,
		}
	}

	return &devicepb.TPMPlatformAttestation{
		Nonce:              stored.Nonce,
		PlatformParameters: pp,
	}
}

func deviceKeyStart() backend.Key {
	return backend.NewKey(devicetrust.DevicesIDPrefix...)
}

func deviceKey(deviceID string) backend.Key {
	return backend.NewKey(append(devicetrust.DevicesIDPrefix, deviceID)...)
}

func deviceTokenKey(deviceID string) backend.Key {
	return backend.NewKey("devices", "enroll_token", deviceID)
}

func deviceWebAuthenticationAttemptKey(attemptID string) backend.Key {
	return backend.NewKey("devices", "web_authn_attempt", attemptID)
}

func devicesByAssetTagKey(assetTag string) backend.Key {
	return backend.NewKey("devices", "byTag", assetTag)
}

func devicesByUserKey(user string) backend.Key {
	return backend.NewKey("devices", "by_user", user)
}

func collectedDataKey(deviceID, cdID string) backend.Key {
	return backend.NewKey("devices", "collected_data", deviceID, cdID)
}

func collectedDataKeyStart(deviceID string) backend.Key {
	return backend.NewKey("devices", "collected_data", deviceID)
}
