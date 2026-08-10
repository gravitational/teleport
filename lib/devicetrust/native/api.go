/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package native

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// EnrollDeviceInit creates the initial enrollment data for the device.
// This includes fetching or creating a device credential, collecting device
// data and filling in any OS-specific fields.
func EnrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	return enrollDeviceInit()
}

// CollectDataMode is the mode of collection used by CollectDeviceData.
type CollectDataMode int

const (
	// CollectedDataNeverEscalate will never escalate privileges, even in the
	// absence of cached data.
	CollectedDataNeverEscalate CollectDataMode = iota
	// CollectedDataMaybeEscalate will attempt to use cached DMI data before
	// privilege escalation, but it may choose to escalate if no cached data is
	// available.
	//
	// Used by `tsh login` and similar operations (ie, device authn).
	CollectedDataMaybeEscalate
	// CollectedDataAlwaysEscalate avoids using cached DMI data and instead will
	// always escalate privileges if necessary.
	//
	// Used by `tsh device enroll`, `tsh device collect` and
	// `tsh device asset-tag`.
	CollectedDataAlwaysEscalate
	// IMPORTANT: CollectDataMode declarations must go from least to most strict.
)

var cachedDeviceData = struct {
	skipCache bool // Set to true for testing.

	mu    sync.Mutex
	mode  CollectDataMode
	value *devicepb.DeviceCollectedData
}{}

func readCachedDeviceDataUnderLock(mode CollectDataMode) (cdd *devicepb.DeviceCollectedData, ok bool) {
	// Use cached data if present and the cached mode is at least as strict as the
	// one requested.
	// This can save some time, but mainly it avoids needless escalation attempts
	// on Linux (past the first).
	if cachedDeviceData.skipCache || cachedDeviceData.mode < mode || cachedDeviceData.value == nil {
		return nil, false
	}

	// Default sudo cache is around 5m, so this seems like a resonable interval.
	const maxAgeSeconds = 60
	cdd = cachedDeviceData.value
	now := time.Now()
	if now.Unix()-cdd.CollectTime.Seconds > maxAgeSeconds {
		// "Evict" cache.
		cachedDeviceData.mode = 0
		cachedDeviceData.value = nil
		return nil, false
	}

	slog.DebugContext(context.Background(), "Device Trust: Using in-process cached device data")
	cdd = proto.Clone(cachedDeviceData.value).(*devicepb.DeviceCollectedData)
	cdd.CollectTime = timestamppb.Now()
	return cdd, true
}

func writeCachedDeviceDataUnderLock(mode CollectDataMode, cdd *devicepb.DeviceCollectedData) {
	cachedDeviceData.mode = mode
	cachedDeviceData.value = proto.Clone(cdd).(*devicepb.DeviceCollectedData)
}

// CollectDeviceData collects OS-specific device data for device enrollment or
// device authentication ceremonies.
func CollectDeviceData(mode CollectDataMode) (*devicepb.DeviceCollectedData, error) {
	cachedDeviceData.mu.Lock()
	defer cachedDeviceData.mu.Unlock()

	if cdd, ok := readCachedDeviceDataUnderLock(mode); ok {
		return cdd, nil
	}

	cdd, err := collectDeviceData(mode)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	writeCachedDeviceDataUnderLock(mode, cdd)
	return cdd, nil
}

// SignChallenge signs a device challenge for device enrollment or device
// authentication ceremonies.
func SignChallenge(chal []byte) (sig []byte, err error) {
	return signChallenge(chal)
}

// GetDeviceCredential returns the current device credential, if it exists.
func GetDeviceCredential() (*devicepb.DeviceCredential, error) {
	return getDeviceCredential()
}

// SolveTPMEnrollChallenge completes a TPM enrollment challenge.
func SolveTPMEnrollChallenge(challenge *devicepb.TPMEnrollChallenge, debug bool) (*devicepb.TPMEnrollChallengeResponse, error) {
	return solveTPMEnrollChallenge(challenge, debug)
}

// SolveTPMAuthnDeviceChallenge completes a TPM device authetication challenge.
func SolveTPMAuthnDeviceChallenge(challenge *devicepb.TPMAuthenticateDeviceChallenge) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	return solveTPMAuthnDeviceChallenge(challenge)
}

// HandleTPMActivateCredential completes the credential activation part of an
// enrollment challenge. This is usually called in an elevated process that's
// created by SolveTPMEnrollChallenge.
//
//nolint:staticcheck // HandleTPMActivateCredential works depending on the platform.
func HandleTPMActivateCredential(encryptedCredential, encryptedCredentialSecret string) error {
	return handleTPMActivateCredential(encryptedCredential, encryptedCredentialSecret)
}

// GetDeviceOSType returns the devicepb.OSType for the current OS
func GetDeviceOSType() devicepb.OSType {
	switch runtime.GOOS {
	case "darwin":
		return devicepb.OSType_OS_TYPE_MACOS
	case "linux":
		return devicepb.OSType_OS_TYPE_LINUX
	case "windows":
		return devicepb.OSType_OS_TYPE_WINDOWS
	default:
		return devicepb.OSType_OS_TYPE_UNSPECIFIED
	}
}
