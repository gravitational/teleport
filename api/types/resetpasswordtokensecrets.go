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

	"github.com/gravitational/teleport/lib/utils"

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
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
}

// NewResetPasswordTokenSecrets creates an instance of ResetPasswordTokenSecrets.
func NewResetPasswordTokenSecrets(tokenID string) (ResetPasswordTokenSecrets, error) {
	secrets := ResetPasswordTokenSecretsV3{
		Kind:    KindResetPasswordTokenSecrets,
		Version: V3,
		Metadata: Metadata{
			Name: tokenID,
		},
	}

	err := secrets.CheckAndSetDefaults()
	if err != nil {
		return &ResetPasswordTokenSecretsV3{}, trace.Wrap(err)
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

// SetTTL sets Expires header using current clock
func (u *ResetPasswordTokenSecretsV3) SetTTL(clock Clock, ttl time.Duration) {
	u.Metadata.SetTTL(clock, ttl)
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

// CheckAndSetDefaults checks and set default values for any missing fields.
func (u ResetPasswordTokenSecretsV3) CheckAndSetDefaults() error {
	return u.Metadata.CheckAndSetDefaults()
}

// // String represents a human readable version of the token secrets
func (u *ResetPasswordTokenSecretsV3) String() string {
	return fmt.Sprintf("ResetPasswordTokenSecretsV3(tokenID=%v, opt_key=%v, qr_code=%v)", u.GetName(), u.Spec.OTPKey, u.Spec.QRCode)
}

// ResetPasswordTokenSecretsSpecV3Template is a template for V3 ResetPasswordTokenSecrets JSON schema
const ResetPasswordTokenSecretsSpecV3Template = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "opt_key": {
      "type": ["string"]
    },
    "qr_code": {
      "type": ["string"]
    },
    "created": {
      "type": ["string"]
    }
  }
}`

// UnmarshalResetPasswordTokenSecrets unmarshals ResetPasswordTokenSecrets
func UnmarshalResetPasswordTokenSecrets(bytes []byte) (ResetPasswordTokenSecrets, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	schema := fmt.Sprintf(V2SchemaTemplate, MetadataSchema, ResetPasswordTokenSecretsSpecV3Template, DefaultDefinitions)

	var secrets ResetPasswordTokenSecretsV3
	err := utils.UnmarshalWithSchema(schema, &secrets, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	return &secrets, nil
}

// MarshalResetPasswordTokenSecrets marshals role to JSON or YAML.
func MarshalResetPasswordTokenSecrets(secrets ResetPasswordTokenSecrets, opts ...MarshalOption) ([]byte, error) {
	return utils.FastMarshal(secrets)
}

// ResetPasswordTokenSecretsMarshaler implements marshal/unmarshal of ResetPasswordTokenSecrets implementations
// mostly adds support for extended versions
type ResetPasswordTokenSecretsMarshaler interface {
	// Marshal marshals token to binary representation
	Marshal(t ResetPasswordTokenSecrets, opts ...MarshalOption) ([]byte, error)
	// Unmarshal unmarshals token from binary representation
	Unmarshal(bytes []byte, opts ...MarshalOption) (ResetPasswordTokenSecrets, error)
}

type teleportResetPasswordTokenSecretsMarshaler struct{}

// Unmarshal unmarshals ResetPasswordTokenSecrets
func (t *teleportResetPasswordTokenSecretsMarshaler) Unmarshal(bytes []byte, opts ...MarshalOption) (ResetPasswordTokenSecrets, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var secrets ResetPasswordTokenSecretsV3
	schema := fmt.Sprintf(V2SchemaTemplate, MetadataSchema, ResetPasswordTokenSecretsSpecV3Template, DefaultDefinitions)
	err := utils.UnmarshalWithSchema(schema, &secrets, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	return &secrets, nil
}

// Marshal marshals role to JSON or YAML.
func (t *teleportResetPasswordTokenSecretsMarshaler) Marshal(secrets ResetPasswordTokenSecrets, opts ...MarshalOption) ([]byte, error) {
	return utils.FastMarshal(secrets)
}

var resetPasswordTokenSecretsMarshaler ResetPasswordTokenSecretsMarshaler = &teleportResetPasswordTokenSecretsMarshaler{}

// SetResetTokenSecretsMarshaler sets global ResetPasswordTokenSecrets marshaler
func SetResetTokenSecretsMarshaler(m ResetPasswordTokenSecretsMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	resetPasswordTokenSecretsMarshaler = m
}

// GetResetPasswordTokenSecretsMarshaler returns ResetPasswordTokenSecrets marshaler
func GetResetPasswordTokenSecretsMarshaler() ResetPasswordTokenSecretsMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return resetPasswordTokenSecretsMarshaler
}
