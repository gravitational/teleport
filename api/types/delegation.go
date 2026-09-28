// Copyright 2026 Gravitational, Inc.
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

package types

import (
	"google.golang.org/protobuf/encoding/protojson"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
)

// DelegationFromLegacy converts a delegation from its legacy form to modern
// protobuf.
func DelegationFromLegacy(legacy *Delegation) *delegationv1.Delegation {
	if legacy == nil {
		return nil
	}

	chain := make([]*delegationv1.DelegationChainLink, len(legacy.GetChain()))
	for idx, link := range legacy.GetChain() {
		chain[idx] = delegationChainLinkFromLegacy(link)
	}

	return delegationv1.Delegation_builder{
		Chain: chain,
	}.Build()
}

func delegationChainLinkFromLegacy(legacy *DelegationChainLink) *delegationv1.DelegationChainLink {
	if legacy == nil {
		return nil
	}

	modern := delegationv1.DelegationChainLink_builder{}
	switch {
	case legacy.GetUser() != nil:
		modern.User = userDelegatorFromLegacy(legacy.GetUser())
	case legacy.GetBot() != nil:
		modern.Bot = botDelegatorFromLegacy(legacy.GetBot())
	}
	return modern.Build()
}

func userDelegatorFromLegacy(legacy *UserDelegator) *delegationv1.UserDelegator {
	if legacy == nil {
		return nil
	}

	return delegationv1.UserDelegator_builder{
		Name: legacy.GetName(),
	}.Build()
}

func botDelegatorFromLegacy(legacy *BotDelegator) *delegationv1.BotDelegator {
	if legacy == nil {
		return nil
	}

	modern := delegationv1.BotDelegator_builder{
		Name: legacy.GetName(),
		Beam: beamInfoFromLegacy(legacy.GetBeam()),
	}
	if legacy.GetScope() != "" {
		scope := legacy.GetScope()
		modern.Scope = &scope
	}
	return modern.Build()
}

func beamInfoFromLegacy(legacy *BeamInfo) *delegationv1.BeamInfo {
	if legacy == nil {
		return nil
	}

	return delegationv1.BeamInfo_builder{
		Id:    legacy.GetId(),
		Alias: legacy.GetAlias(),
	}.Build()
}

// DelegationToLegacy converts a delegation from its modern protobuf form to
// legacy for storage.
func DelegationToLegacy(modern *delegationv1.Delegation) *Delegation {
	if modern == nil {
		return nil
	}

	chain := make([]*DelegationChainLink, len(modern.GetChain()))
	for idx, link := range modern.GetChain() {
		chain[idx] = delegationChainLinkToLegacy(link)
	}

	return &Delegation{
		Chain: chain,
	}
}

func delegationChainLinkToLegacy(modern *delegationv1.DelegationChainLink) *DelegationChainLink {
	if modern == nil {
		return nil
	}

	legacy := &DelegationChainLink{}
	switch modern.WhichIdentity() {
	case delegationv1.DelegationChainLink_User_case:
		legacy.User = userDelegatorToLegacy(modern.GetUser())
	case delegationv1.DelegationChainLink_Bot_case:
		legacy.Bot = botDelegatorToLegacy(modern.GetBot())
	}
	return legacy
}

func userDelegatorToLegacy(modern *delegationv1.UserDelegator) *UserDelegator {
	if modern == nil {
		return nil
	}

	return &UserDelegator{
		Name: modern.GetName(),
	}
}

func botDelegatorToLegacy(modern *delegationv1.BotDelegator) *BotDelegator {
	if modern == nil {
		return nil
	}

	legacy := &BotDelegator{
		Name: modern.GetName(),
		Beam: beamInfoToLegacy(modern.GetBeam()),
	}
	if modern.HasScope() {
		legacy.Scope = modern.GetScope()
	}
	return legacy
}

func beamInfoToLegacy(modern *delegationv1.BeamInfo) *BeamInfo {
	if modern == nil {
		return nil
	}

	return &BeamInfo{
		Id:    modern.GetId(),
		Alias: modern.GetAlias(),
	}
}

// MarshalJSON implements json.Marshaler by using the modern protobuf message's
// JSON representation to ensure oneofs are represented correctly.
func (m *Delegation) MarshalJSON() ([]byte, error) {
	modern := DelegationFromLegacy(m)
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal(modern)
}

// Unmarshal implements json.Marshaler by using the modern protobuf message's
// JSON representation to ensure oneofs are represented correctly.
func (m *Delegation) UnmarshalJSON(data []byte) error {
	var modern delegationv1.Delegation
	if err := (protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}).Unmarshal(data, &modern); err != nil {
		return err
	}
	if l := DelegationToLegacy(&modern); l != nil {
		*m = *l
	}
	return nil
}
