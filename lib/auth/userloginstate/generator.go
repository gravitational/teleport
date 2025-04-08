/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package userloginstate

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// AccessListsAndLockGetter is an interface for retrieving access lists and locks.
type AccessListsAndLockGetter interface {
	services.AccessListsGetter
	services.LockGetter
}

// GeneratorConfig is the configuration for the user login state generator.
type GeneratorConfig struct {
	// Log is a logger to use for the generator.
	Log *logrus.Entry

	// AccessLists is a service for retrieving access lists and locks from the backend.
	AccessLists AccessListsAndLockGetter

	// Access is a service that will be used for retrieving roles from the backend.
	Access services.Access

	// UsageEventsClient is the client for sending usage events metrics.
	UsageEvents UsageEventsClient

	// Clock is the clock to use for the generator.
	Clock clockwork.Clock

	// Emitter is the emitter for audit events.
	Emitter apievents.Emitter
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

	if g.Emitter == nil {
		return trace.BadParameter("missing audit event emitter")
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
	log           *logrus.Entry
	accessLists   AccessListsAndLockGetter
	access        services.Access
	usageEvents   UsageEventsClient
	memberChecker *services.AccessListMembershipChecker
	clock         clockwork.Clock
	emitter       apievents.Emitter
}

// NewGenerator creates a new user login state generator.
func NewGenerator(config GeneratorConfig) (*Generator, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Generator{
		log:           config.Log,
		accessLists:   config.AccessLists,
		access:        config.Access,
		usageEvents:   config.UsageEvents,
		memberChecker: services.NewAccessListMembershipChecker(config.Clock, config.AccessLists, config.Access),
		clock:         config.Clock,
		emitter:       config.Emitter,
	}, nil
}

// Generate will generate the user login state for the given user.
func (g *Generator) Generate(ctx context.Context, user types.User) (*userloginstate.UserLoginState, error) {
	var originalTraits map[string][]string
	var traits map[string][]string
	if len(user.GetTraits()) > 0 {
		originalTraits = make(map[string][]string, len(user.GetTraits()))
		traits = make(map[string][]string, len(user.GetTraits()))
		for k, v := range user.GetTraits() {
			originalTraits[k] = utils.CopyStrings(v)
			traits[k] = utils.CopyStrings(v)
		}
	}

	labels := make(map[string]string, len(user.GetAllLabels()))
	for k, v := range user.GetAllLabels() {
		labels[k] = v
	}
	labels[userloginstate.OriginalRolesAndTraitsSet] = "true"

	// Create a new empty user login state.
	uls, err := userloginstate.New(
		header.Metadata{
			Name:   user.GetName(),
			Labels: labels,
		}, userloginstate.Spec{
			OriginalRoles:  utils.CopyStrings(user.GetRoles()),
			OriginalTraits: originalTraits,
			Roles:          utils.CopyStrings(user.GetRoles()),
			Traits:         traits,
			UserType:       user.GetUserType(),
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
			g.log.WithError(err).Debug("Error emitting usage event during user login state generation, skipping")
		}
	}

	return uls, nil
}

// addAccessListsToState will add the user's applicable access lists to the user login state after validating them.
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
		if err := services.IsAccessListOwner(identity, accessList); err == nil {
			g.handleAccessListOwnership(ctx, identity, accessList, state)
		}

		if err := g.memberChecker.IsAccessListMember(ctx, identity, accessList); err == nil {
			g.handleAccessListMembership(ctx, identity, accessList, state)
		}
	}

	return nil
}

// handleAccessListMembership validates the access list and applies the grants and traits from the access list to the user if they are a member of the access list.
// If the access list is invalid (because it references a non-existent role, for example,
// then it will not be applied.
func (g *Generator) handleAccessListMembership(ctx context.Context, identity tlsca.Identity, accessList *accesslist.AccessList, state *userloginstate.UserLoginState) error {
	// Validate that all the roles in the access list exist.
	missingRoles, err := g.identifyMissingRoles(ctx, accessList.Spec.Grants.Roles)
	if err != nil {
		return trace.Wrap(err)
	}

	// If there are any missing roles, then we cannot apply the access list.
	// Emit an audit event and return early.
	// This flow is designed to skip the entire access list rather than processing individual roles within it.
	// This approach ensures that access lists are treated as cohesive units of access control. Partial
	// application of an access list could result in unintended permission configurations, potentially leading
	// to security vulnerabilities or unpredictable behavior.
	if missingRoles != nil {
		g.emitSkippedAccessListEvent(ctx, accessList.Spec.Title, missingRoles, identity.Username)
		return nil
	}

	g.grantRolesAndTraits(identity, accessList.Spec.Grants, state)

	return nil
}

