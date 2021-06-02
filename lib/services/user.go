/*
Copyright 2019 Gravitational, Inc.

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
	"encoding/json"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ValidateUser validates the User and sets default values
func ValidateUser(u User) error {
	if err := u.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if localAuth := u.GetLocalAuth(); localAuth != nil {
		if err := ValidateLocalAuthSecrets(localAuth); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// UsersEquals checks if the users are equal
func UsersEquals(u User, other User) bool {
	return cmp.Equal(u, other,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		cmpopts.SortSlices(func(a, b *types.MFADevice) bool {
			return a.Metadata.Name < b.Metadata.Name
		}),
	)
}

// LoginAttempt represents successful or unsuccessful attempt for user to login
type LoginAttempt struct {
	// Time is time of the attempt
	Time time.Time `json:"time"`
	// Success indicates whether attempt was successful
	Success bool `json:"bool"`
}

// Check checks parameters
func (la *LoginAttempt) Check() error {
	if la.Time.IsZero() {
		return trace.BadParameter("missing parameter time")
	}
	return nil
}

// UnmarshalUser unmarshals the User resource from JSON.
func UnmarshalUser(bytes []byte, opts ...MarshalOption) (User, error) {
	var h ResourceHeader
	err := json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch h.Version {
	case V2:
		var u UserV2
		if err := utils.FastUnmarshal(bytes, &u); err != nil {
			return nil, trace.BadParameter(err.Error())
		}

		if err := ValidateUser(&u); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			u.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			u.SetExpiry(cfg.Expires)
		}

		return &u, nil
	}
	return nil, trace.BadParameter("user resource version %v is not supported", h.Version)
}

// MarshalUser marshals the User resource to JSON.
func MarshalUser(user User, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch user := user.(type) {
	case *UserV2:
		if version := user.GetVersion(); version != V2 {
			return nil, trace.BadParameter("mismatched user version %v and type %T", version, user)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *user
			copy.SetResourceID(0)
			user = &copy
		}
		return utils.FastMarshal(user)
	default:
		return nil, trace.BadParameter("unrecognized user version %T", user)
	}
}
