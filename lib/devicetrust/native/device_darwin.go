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

// #cgo CFLAGS: -Wall -xobjective-c -fblocks -fobjc-arc -mmacosx-version-min=10.13
// #cgo LDFLAGS: -framework CoreFoundation -framework Foundation -framework IOKit -framework Security
// #include <stdint.h>
// #include <stdlib.h>
// #include "device_darwin.h"
import "C"

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"os/user"
	"strings"
	"sync"
	"unsafe"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/darwin"
)

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	cred, err := deviceKeyGetOrCreate()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cd, err := CollectDeviceData(CollectedDataAlwaysEscalate)
	if err != nil {
		return nil, trace.Wrap(err, "collecting device data")
	}

	return &devicepb.EnrollDeviceInit{
		CredentialId: cred.Id,
		DeviceData:   cd,
		Macos: &devicepb.MacOSEnrollPayload{
			PublicKeyDer: cred.PublicKeyDer,
		},
	}, nil
}

func deviceKeyGetOrCreate() (*devicepb.DeviceCredential, error) {
	newID := uuid.NewString()
	newIDC := C.CString(newID)
	defer C.free(unsafe.Pointer(newIDC))

	var pubKeyC C.PublicKey
	defer func() {
		C.free(unsafe.Pointer(pubKeyC.id))
		C.free(unsafe.Pointer(pubKeyC.pub_key))
	}()

	if res := C.DeviceKeyGetOrCreate(newIDC, &pubKeyC); res != 0 {
		return nil, trace.Wrap(statusErrorFromC(res))
	}

	id := C.GoString(pubKeyC.id)
	pubKeyRaw := C.GoBytes(unsafe.Pointer(pubKeyC.pub_key), C.int(pubKeyC.pub_key_len))
	return pubKeyToCredential(id, pubKeyRaw)
}

func pubKeyToCredential(id string, pubKeyRaw []byte) (*devicepb.DeviceCredential, error) {
	ecPubKey, err := darwin.ECDSAPublicKeyFromRaw(pubKeyRaw)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pubKeyDER, err := x509.MarshalPKIXPublicKey(ecPubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &devicepb.DeviceCredential{
		Id:           id,
		PublicKeyDer: pubKeyDER,
	}, nil
}

func collectDeviceData(_ CollectDataMode) (*devicepb.DeviceCollectedData, error) {
	var dd C.DeviceData
	defer func() {
		C.free(unsafe.Pointer(dd.serial_number))
		C.free(unsafe.Pointer(dd.model))
		C.free(unsafe.Pointer(dd.os_version_string))
	}()

	if res := C.DeviceCollectData(&dd); res != 0 {
		return nil, trace.Wrap(statusErrorFromC(res))
	}

	osUser, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err, "reading current user")
	}

	// Run exec-ed commands concurrently.
	var wg sync.WaitGroup
	// Note: We could read the OS build from dd.os_version_string, but this
	// requires no string parsing.
	var osBuild, jamfVersion, macosEnrollmentProfiles string
	for _, spec := range []struct {
		fn   func() (string, error)
		out  *string
		desc string
	}{
		{fn: getOSBuild, out: &osBuild, desc: "macOS build"},
		{fn: getJamfBinaryVersion, out: &jamfVersion, desc: "Jamf version"},
		{fn: getMacosEnrollmentProfiles, out: &macosEnrollmentProfiles, desc: "macOs enrollment profiles"},
	} {
		spec := spec
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := spec.fn()
			if err != nil {
				log.WithError(err).Warnf("Device Trust: Failed to get %v", spec.desc)
				return
			}
			*spec.out = out
		}()
	}
	wg.Wait()

	sn := C.GoString(dd.serial_number)
	return &devicepb.DeviceCollectedData{
		CollectTime:             timestamppb.Now(),
		OsType:                  devicepb.OSType_OS_TYPE_MACOS,
		SerialNumber:            sn,
		ModelIdentifier:         C.GoString(dd.model),
		OsVersion:               fmt.Sprintf("%v.%v.%v", dd.os_major, dd.os_minor, dd.os_patch),
		OsBuild:                 osBuild,
		OsUsername:              osUser.Username,
		JamfBinaryVersion:       jamfVersion,
		MacosEnrollmentProfiles: macosEnrollmentProfiles,
		SystemSerialNumber:      sn,
	}, nil
}

