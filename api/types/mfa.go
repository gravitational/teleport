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

package types

import (
	"bytes"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// NewMFADevice creates a new MFADevice with the given name. Caller must set
// the Device field in the returned MFADevice.
func NewMFADevice(name, id string, addedAt time.Time, device isMFADevice_Device) (*MFADevice, error) {
	dev := &MFADevice{
		Metadata: Metadata{
			Name: name,
		},
		Id:       id,
		AddedAt:  addedAt,
		LastUsed: addedAt,
		Device:   device,
	}
	return dev, dev.CheckAndSetDefaults()
}

// setStaticFields sets static resource header and metadata fields.
func (d *MFADevice) setStaticFields() {
	d.Kind = KindMFADevice
	d.Version = V1
}

// CheckAndSetDefaults validates MFADevice fields and populates empty fields
// with default values.
func (d *MFADevice) CheckAndSetDefaults() error {
	d.setStaticFields()
	if err := d.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if d.Id == "" {
		return trace.BadParameter("MFADevice missing ID field")
	}
	if d.AddedAt.IsZero() {
		return trace.BadParameter("MFADevice missing AddedAt field")
	}
	if d.LastUsed.IsZero() {
		return trace.BadParameter("MFADevice missing LastUsed field")
	}
	if d.LastUsed.Before(d.AddedAt) {
		return trace.BadParameter("MFADevice LastUsed field must be earlier than AddedAt")
	}
	if d.Device == nil {
		return trace.BadParameter("MFADevice missing Device field")
	}
	if err := d.validateDevice(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// validateDevice runs additional validations for OTP devices.
// Prefer adding new validation logic to types.MFADevice.CheckAndSetDefaults
// instead.
func (d *MFADevice) validateDevice() error {
	switch dev := d.Device.(type) {
	case *MFADevice_Totp:
		if dev.Totp == nil {
			return trace.BadParameter("MFADevice has malformed TOTPDevice")
		}
		if dev.Totp.Key == "" {
			return trace.BadParameter("TOTPDevice missing Key field")
		}
	case *MFADevice_Webauthn:
		if dev.Webauthn == nil {
			return trace.BadParameter("MFADevice has malformed WebauthnDevice")
		}
		if len(dev.Webauthn.CredentialId) == 0 {
			return trace.BadParameter("WebauthnDevice missing CredentialId field")
		}
		if len(dev.Webauthn.PublicKeyCbor) == 0 {
			return trace.BadParameter("WebauthnDevice missing PublicKeyCbor field")
		}
	case *MFADevice_Sso:
		if dev.Sso == nil {
			return trace.BadParameter("MFADevice has malformed SSODevice")
		}
		if dev.Sso.ConnectorId == "" {
			return trace.BadParameter("SSODevice missing ConnectorId field")
		}
		if dev.Sso.ConnectorType == "" {
			return trace.BadParameter("SSODevice missing ConnectorType field")
		}
	case *MFADevice_U2F:
	default:
		return trace.BadParameter("MFADevice has Device field of unknown type %T", dev)
	}
	return nil
}

func (d *MFADevice) WithoutSensitiveData() (*MFADevice, error) {
	if d == nil {
		return nil, trace.BadParameter("cannot hide sensitive data on empty object")
	}
	out := utils.CloneProtoMsg(d)

	switch mfad := out.Device.(type) {
	case *MFADevice_Totp:
		mfad.Totp.Key = ""
	case *MFADevice_U2F:
		// OK, no sensitive secrets.
	case *MFADevice_Webauthn:
		// OK, no sensitive secrets.
	case *MFADevice_Sso:
		// OK, no sensitive secrets.
	default:
		return nil, trace.BadParameter("unsupported MFADevice type %T", d.Device)
	}

	return out, nil
}

func (d *MFADevice) GetKind() string         { return d.Kind }
func (d *MFADevice) GetSubKind() string      { return d.SubKind }
func (d *MFADevice) SetSubKind(sk string)    { d.SubKind = sk }
func (d *MFADevice) GetVersion() string      { return d.Version }
func (d *MFADevice) GetMetadata() Metadata   { return d.Metadata }
func (d *MFADevice) GetName() string         { return d.Metadata.GetName() }
func (d *MFADevice) SetName(n string)        { d.Metadata.SetName(n) }
func (d *MFADevice) GetRevision() string     { return d.Metadata.GetRevision() }
func (d *MFADevice) SetRevision(rev string)  { d.Metadata.SetRevision(rev) }
func (d *MFADevice) Expiry() time.Time       { return d.Metadata.Expiry() }
func (d *MFADevice) SetExpiry(exp time.Time) { d.Metadata.SetExpiry(exp) }

// MFAType returns the human-readable name of the MFA protocol of this device.
func (d *MFADevice) MFAType() string {
	switch d.Device.(type) {
	case *MFADevice_Totp:
		return "TOTP"
	case *MFADevice_U2F:
		return "U2F"
	case *MFADevice_Webauthn:
		return "WebAuthn"
	case *MFADevice_Sso:
		return "SSO"
	default:
		return "unknown"
	}
}

func (d *MFADevice) MarshalJSON() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := (&jsonpb.Marshaler{}).Marshal(buf, d)
	return buf.Bytes(), trace.Wrap(err)
}

func (d *MFADevice) UnmarshalJSON(buf []byte) error {
	unmarshaler := jsonpb.Unmarshaler{AllowUnknownFields: true}
	err := unmarshaler.Unmarshal(bytes.NewReader(buf), d)
	return trace.Wrap(err)
}
