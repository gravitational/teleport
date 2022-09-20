/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package u2f

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/tstranex/u2f"
)

// DeviceStorage is a persistent storage for MFA devices.
type DeviceStorage interface {
	UpsertMFADevice(ctx context.Context, key string, d *types.MFADevice) error
}

// NewDevice creates a U2F MFADevice object from a completed U2F registration.
//
// This function is not in lib/services due to an import cycle (lib/services
// depends on lib/auth/u2f).
func NewDevice(name string, reg *Registration, addedAt time.Time) (*types.MFADevice, error) {
	d := types.NewMFADevice(name, uuid.New(), addedAt)
	pubKey, err := x509.MarshalPKIXPublicKey(&reg.PubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	d.Device = &types.MFADevice_U2F{U2F: &types.U2FDevice{
		KeyHandle: reg.KeyHandle,
		PubKey:    pubKey,
	}}
	if err := d.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ValidateDevice(d.GetU2F()); err != nil {
		return nil, trace.Wrap(err)
	}
	return d, nil
}

// ValidateMFADevice validates a U2F MFA device.
//
// This function is not in lib/services due to an import cycle (lib/services
// depends on lib/auth/u2f).
func ValidateDevice(d *types.U2FDevice) error {
	if len(d.KeyHandle) == 0 {
		return trace.BadParameter("U2FDevice missing KeyHandle field")
	}
	if len(d.PubKey) == 0 {
		return trace.BadParameter("U2FDevice missing PubKey field")
	}
	if _, err := decodeDevicePubKey(d); err != nil {
		return trace.BadParameter("U2FDevice PubKey is invalid: %v", err)
	}
	return nil
}

// DeviceToRegistration decodes the U2F registration data and builds the
// expected registration object.
func DeviceToRegistration(d *types.U2FDevice) (*Registration, error) {
	pubKey, err := decodeDevicePubKey(d)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &u2f.Registration{
		KeyHandle: d.KeyHandle,
		PubKey:    *pubKey,
	}, nil
}

func decodeDevicePubKey(d *types.U2FDevice) (*ecdsa.PublicKey, error) {
	pubKeyI, err := x509.ParsePKIXPublicKey(d.PubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pubKey, ok := pubKeyI.(*ecdsa.PublicKey)
	if !ok {
		return nil, trace.BadParameter("expected *ecdsa.PublicKey, got %T", pubKeyI)
	}
	return pubKey, nil
}
