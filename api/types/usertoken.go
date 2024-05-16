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

// UserToken represents a temporary token used for various user related actions ie: change password.
type UserToken interface {
	// Resource provides common resource properties
	Resource
	// GetUser returns User
	GetUser() string
	// SetUser sets User
	SetUser(string)
	// GetCreated returns Created
	GetCreated() time.Time
	// SetCreated sets Created
	SetCreated(time.Time)
	// GetURL returns URL
	GetURL() string
	// SetURL returns URL
	SetURL(string)
	// GetUsage returns usage type.
	GetUsage() UserTokenUsage
	// SetUsage sets usage type.
	SetUsage(UserTokenUsage)
}

// NewUserToken creates an instance of UserToken.
func NewUserToken(tokenID string) (UserToken, error) {
	u := &UserTokenV3{
		Metadata: Metadata{
			Name: tokenID,
		},
	}
	if err := u.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

// GetName returns token ID.
func (u *UserTokenV3) GetName() string {
	return u.Metadata.Name
}

// SetName sets the name of the resource
func (u *UserTokenV3) SetName(name string) {
	u.Metadata.Name = name
}

// GetUser returns User
func (u *UserTokenV3) GetUser() string {
	return u.Spec.User
}

// SetUser sets the name of the resource
func (u *UserTokenV3) SetUser(name string) {
	u.Spec.User = name
}

// GetCreated returns Created
func (u *UserTokenV3) GetCreated() time.Time {
	return u.Spec.Created
}

// SetCreated sets the name of the resource
func (u *UserTokenV3) SetCreated(t time.Time) {
	u.Spec.Created = t
}

// GetURL returns URL
func (u *UserTokenV3) GetURL() string {
	return u.Spec.URL
}

// SetURL sets URL
func (u *UserTokenV3) SetURL(url string) {
	u.Spec.URL = url
}

// Expiry returns object expiry setting
func (u *UserTokenV3) Expiry() time.Time {
	return u.Metadata.Expiry()
}

// SetExpiry sets object expiry
func (u *UserTokenV3) SetExpiry(t time.Time) {
	u.Metadata.SetExpiry(t)
}

// GetMetadata returns object metadata
func (u *UserTokenV3) GetMetadata() Metadata {
	return u.Metadata
}

// GetVersion returns resource version
func (u *UserTokenV3) GetVersion() string {
	return u.Version
}

// GetKind returns resource kind
func (u *UserTokenV3) GetKind() string {
	return u.Kind
}

// GetRevision returns the revision
func (u *UserTokenV3) GetRevision() string {
	return u.Metadata.GetRevision()
}

// SetRevision sets the revision
func (u *UserTokenV3) SetRevision(rev string) {
	u.Metadata.SetRevision(rev)
}

// GetSubKind returns resource sub kind
func (u *UserTokenV3) GetSubKind() string {
	return u.SubKind
}

// SetSubKind sets resource subkind
func (u *UserTokenV3) SetSubKind(s string) {
	u.SubKind = s
}

// setStaticFields sets static resource header and metadata fields.
func (u *UserTokenV3) setStaticFields() {
	u.Kind = KindUserToken
	u.Version = V3
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (u *UserTokenV3) CheckAndSetDefaults() error {
	u.setStaticFields()
	if err := u.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// String represents a human readable version of the token
func (u *UserTokenV3) String() string {
	return fmt.Sprintf("UserTokenV3(tokenID=%v, type=%v user=%v, expires at %v)", u.GetName(), u.GetSubKind(), u.Spec.User, u.Expiry())
}

// GetUsage returns a usage type.
func (u *UserTokenV3) GetUsage() UserTokenUsage {
	return u.Spec.Usage
}

// SetUsage sets a usage type.
func (u *UserTokenV3) SetUsage(r UserTokenUsage) {
	u.Spec.Usage = r
}
