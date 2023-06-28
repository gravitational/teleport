/*
Copyright 2023 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/trace"
)

// NewHeadlessAuthentication creates a new a headless authentication resource.
func NewHeadlessAuthentication(username, name string, expires time.Time) (*HeadlessAuthentication, error) {
	ha := &HeadlessAuthentication{
		ResourceHeader: ResourceHeader{
			Metadata: Metadata{
				Name:    name,
				Expires: &expires,
			},
		},
		User: username,
	}
	return ha, ha.CheckAndSetDefaults()
}

// CheckAndSetDefaults does basic validation and default setting.
func (h *HeadlessAuthentication) CheckAndSetDefaults() error {
	h.setStaticFields()
	if err := h.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if h.Metadata.Expires == nil || h.Metadata.Expires.IsZero() {
		return trace.BadParameter("headless authentication resource must have non-zero header.metadata.expires")
	}

	if h.User == "" {
		return trace.BadParameter("headless authentication resource must have non-empty user")
	}

	if h.Version == "" {
		h.Version = V1
	}

	return nil
}

// setStaticFields sets static resource header and metadata fields.
func (h *HeadlessAuthentication) setStaticFields() {
	h.Kind = KindHeadlessAuthentication
}

// Stringify returns the readable string for a headless authentication state.
func (h HeadlessAuthenticationState) Stringify() string {
	switch h {
	case HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING:
		return "pending"
	case HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED:
		return "denied"
	case HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED:
		return "approved"
	default:
		return "unknown"
	}
}

// IsUnspecified headless authentication state. This usually means the headless
// authentication resource is a headless authentication stub, with limited data.
func (s HeadlessAuthenticationState) IsUnspecified() bool {
	return s == HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_UNSPECIFIED
}

// IsPending headless authentication state.
func (s HeadlessAuthenticationState) IsPending() bool {
	return s == HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING
}

// headlessStateVariants allows iteration of the expected variants
// of HeadlessAuthenticationState.
var headlessStateVariants = [4]HeadlessAuthenticationState{
	HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_UNSPECIFIED,
	HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING,
	HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED,
	HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED,
}

// Parse attempts to interpret a value as a string representation
// of a HeadlessAuthenticationState.
func (s *HeadlessAuthenticationState) Parse(val string) error {
	for _, state := range headlessStateVariants {
		if state.String() == val {
			*s = state
			return nil
		}
	}
	return trace.BadParameter("unknown request state: %q", val)
}

// HeadlessAuthenticationFilter encodes filter params for headless authentications.
type HeadlessAuthenticationFilter struct {
	Name     string
	Username string
	State    HeadlessAuthenticationState
}

// key values for map encoding of headless authn filter.
const (
	headlessFilterKeyName     = "name"
	headlessFilterKeyUsername = "username"
	headlessFilterKeyState    = "state"
)

// IntoMap copies HeadlessAuthenticationFilter values into a map.
func (f *HeadlessAuthenticationFilter) IntoMap() map[string]string {
	m := make(map[string]string)
	if f.Name != "" {
		m[headlessFilterKeyName] = f.Name
	}
	if f.Username != "" {
		m[headlessFilterKeyUsername] = f.Username
	}
	if !f.State.IsUnspecified() {
		m[headlessFilterKeyState] = f.State.String()
	}
	return m
}

// FromMap copies values from a map into this HeadlessAuthenticationFilter value.
func (f *HeadlessAuthenticationFilter) FromMap(m map[string]string) error {
	for key, val := range m {
		switch key {
		case headlessFilterKeyName:
			f.Name = val
		case headlessFilterKeyUsername:
			f.Username = val
		case headlessFilterKeyState:
			if err := f.State.Parse(val); err != nil {
				return trace.Wrap(err)
			}
		default:
			return trace.BadParameter("unknown filter key %s", key)
		}
	}
	return nil
}

// Match checks if a given headless authentication matches this filter.
func (f *HeadlessAuthenticationFilter) Match(req HeadlessAuthentication) bool {
	if f.Name != "" && req.GetName() != f.Name {
		return false
	}
	if f.Username != "" && req.User != f.Username {
		return false
	}
	if !f.State.IsUnspecified() && req.State != f.State {
		return false
	}
	return true
}
