// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"github.com/gravitational/trace"

	userprovisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
)

// FromProto converts a v1 static host user into an internal static host user.
func FromProto(msg *userprovisioningv1.StaticHostUser) (*userprovisioning.StaticHostUser, error) {
	if msg == nil {
		return nil, trace.BadParameter("static host user message is missing")
	}
	if msg.Spec == nil {
		return nil, trace.BadParameter("spec is missing")
	}
	if msg.Spec.Login == "" {
		return nil, trace.BadParameter("login is missing")
	}

	labels := make(types.Labels)
	if msgLabels := msg.Spec.NodeLabels; msgLabels != nil {
		for k, v := range msgLabels.Values {
			labels[k] = v.Values
		}
	}

	u := userprovisioning.NewStaticHostUser(msg.GetMetadata(), userprovisioning.Spec{
		Login:                msg.Spec.Login,
		Groups:               msg.Spec.Groups,
		Sudoers:              msg.Spec.Sudoers,
		Uid:                  msg.Spec.Uid,
		Gid:                  msg.Spec.Gid,
		NodeLabels:           labels,
		NodeLabelsExpression: msg.Spec.NodeLabelsExpression,
	})
	return u, nil
}

// ToProto converts an internal static host user into a v1 static host user.
func ToProto(hostUser *userprovisioning.StaticHostUser) *userprovisioningv1.StaticHostUser {
	u := &userprovisioningv1.StaticHostUser{
		Kind:     hostUser.GetKind(),
		SubKind:  hostUser.GetSubKind(),
		Version:  hostUser.GetVersion(),
		Metadata: hostUser.GetMetadata(),
		Spec: &userprovisioningv1.StaticHostUserSpec{
			Login:                hostUser.Spec.Login,
			Groups:               hostUser.Spec.Groups,
			Sudoers:              hostUser.Spec.Sudoers,
			Uid:                  hostUser.Spec.Uid,
			Gid:                  hostUser.Spec.Gid,
			NodeLabels:           hostUser.Spec.NodeLabels.ToProto(),
			NodeLabelsExpression: hostUser.Spec.NodeLabelsExpression,
		},
	}
	return u
}
