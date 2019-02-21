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
	"time"
)

// InviteTokenV3 is an invite token spec format V3
type InviteTokenV3 struct {
	// Kind is a resource kind - always resource.
	Kind string `json:"kind"`

	// SubKind is a resource sub kind
	SubKind string `json:"sub_kind,omitempty"`

	// Version is a resource version.
	Version string `json:"version"`

	// Metadata is metadata about the resource.
	Metadata Metadata `json:"metadata"`

	// Spec is a spec of the invite token
	Spec InviteTokenSpecV3 `json:"spec"`
}

// InviteTokenSpecV3 is a spec for invite token
type InviteTokenSpecV3 struct {
	// URL is a helper invite token URL
	URL string `json:"url"`
}

// NewInviteToken returns a new instance of the invite token
func NewInviteToken(token, signupURL string, expires time.Time) *InviteTokenV3 {
	tok := InviteTokenV3{
		Kind:    KindInviteToken,
		Version: V3,
		Metadata: Metadata{
			Name: token,
		},
		Spec: InviteTokenSpecV3{
			URL: signupURL,
		},
	}
	if !expires.IsZero() {
		tok.Metadata.SetExpiry(expires)
	}
	return &tok
}
