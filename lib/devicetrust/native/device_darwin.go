// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package native

// #cgo CFLAGS: -Wall -xobjective-c -fblocks -fobjc-arc -mmacosx-version-min=10.13
// #cgo LDFLAGS: -framework CoreFoundation -framework Foundation -framework IOKit -framework Security
// #include <stdint.h>
// #include <stdlib.h>
// #include "device_darwin.h"
import "C"

import (
	"crypto/sha256"
	"crypto/x509"
	"unsafe"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/darwin"
)

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	cred, err := deviceKeyGetOrCreate()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cd, err := collectDeviceData()
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

func collectDeviceData() (*devicepb.DeviceCollectedData, error) {
	var dd C.DeviceData
	defer func() { C.free(unsafe.Pointer(dd.serial_number)) }()

	if res := C.DeviceCollectData(&dd); res != 0 {
		return nil, trace.Wrap(statusErrorFromC(res))
	}

	return &devicepb.DeviceCollectedData{
		CollectTime:  timestamppb.Now(),
		OsType:       devicepb.OSType_OS_TYPE_MACOS,
		SerialNumber: C.GoString(dd.serial_number),
	}, nil
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
