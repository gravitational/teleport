/*
Copyright 2025 Gravitational, Inc.

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

package authinfo

import (
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/authinfo/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

const (
	// authInfoExpire defines expiration time for the auth information resource.
	authInfoExpire = 24 * time.Hour
)

// NewAuthInfo creates a new auth info resource.
func NewAuthInfo(name string, spec *authinfo.AuthInfoSpec) (*authinfo.AuthInfo, error) {
	info := &authinfo.AuthInfo{
		Kind:    types.KindAuthInfo,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    name,
			Expires: timestamppb.New(time.Now().Add(authInfoExpire)),
		},
		Spec: spec,
	}
	if err := ValidateAuthInfo(info); err != nil {
		return nil, trace.Wrap(err)
	}

	return info, nil
}

// ValidateAuthInfo checks that required parameters are set
// for the specified AuthInfo.
func ValidateAuthInfo(c *authinfo.AuthInfo) error {
	if c == nil {
		return trace.BadParameter("AuthInfo is nil")
	}
	if c.Metadata == nil {
		return trace.BadParameter("Metadata is nil")
	}
	if c.Spec == nil {
		return trace.BadParameter("Spec is nil")
	}

	return nil
}
