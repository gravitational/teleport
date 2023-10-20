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

package userloginstate

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// GeneratorConfig is the configuration for the user login state generator.
type GeneratorConfig struct {
	// Log is a logger to use for the generator.
	Log *logrus.Entry

	// AccessLists is a service for retrieving access lists from the backend.
	AccessLists services.AccessListsGetter

	// Access is a service that will be used for retrieving roles from the backend.
	Access services.Access

	// UsageEventsClient is the client for sending usage events metrics.
	UsageEvents UsageEventsClient

	// Clock is the clock to use for the generator.
	Clock clockwork.Clock
}

// UsageEventsClient is an interface that allows for submitting usage events to Posthog.
type UsageEventsClient interface {
	// SubmitUsageEvent submits an external usage event.
	SubmitUsageEvent(ctx context.Context, req *proto.SubmitUsageEventRequest) error
}

func (g *GeneratorConfig) CheckAndSetDefaults() error {
	if g.Log == nil {
		return trace.BadParameter("missing log")
	}

	if g.AccessLists == nil {
		return trace.BadParameter("missing access lists")
	}

	if g.Access == nil {
		return trace.BadParameter("missing access")
	}

	if modules.GetModules().Features().Cloud {
		if g.UsageEvents == nil {
			return trace.BadParameter("missing usage events")
		}
	} else {
		g.UsageEvents = nil
	}

	if g.Clock == nil {
		g.Clock = clockwork.NewRealClock()
	}

	return nil
}

// Generator will generate a user login state from a user.
type Generator struct {
	log         *logrus.Entry
	accessLists services.AccessListsGetter
	access      services.Access
	usageEvents UsageEventsClient
	clock       clockwork.Clock
}

// NewGenerator creates a new user login state generator.
func NewGenerator(config GeneratorConfig) (*Generator, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Generator{
		log:         config.Log,
		accessLists: config.AccessLists,
		access:      config.Access,
		usageEvents: config.UsageEvents,
		clock:       config.Clock,
	}, nil
}

// Generate will generate the user login state for the given user.
func (g *Generator) Generate(ctx context.Context, user types.User) (*userloginstate.UserLoginState, error) {
	var traits map[string][]string
	if len(user.GetTraits()) > 0 {
		traits = make(map[string][]string, len(user.GetTraits()))
		for k, v := range user.GetTraits() {
			traits[k] = utils.CopyStrings(v)
		}
	}
	// Create a new empty user login state.
	uls, err := userloginstate.New(
		header.Metadata{
			Name: user.GetName(),
		}, userloginstate.Spec{
			Roles:    utils.CopyStrings(user.GetRoles()),
			Traits:   traits,
			UserType: user.GetUserType(),
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate the user login state.
	if err := g.addAccessListsToState(ctx, user, uls); err != nil {
		return nil, trace.Wrap(err)
	}

	// Clean up the user login state after generating it.
	if err := g.postProcess(ctx, uls); err != nil {
		return nil, trace.Wrap(err)
	}

	if g.usageEvents != nil {
		// Emit the usage event metadata.
		if err := g.emitUsageEvent(ctx, user, uls); err != nil {
			g.log.Debug("Error emitting usage event during user login state generation, skipping")
		}
	}

	return uls, nil
}

// addAccessListsToState will added the user's applicable access lists to the user login state.
func (g *Generator) addAccessListsToState(ctx context.Context, user types.User, state *userloginstate.UserLoginState) error {
	accessLists, err := g.accessLists.GetAccessLists(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Create an identity for testing membership to access lists.
	identity := tlsca.Identity{
		Username: user.GetName(),
		Groups:   user.GetRoles(),
		Traits:   user.GetTraits(),
		UserType: user.GetUserType(),
	}

	for _, accessList := range accessLists {
		// Check that the user meets the access list requirements.
		if err := services.IsAccessListMember(ctx, identity, g.clock, accessList, g.accessLists); err != nil {
			continue
		}

		state.Spec.Roles = append(state.Spec.Roles, accessList.Spec.Grants.Roles...)

		if state.Spec.Traits == nil && len(accessList.Spec.Grants.Traits) > 0 {
			state.Spec.Traits = map[string][]string{}
		}

		for k, values := range accessList.Spec.Grants.Traits {
			state.Spec.Traits[k] = append(state.Spec.Traits[k], values...)
		}
	}

	return nil
}

// postProcess will perform cleanup to the user login state after its generation.
func (g *Generator) postProcess(ctx context.Context, state *userloginstate.UserLoginState) error {
	// Deduplicate roles and traits
	state.Spec.Roles = utils.Deduplicate(state.Spec.Roles)

	for k, v := range state.Spec.Traits {
		state.Spec.Traits[k] = utils.Deduplicate(v)
	}

	// If there are no roles, don't bother filtering out non-existent roles
	if len(state.Spec.Roles) == 0 {
		return nil
	}

	// Remove roles that don't exist in the backend so that we don't generate certs for non-existent roles.
	// Doing so can prevent login from working properly. This could occur if access lists refer to roles that
	// no longer exist, for example.
	roles, err := g.access.GetRoles(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	roleLookup := map[string]bool{}
	for _, role := range roles {
		roleLookup[role.GetName()] = true
	}

	existingRoles := []string{}
	for _, role := range state.Spec.Roles {
		if roleLookup[role] {
			existingRoles = append(existingRoles, role)
		} else {
			g.log.Warnf("Role %s does not exist when trying to add user login state, will be skipped", role)
		}
	}
	state.Spec.Roles = existingRoles

	return nil
}

// emitUsageEvent will emit the usage event for user state generation.
func (g *Generator) emitUsageEvent(ctx context.Context, user types.User, state *userloginstate.UserLoginState) error {
	staticRoleCount := len(user.GetRoles())
	staticTraitCount := 0
	for _, values := range user.GetTraits() {
		staticTraitCount += len(values)
	}

	stateRoleCount := len(state.GetRoles())
	stateTraitCount := 0
	for _, values := range state.GetTraits() {
		stateTraitCount += len(values)
	}

	countRolesGranted := stateRoleCount - staticRoleCount
	countTraitsGranted := stateTraitCount - staticTraitCount

	// No roles or traits were granted, so skip emitting the event.
	if countRolesGranted == 0 && countTraitsGranted == 0 {
		return nil
	}

	grantsToUser := &usageeventsv1.AccessListGrantsToUser{
		CountRolesGranted:  int32(countRolesGranted),
		CountTraitsGranted: int32(countTraitsGranted),
	}

	if err := g.usageEvents.SubmitUsageEvent(ctx, &proto.SubmitUsageEventRequest{
		Event: &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AccessListGrantsToUser{
				AccessListGrantsToUser: grantsToUser,
			},
		},
	}); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// LoginHook creates a login hook from the Generator and the user login state service.
func (g *Generator) LoginHook(ulsService services.UserLoginStates) func(context.Context, types.User) error {
	return func(ctx context.Context, user types.User) error {
		uls, err := g.Generate(ctx, user)
		if err != nil {
			return trace.Wrap(err)
		}

		_, err = ulsService.UpsertUserLoginState(ctx, uls)
		return trace.Wrap(err)
	}
}
