package types

import (
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
)

// DelegationFromLegacy converts a delegation from its legacy/gogoproto form to
// modern protobuf.
func DelegationFromLegacy(legacy *Delegation) *delegationv1.Delegation {
	if legacy == nil {
		return nil
	}

	modern := delegationv1.Delegation_builder{
		Previous: DelegationFromLegacy(legacy.Previous),
	}

	if legacy.User != nil {
		modern.User = delegationv1.UserDelegator_builder{
			Username: legacy.User.Username,
		}.Build()
	}

	if legacy.Bot != nil {
		modern.Bot = delegationv1.BotDelegator_builder{
			Name:  legacy.Bot.Name,
			Scope: legacy.Bot.Scope,
		}.Build()
	}

	return modern.Build()
}

// DelegationToLegacy converts a delegation from its modern protobuf form to
// legacy/gogoproto.
func DelegationToLegacy(modern *delegationv1.Delegation) *Delegation {
	if modern == nil {
		return nil
	}

	legacy := &Delegation{
		Previous: DelegationToLegacy(modern.GetPrevious()),
	}

	if v := modern.GetUser(); v != nil {
		legacy.User = &UserDelegator{
			Username: v.GetUsername(),
		}
	}

	if v := modern.GetBot(); v != nil {
		legacy.Bot = &BotDelegator{
			Name:  v.GetName(),
			Scope: v.GetScope(),
		}
	}

	return legacy
}
