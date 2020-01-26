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

package services

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// UserTokenSecrets contains secrets of user token
type UserTokenSecrets interface {
	// Resource provides common resource properties
	Resource
	// GetCreated returns Created
	GetCreated() time.Time
	// GetQRCode returns QRCode
	GetQRCode() []byte
	// GetOTPKey returns OTP key
	GetOTPKey() string
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
}

// GetName returns Name
func (u *UserTokenSecretsV3) GetName() string {
	return u.Metadata.Name
}

// GetCreated returns Created
func (u *UserTokenSecretsV3) GetCreated() time.Time {
	return u.Spec.Created
}

// GetOTPKey returns OTP Key
func (u *UserTokenSecretsV3) GetOTPKey() string {
	return u.Spec.OTPKey
}

// GetQRCode returns QRCode
func (u *UserTokenSecretsV3) GetQRCode() []byte {
	return []byte(u.Spec.QRCode)
}

// Expiry returns object expiry setting
func (u *UserTokenSecretsV3) Expiry() time.Time {
	return u.Metadata.Expiry()
}

// SetExpiry sets object expiry
func (u *UserTokenSecretsV3) SetExpiry(t time.Time) {
	u.Metadata.SetExpiry(t)
}

// SetTTL sets Expires header using current clock
func (u *UserTokenSecretsV3) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	u.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (u *UserTokenSecretsV3) GetMetadata() Metadata {
	return u.Metadata
}

// GetVersion returns resource version
func (u *UserTokenSecretsV3) GetVersion() string {
	return u.Version
}

// GetKind returns resource kind
func (u *UserTokenSecretsV3) GetKind() string {
	return u.Kind
}

// SetName sets the name of the resource
func (u *UserTokenSecretsV3) SetName(name string) {
	u.Metadata.Name = name
}

// GetResourceID returns resource ID
func (u *UserTokenSecretsV3) GetResourceID() int64 {
	return u.Metadata.ID
}

// SetResourceID sets resource ID
func (u *UserTokenSecretsV3) SetResourceID(id int64) {
	u.Metadata.ID = id
}

// GetSubKind returns resource sub kind
func (u *UserTokenSecretsV3) GetSubKind() string {
	return u.SubKind
}

// SetSubKind sets resource subkind
func (u *UserTokenSecretsV3) SetSubKind(s string) {
	u.SubKind = s
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (u UserTokenSecretsV3) CheckAndSetDefaults() error {
	return u.Metadata.CheckAndSetDefaults()
}

// // String represents a human readable version of the user token secrets
func (u *UserTokenSecretsV3) String() string {
	return fmt.Sprintf("UserTokenSecretsV3(tokenID=%v, opt_key=%v, qr_code=%v)", u.GetName(), u.Spec.OTPKey, u.Spec.QRCode)
}

// NewUserTokenSecrets creates an instance of UserTokenSecrets
func NewUserTokenSecrets(tokenID string) (UserTokenSecretsV3, error) {
	secrets := UserTokenSecretsV3{
		Kind:    KindUserTokenSecrets,
		Version: V3,
		Metadata: Metadata{
			Name: tokenID,
		},
	}

	err := secrets.CheckAndSetDefaults()
	if err != nil {
		return UserTokenSecretsV3{}, trace.Wrap(err)
	}

	return secrets, nil
}

// UserTokenSecretsSpecV3Template is a template for V3 UserTokenSecrets JSON schema
const UserTokenSecretsSpecV3Template = `{
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

// UnmarshalUserTokenSecrets unmarshals UserTokenSecrets
func UnmarshalUserTokenSecrets(bytes []byte) (UserTokenSecrets, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	schema := fmt.Sprintf(V2SchemaTemplate, MetadataSchema, UserTokenSecretsSpecV3Template, DefaultDefinitions)

	var usertokenSecrets UserTokenSecretsV3
	err := utils.UnmarshalWithSchema(schema, &usertokenSecrets, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	return &usertokenSecrets, nil
}

// MarshalUserTokenSecrets marshals role to JSON or YAML.
func MarshalUserTokenSecrets(usertokenSecrets UserTokenSecrets, opts ...MarshalOption) ([]byte, error) {
	return utils.FastMarshal(usertokenSecrets)
}