func getOSBuild() (string, error) {
	cmd := exec.Command("/usr/bin/sw_vers", "-buildVersion")
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err, "running sw_vers -buildVersion")
	}
	return string(bytes.TrimSpace(out)), nil
}

func getJamfBinaryVersion() (string, error) {
	// See https://learn.jamf.com/bundle/jamf-pro-documentation-current/page/Components_Installed_on_Managed_Computers.html
	cmd := exec.Command("/usr/local/bin/jamf", "version")
	out, err := cmd.Output()
	if err != nil {
		// Jamf binary may not exist. This is alright.
		pathErr := &fs.PathError{}
		if errors.As(err, &pathErr) {
			log.Debugf("Device Trust: Jamf binary not found: %q", pathErr.Path)
			return "", nil
		}

		return "", trace.Wrap(err, "running jamf version")
	}

	// Eg: "version=10.46.1-t1683911857"
	s := string(bytes.TrimSpace(out))
	tmp := strings.Split(s, "=")
	if len(tmp) != 2 {
		return "", fmt.Errorf("unexpected jamf version string: %q", s)
	}

	return tmp[1], nil
}

func getMacosEnrollmentProfiles() (string, error) {
	cmd := exec.Command("/usr/bin/profiles", "status", "-type", "enrollment")
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err, "running /usr/bin/profiles status -type enrollment")
	}
	return string(bytes.TrimSpace(out)), nil
}

func signChallenge(chal []byte) (sig []byte, err error) {
	h := sha256.Sum256(chal)
	digC := C.Digest{
		data:     (*C.uint8_t)(C.CBytes(h[:])),
		data_len: (C.size_t)(len(h)),
	}
	defer func() { C.free(unsafe.Pointer(digC.data)) }()

	var sigC C.Signature
	defer func() { C.free(unsafe.Pointer(sigC.data)) }()

	if res := C.DeviceKeySign(digC, &sigC); res != 0 {
		return nil, trace.Wrap(statusErrorFromC(res))
	}

	sig = C.GoBytes(unsafe.Pointer(sigC.data), C.int(sigC.data_len))
	return sig, err
}

func getDeviceCredential() (*devicepb.DeviceCredential, error) {
	var pubKeyC C.PublicKey
	defer func() {
		C.free(unsafe.Pointer(pubKeyC.id))
		C.free(unsafe.Pointer(pubKeyC.pub_key))
	}()

	if res := C.DeviceKeyGet(&pubKeyC); res != 0 {
		return nil, trace.Wrap(statusErrorFromC(res))
	}

	id := C.GoString(pubKeyC.id)
	pubKeyRaw := C.GoBytes(unsafe.Pointer(pubKeyC.pub_key), C.int(pubKeyC.pub_key_len))
	return pubKeyToCredential(id, pubKeyRaw)
}

func statusErrorFromC(res C.int32_t) error {
	return &statusError{status: int32(res)}
}

func solveTPMEnrollChallenge(_ *devicepb.TPMEnrollChallenge, _ bool) (*devicepb.TPMEnrollChallengeResponse, error) {
	return nil, trace.BadParameter("called solveTPMEnrollChallenge on darwin")
}

func solveTPMAuthnDeviceChallenge(_ *devicepb.TPMAuthenticateDeviceChallenge) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	return nil, trace.BadParameter("called solveTPMAuthnDeviceChallenge on darwin")
}

func handleTPMActivateCredential(_, _ string) error {
	return trace.BadParameter("called handleTPMActivateCredential on darwin")
}