// handleAccessListOwnership validates the access list and applies the grants and traits from the access list to the user if they are an owner of the access list.
// If the access list is invalid (because it references a non-existent role, for example,
// then it will not be applied.
func (g *Generator) handleAccessListOwnership(ctx context.Context, identity tlsca.Identity, accessList *accesslist.AccessList, state *userloginstate.UserLoginState) error {
	// Validate that all the roles in the access list exist.
	missingRoles, err := g.identifyMissingRoles(ctx, accessList.Spec.OwnerGrants.Roles)
	if err != nil {
		return trace.Wrap(err)
	}

	// If there are any missing roles, then we cannot apply the access list.
	// Emit an audit event and return early.
	// This flow is designed to skip the entire access list rather than processing individual roles within it.
	// This approach ensures that access lists are treated as cohesive units of access control. Partial
	// application of an access list could result in unintended permission configurations, potentially leading
	// to security vulnerabilities or unpredictable behavior.
	if missingRoles != nil {
		g.emitSkippedAccessListEvent(ctx, accessList.Spec.Title, missingRoles, identity.Username)
		return nil
	}

	g.grantRolesAndTraits(identity, accessList.Spec.OwnerGrants, state)

	return nil
}

func (g *Generator) grantRolesAndTraits(identity tlsca.Identity, grants accesslist.Grants, state *userloginstate.UserLoginState) {
	state.Spec.Roles = append(state.Spec.Roles, grants.Roles...)

	if state.Spec.Traits == nil && len(grants.Traits) > 0 {
		state.Spec.Traits = map[string][]string{}
	}

	for k, values := range grants.Traits {
		state.Spec.Traits[k] = append(state.Spec.Traits[k], values...)
	}
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

	// Make sure all the roles exist. If they don't, error out.
	var existingRoles []string
	for _, role := range state.Spec.Roles {
		_, err := g.access.GetRole(ctx, role)
		if err == nil {
			existingRoles = append(existingRoles, role)
		} else {
			return trace.Wrap(err)
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
		UserName:           user.GetName(),
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

// Refresh will take the user and update the user login state in the backend.
func (g *Generator) Refresh(ctx context.Context, user types.User, ulsService services.UserLoginStates) (*userloginstate.UserLoginState, error) {
	uls, err := g.Generate(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uls, err = ulsService.UpsertUserLoginState(ctx, uls)
	return uls, trace.Wrap(err)
}

// LoginHook creates a login hook from the Generator and the user login state service.
func (g *Generator) LoginHook(ulsService services.UserLoginStates) func(context.Context, types.User) error {
	return func(ctx context.Context, user types.User) error {
		_, err := g.Refresh(ctx, user, ulsService)
		return trace.Wrap(err)
	}
}

// identifyMissingRoles is a helper function which identifies any roles from the provided list that don't exist, and returns nil if they all exist.
func (g *Generator) identifyMissingRoles(ctx context.Context, roles []string) ([]string, error) {
	var missingRoles []string

	for _, role := range roles {
		_, err := g.access.GetRole(ctx, role)
		if err != nil {
			if trace.IsNotFound(err) {
				missingRoles = append(missingRoles, role)
				continue
			}
			return nil, trace.Wrap(err)
		}
	}

	if len(missingRoles) > 0 {
		return missingRoles, nil
	}

	return nil, nil
}

// emitSkippedAccessListEvent emits an audit log event to indicate that an invalid
// access list could not be applied during user login.
func (g *Generator) emitSkippedAccessListEvent(ctx context.Context, accessListName string, missingRoles []string, username string) {
	if err := g.emitter.EmitAuditEvent(ctx, &apievents.UserLoginAccessListInvalid{
		Metadata: apievents.Metadata{
			Type: events.UserLoginAccessListInvalidEvent,
			Code: events.UserLoginAccessListInvalidCode,
		},
		AccessListInvalidMetadata: apievents.AccessListInvalidMetadata{
			AccessListName: accessListName,
			User:           username,
			MissingRoles:   missingRoles,
		},
		Status: apievents.Status{
			Success:     false,
			Error:       fmt.Sprintf("roles %v were not found", missingRoles),
			UserMessage: "access list skipped because it references non-existent role(s)",
		},
	}); err != nil {
		g.log.WithError(err).Warn("Failed to emit access list skipped warning audit event.")
	}
}
