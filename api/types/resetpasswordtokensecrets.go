/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"fmt"
	"time"

	"github.com/gravitational/trace"
)

// ResetPasswordTokenSecrets contains token secrets
type ResetPasswordTokenSecrets interface {
	// Resource provides common resource properties
	Resource
	// GetCreated returns Created
	GetCreated() time.Time
	// SetCreated sets Created
	SetCreated(time.Time)
	// GetQRCode returns QRCode
	GetQRCode() []byte
	// SetQRCode sets QRCode
	SetQRCode([]byte)
	// GetOTPKey returns OTP key
	GetOTPKey() string
	// SetOTPKey sets OTP Key
	SetOTPKey(string)
}

// NewResetPasswordTokenSecrets creates an instance of ResetPasswordTokenSecrets.
func NewResetPasswordTokenSecrets(tokenID string) (ResetPasswordTokenSecrets, error) {
	secrets := ResetPasswordTokenSecretsV3{
		Metadata: Metadata{
			Name: tokenID,
		},
	}
	if err := secrets.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &secrets, nil
}

// GetName returns Name
func (u *ResetPasswordTokenSecretsV3) GetName() string {
	return u.Metadata.Name
}

// GetCreated returns Created
func (u *ResetPasswordTokenSecretsV3) GetCreated() time.Time {
	return u.Spec.Created
}

// SetCreated sets Created
func (u *ResetPasswordTokenSecretsV3) SetCreated(t time.Time) {
	u.Spec.Created = t
}

// GetOTPKey returns OTP Key
func (u *ResetPasswordTokenSecretsV3) GetOTPKey() string {
	return u.Spec.OTPKey
}

// SetOTPKey sets OTP Key
func (u *ResetPasswordTokenSecretsV3) SetOTPKey(key string) {
	u.Spec.OTPKey = key
}

// GetQRCode returns QRCode
func (u *ResetPasswordTokenSecretsV3) GetQRCode() []byte {
	return []byte(u.Spec.QRCode)
}

// SetQRCode sets QRCode
func (u *ResetPasswordTokenSecretsV3) SetQRCode(code []byte) {
	u.Spec.QRCode = string(code)
}

// Expiry returns object expiry setting
func (u *ResetPasswordTokenSecretsV3) Expiry() time.Time {
	return u.Metadata.Expiry()
}

// SetExpiry sets object expiry
func (u *ResetPasswordTokenSecretsV3) SetExpiry(t time.Time) {
	u.Metadata.SetExpiry(t)
}

// GetMetadata returns object metadata
func (u *ResetPasswordTokenSecretsV3) GetMetadata() Metadata {
	return u.Metadata
}

// GetVersion returns resource version
func (u *ResetPasswordTokenSecretsV3) GetVersion() string {
	return u.Version
}

// GetKind returns resource kind
func (u *ResetPasswordTokenSecretsV3) GetKind() string {
	return u.Kind
}

// SetName sets the name of the resource
func (u *ResetPasswordTokenSecretsV3) SetName(name string) {
	u.Metadata.Name = name
}

// GetResourceID returns resource ID
func (u *ResetPasswordTokenSecretsV3) GetResourceID() int64 {
	return u.Metadata.ID
}

// SetResourceID sets resource ID
func (u *ResetPasswordTokenSecretsV3) SetResourceID(id int64) {
	u.Metadata.ID = id
}

// GetSubKind returns resource sub kind
func (u *ResetPasswordTokenSecretsV3) GetSubKind() string {
	return u.SubKind
}

// SetSubKind sets resource subkind
func (u *ResetPasswordTokenSecretsV3) SetSubKind(s string) {
	u.SubKind = s
}

// setStaticFields sets static resource header and metadata fields.
func (u *ResetPasswordTokenSecretsV3) setStaticFields() {
	u.Kind = KindResetPasswordTokenSecrets
	u.Version = V3
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (u ResetPasswordTokenSecretsV3) CheckAndSetDefaults() error {
	u.setStaticFields()
	if err := u.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// // String represents a human readable version of the token secrets
func (u *ResetPasswordTokenSecretsV3) String() string {
	return fmt.Sprintf("ResetPasswordTokenSecretsV3(tokenID=%v, opt_key=%v, qr_code=%v)", u.GetName(), u.Spec.OTPKey, u.Spec.QRCode)
}
