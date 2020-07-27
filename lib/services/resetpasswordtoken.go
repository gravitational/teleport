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

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// ResetPasswordToken represents a temporary token used to reset passwords
type ResetPasswordToken interface {
	// Resource provides common resource properties
	Resource
	// GetUser returns User
	GetUser() string
	// GetCreated returns Created
	GetCreated() time.Time
	// GetURL returns URL
	GetURL() string
	// SetURL returns URL
	SetURL(string)
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
}

// GetName returns Name
func (u *ResetPasswordTokenV3) GetName() string {
	return u.Metadata.Name
}

// GetUser returns User
func (u *ResetPasswordTokenV3) GetUser() string {
	return u.Spec.User
}

// GetCreated returns Created
func (u *ResetPasswordTokenV3) GetCreated() time.Time {
	return u.Spec.Created
}

// GetURL returns URL
func (u *ResetPasswordTokenV3) GetURL() string {
	return u.Spec.URL
}

// SetURL sets URL
func (u *ResetPasswordTokenV3) SetURL(url string) {
	u.Spec.URL = url
}

// Expiry returns object expiry setting
func (u *ResetPasswordTokenV3) Expiry() time.Time {
	return u.Metadata.Expiry()
}

// SetExpiry sets object expiry
func (u *ResetPasswordTokenV3) SetExpiry(t time.Time) {
	u.Metadata.SetExpiry(t)
}

// SetTTL sets Expires header using current clock
func (u *ResetPasswordTokenV3) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	u.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (u *ResetPasswordTokenV3) GetMetadata() Metadata {
	return u.Metadata
}

// GetVersion returns resource version
func (u *ResetPasswordTokenV3) GetVersion() string {
	return u.Version
}

// GetKind returns resource kind
func (u *ResetPasswordTokenV3) GetKind() string {
	return u.Kind
}

// SetName sets the name of the resource
func (u *ResetPasswordTokenV3) SetName(name string) {
	u.Metadata.Name = name
}

// GetResourceID returns resource ID
func (u *ResetPasswordTokenV3) GetResourceID() int64 {
	return u.Metadata.ID
}

// SetResourceID sets resource ID
func (u *ResetPasswordTokenV3) SetResourceID(id int64) {
	u.Metadata.ID = id
}

// GetSubKind returns resource sub kind
func (u *ResetPasswordTokenV3) GetSubKind() string {
	return u.SubKind
}

// SetSubKind sets resource subkind
func (u *ResetPasswordTokenV3) SetSubKind(s string) {
	u.SubKind = s
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (u ResetPasswordTokenV3) CheckAndSetDefaults() error {
	return u.Metadata.CheckAndSetDefaults()
}

// // String represents a human readable version of the token
func (u *ResetPasswordTokenV3) String() string {
	return fmt.Sprintf("ResetPasswordTokenV3(tokenID=%v, user=%v, expires at %v)", u.GetName(), u.Spec.User, u.Expiry())
}

// NewResetPasswordToken is a convenience wa to create a RemoteCluster resource.
func NewResetPasswordToken(tokenID string) ResetPasswordTokenV3 {
	return ResetPasswordTokenV3{
		Kind:    KindResetPasswordToken,
		Version: V3,
		Metadata: Metadata{
			Name:      tokenID,
			Namespace: defaults.Namespace,
		},
	}
}

// ResetPasswordTokenSpecV3Template is a template for V3 ResetPasswordToken JSON schema
const ResetPasswordTokenSpecV3Template = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
		"user": {
			"type": ["string"]
		},
		"created": {
			"type": ["string"]
		},
		"url": {
			"type": ["string"]
		}
  }
}`

type TeleportResetPasswordTokenMarshaler struct{}

// ResetPasswordTokenMarshaler implements marshal/unmarshal of ResetPasswordToken implementations
// mostly adds support for extended versions
type ResetPasswordTokenMarshaler interface {
	// Marshal marshals token to binary representation
	Marshal(t ResetPasswordToken, opts ...MarshalOption) ([]byte, error)
	// Unmarshal unmarshals token from binary representation
	Unmarshal(bytes []byte, opts ...MarshalOption) (ResetPasswordToken, error)
}

var resetPasswordTokenMarshaler ResetPasswordTokenMarshaler = &TeleportResetPasswordTokenMarshaler{}

// SetResetTokenMarshaler sets global ResetPasswordToken marshaler
func SetResetTokenMarshaler(m ResetPasswordTokenMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	resetPasswordTokenMarshaler = m
}

// GetResetPasswordTokenMarshaler returns ResetPasswordToken marshaler
func GetResetPasswordTokenMarshaler() ResetPasswordTokenMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return resetPasswordTokenMarshaler
}

// Unmarshal unmarshals ResetPasswordToken
func (t *TeleportResetPasswordTokenMarshaler) Unmarshal(bytes []byte, opts ...MarshalOption) (ResetPasswordToken, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var token ResetPasswordTokenV3
	schema := fmt.Sprintf(V2SchemaTemplate, MetadataSchema, ResetPasswordTokenSpecV3Template, DefaultDefinitions)
	err := utils.UnmarshalWithSchema(schema, &token, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	return &token, nil
}

// Marshal marshals role to JSON or YAML.
func (t *TeleportResetPasswordTokenMarshaler) Marshal(token ResetPasswordToken, opts ...MarshalOption) ([]byte, error) {
	return utils.FastMarshal(token)
}
