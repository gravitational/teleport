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

package backendinfo

import (
	"github.com/gravitational/trace"

	backendinfov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/backendinfo/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewBackendInfo creates a new auth info resource.
func NewBackendInfo(spec *backendinfov1.BackendInfoSpec) (*backendinfov1.BackendInfo, error) {
	info := &backendinfov1.BackendInfo{
		Kind:    types.KindBackendInfo,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameBackendInfo,
		},
		Spec: spec,
	}
	if err := ValidateBackendInfo(info); err != nil {
		return nil, trace.Wrap(err)
	}

	return info, nil
}

// ValidateBackendInfo checks that required parameters are set
// for the specified BackendInfo.
func ValidateBackendInfo(info *backendinfov1.BackendInfo) error {
	switch {
	case info.GetKind() != types.KindBackendInfo:
		return trace.BadParameter("wrong BackendInfo Kind: %+q", info.Kind)
	case info.GetMetadata().Name != types.MetaNameBackendInfo:
		return trace.BadParameter("wrong BackendInfo Metadata name: %+q", info.GetMetadata().Name)
	case info.GetSubKind() != "":
		return trace.BadParameter("wrong BackendInfo SubKind: %+q", info.GetSubKind())
	case info.GetVersion() != types.V1:
		return trace.BadParameter("wrong BackendInfo Version: %+q", info.GetVersion())
	default:
		return nil
	}
}
