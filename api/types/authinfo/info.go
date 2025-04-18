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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/authinfo/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewAuthInfo creates a new auth info resource.
func NewAuthInfo(spec *authinfov1.AuthInfoSpec) (*authinfov1.AuthInfo, error) {
	info := &authinfov1.AuthInfo{
		Kind:    types.KindAuthInfo,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAuthInfo,
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
func ValidateAuthInfo(info *authinfov1.AuthInfo) error {
	switch {
	case info.GetKind() != types.KindAuthInfo:
		return trace.BadParameter("wrong AuthInfo Kind: %s", info.Kind)
	case info.GetMetadata().Name != types.MetaNameAuthInfo:
		return trace.BadParameter("wrong AuthInfo Metadata name: %s", info.GetMetadata().Name)
	case info.GetSubKind() != "":
		return trace.BadParameter("wrong AuthInfo SubKind: %s", info.GetSubKind())
	case info.GetVersion() != types.V1:
		return trace.BadParameter("wrong AuthInfo Version: %s", info.GetVersion())
	default:
		return nil
	}
}
