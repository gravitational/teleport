/*
Copyright 2022 Gravitational, Inc.

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
	"encoding/json"
	"time"

	"github.com/gravitational/trace"
)

// OIDCClaims is a redefinition of jose.Claims with additional methods, required for serialization to/from protobuf.
// With those we can reference it with an option like so: `(gogoproto.customtype) = "OIDCClaims"`
type OIDCClaims map[string]interface{}

// Size returns size of the object when marshaled
func (a *OIDCClaims) Size() int {
	bytes, err := json.Marshal(a)
	if err != nil {
		return 0
	}
	return len(bytes)
}

// Unmarshal the object from provided buffer.
func (a *OIDCClaims) Unmarshal(bytes []byte) error {
	return trace.Wrap(json.Unmarshal(bytes, a))
}

// MarshalTo marshals the object to sized buffer
func (a *OIDCClaims) MarshalTo(bytes []byte) (int, error) {
	out, err := json.Marshal(a)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	if len(out) > cap(bytes) {
		return 0, trace.BadParameter("capacity too low: %v, need %v", cap(bytes), len(out))
	}

	copy(bytes, out)

	return len(out), nil
}

// OIDCIdentity is a redefinition of oidc.Identity with additional methods, required for serialization to/from protobuf.
// With those we can reference it with an option like so: `(gogoproto.customtype) = "OIDCIdentity"`
type OIDCIdentity struct {
	// ID is populated from "subject" claim.
	ID string
	// Name of user. Empty in current version of library.
	Name string
	// Email is populated from "email" claim.
	Email string
	// ExpiresAt populated from "exp" claim, represents expiry time.
	ExpiresAt time.Time
}

// Size returns size of the object when marshaled
func (a *OIDCIdentity) Size() int {
	bytes, err := json.Marshal(a)
	if err != nil {
		return 0
	}
	return len(bytes)
}

// Unmarshal the object from provided buffer.
func (a *OIDCIdentity) Unmarshal(bytes []byte) error {
	return trace.Wrap(json.Unmarshal(bytes, a))
}

// MarshalTo marshals the object to sized buffer
func (a *OIDCIdentity) MarshalTo(bytes []byte) (int, error) {
	out, err := json.Marshal(a)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	if len(out) > cap(bytes) {
		return 0, trace.BadParameter("capacity too low: %v, need %v", cap(bytes), len(out))
	}

	copy(bytes, out)

	return len(out), nil
}
