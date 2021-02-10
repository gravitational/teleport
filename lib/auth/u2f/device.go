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

	"github.com/flynn/u2f/u2fhid"
	"github.com/flynn/u2f/u2ftoken"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tstranex/u2f"

	"github.com/gravitational/teleport/api/types"
)

// DeviceStorage is a persistent storage for MFA devices.
type DeviceStorage interface {
	UpsertMFADevice(ctx context.Context, key string, d *types.MFADevice) error
}

// NewDevice creates a U2F MFADevice object from a completed U2F registration.
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

// pollLocalDevices calls fn against all local U2F devices until one succeeds
// (fn returns nil) or until the context is cancelled.
func pollLocalDevices(ctx context.Context, fn func(t *u2ftoken.Token) error) error {
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		err := foreachLocalDevice(fn)
		if err == nil {
			return nil
		}
		// Don't spam the logs while we're waiting for a key to be plugged
		// in or tapped.
		if err != errAuthNoKeyOrUserPresence {
			logrus.WithError(err).Debugf("Error polling U2F devices for registration")
		}

		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-tick.C:
		}
	}
}

// foreachLocalDevice runs fn against each currently available U2F device. It
// stops when fn returns nil or when finished iterating against the available
// devices.
//
// You most likely want to call pollLocalDevices instead.
func foreachLocalDevice(fn func(t *u2ftoken.Token) error) error {
	// Note: we fetch and open all devices on every polling iteration on
	// purpose. This will handle the device that a user inserts after the
	// polling has started.
	devices, err := u2fhid.Devices()
	if err != nil {
		return trace.Wrap(err)
	}
	var errs []error
	for _, d := range devices {
		dev, err := u2fhid.Open(d)
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		// There are usually 1-2 devices plugged in, so deferring closing all
		// of them until the end of function is not too wasteful.
		defer dev.Close()

		t := u2ftoken.NewToken(dev)
		// fn is usually t.Authenticate or t.Register. Both methods are
		// non-blocking - the U2F device returns u2ftoken.ErrPresenceRequired
		// immediately, unless the user has recently tapped the device.
		if err := fn(t); err == u2ftoken.ErrPresenceRequired || err == errAuthNoKeyOrUserPresence {
			continue
		} else if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		return nil
	}

	if len(errs) > 0 {
		return trace.NewAggregate(errs...)
	}
	return errAuthNoKeyOrUserPresence
}
