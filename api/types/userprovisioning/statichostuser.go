/*
Copyright 2024 Gravitational, Inc.

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

package userprovisioning

import (
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
)

// NewStaticHostUser creates a new host user to be applied to matching SSH nodes.
func NewStaticHostUser(name string, spec *userprovisioningpb.StaticHostUserSpec) *userprovisioningpb.StaticHostUser {
	return &userprovisioningpb.StaticHostUser{
		Kind:    types.KindStaticHostUser,
		Version: types.V2,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: spec,
	}
}
