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
	"log/slog"
	"slices"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

// AccessListsAndLockGetter is an interface for retrieving access lists and locks.
type AccessListsAndLockGetter interface {
	services.AccessListsGetter
	services.LockGetter
}

// GeneratorConfig is the configuration for the user login state generator.
type GeneratorConfig struct {
	// Log is a logger to use for the generator.
	Log *slog.Logger

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
	log         *slog.Logger
	accessLists AccessListsAndLockGetter
	access      services.Access
	usageEvents UsageEventsClient
	clock       clockwork.Clock
	emitter     apievents.Emitter
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
		emitter:     config.Emitter,
	}, nil
}

// Generate will generate the user login state for the given user.
func (g *Generator) Generate(ctx context.Context, user types.User, ulsService services.UserLoginStates) (*userloginstate.UserLoginState, error) {
	return g.generate(ctx, user, ulsService, false)
}

// GeneratePureULS is a variant of user login state generation that emits no usage events and ignores any existing user login state
// in the backend. Used for auditing/introspection purposes.
func (g *Generator) GeneratePureULS(ctx context.Context, user types.User) (*userloginstate.UserLoginState, error) {
	return g.generate(ctx, user, nil, true)
}

// generate is the underlying implementation for Generate and GeneratePure.
func (g *Generator) generate(ctx context.Context, user types.User, ulsService services.UserLoginStates, pure bool) (*userloginstate.UserLoginState, error) {
	var originalTraits map[string][]string
	var traits map[string][]string
	var githubIdentity *userloginstate.ExternalIdentity
	if len(user.GetTraits()) > 0 {
		originalTraits = make(map[string][]string, len(user.GetTraits()))
		traits = make(map[string][]string, len(user.GetTraits()))
		for k, v := range user.GetTraits() {
			originalTraits[k] = slices.Clone(v)
			traits[k] = slices.Clone(v)
		}
	}

	// Only expecting one for now.
	if githubIdentities := user.GetGithubIdentities(); len(githubIdentities) > 0 {
		githubIdentity = &userloginstate.ExternalIdentity{
			UserID:   githubIdentities[0].UserID,
			Username: githubIdentities[0].Username,
		}
	}

	// Create a new empty user login state.
	uls, err := userloginstate.New(
		header.Metadata{
			Name:   user.GetName(),
			Labels: user.GetAllLabels(),
		}, userloginstate.Spec{
			OriginalRoles:  slices.Clone(user.GetRoles()),
			OriginalTraits: originalTraits,
			Roles:          slices.Clone(user.GetRoles()),
			Traits:         traits,
			UserType:       user.GetUserType(),
			GitHubIdentity: githubIdentity,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate the user login state.
	inheritedRoles, inheritedTraits, err := g.addAccessListsToState(ctx, user, uls)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !pure {
		// Preserve states like GitHub identities across logins.
		if err := UpdatePreservedAttributes(ctx, uls, ulsService); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Clean up the user login state after generating it.
	if err := g.postProcess(ctx, uls); err != nil {
		return nil, trace.Wrap(err)
	}

	if g.usageEvents != nil && !pure {
		// Emit the usage event metadata.
		if err := g.emitUsageEvent(ctx, user, uls, inheritedRoles, inheritedTraits); err != nil {
			g.log.DebugContext(ctx, "Error emitting usage event during user login state generation, skipping", "error", err)
		}
	}

	return uls, nil
}

// addAccessListsToState will add the user's applicable access lists to the user login state after validating them, returning any inherited roles and traits.
func (g *Generator) addAccessListsToState(ctx context.Context, user types.User, state *userloginstate.UserLoginState) (inheritedRoles []string, inheritedTraits map[string][]string, err error) {
	locks, err := g.accessLists.GetLocks(ctx, true, types.LockTarget{
		User: user.GetName(),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if len(locks) > 0 {
		return inheritedRoles, inheritedTraits, nil
	}

	var allInheritedRoles []string
	allInheritedTraits := make(map[string][]string)
	applyHierarchy := func(hierarchy []*accesslist.AccessList, accessListName string, ownerHierarchy bool) error {
		if len(hierarchy) == 0 {
			return nil
		}
		inheritedRoles, inheritedTraits, err := g.applyGrantsAcrossACLs(ctx, hierarchy, accessListName, user, state, ownerHierarchy)
		if err != nil {
			return trace.Wrap(err)
		}
		allInheritedRoles = append(allInheritedRoles, inheritedRoles...)
		for k, values := range inheritedTraits {
			allInheritedTraits[k] = append(allInheritedTraits[k], values...)
		}
		return nil
	}

	h, err := accesslists.NewHierarchy(accesslists.HierarchyConfig{
		AccessListsService: g.accessLists,
		Clock:              g.clock,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	for acl, err := range clientutils.Resources(ctx, g.accessLists.ListAccessLists) {
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		memberOf, ownerOf, err := h.GetHierarchyForUser(ctx, acl, user)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		// Grants are inherited if the user is a member of the access list, explicitly or via inheritance.
		if err := applyHierarchy(memberOf, acl.GetName(), false); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		// OwnerGrants are inherited if the user is an owner of the access list, explicitly or via inheritance.
		if err := applyHierarchy(ownerOf, acl.GetName(), true); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}
	return allInheritedRoles, allInheritedTraits, nil
}

func (g *Generator) applyGrantsAcrossACLs(ctx context.Context, acls []*accesslist.AccessList, baseName string, user types.User, state *userloginstate.UserLoginState, owner bool) (inheritedRoles []string, inheritedTraits map[string][]string, err error) {
	inheritedTraits = make(map[string][]string)
	for _, acl := range acls {
		grants := selectGrants(acl, owner)
		missing, err := g.identifyMissingRoles(ctx, grants.Roles)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if len(missing) > 0 {
			// If there are any missing roles, then we cannot apply the access list.
			// Emit an audit event and skip the access list.
			// This flow is designed to skip the entire access list rather than processing individual roles within it.
			// This approach ensures that access lists are treated as cohesive units of access control. Partial
			// application of an access list could result in unintended permission configurations, potentially leading
			// to security vulnerabilities or unpredictable behavior.
			g.emitSkippedAccessListEvent(ctx, acl.Spec.Title, missing, user.GetName())
			continue
		}
		g.grantRolesAndTraits(grants, state)
		if acl.GetName() == baseName {
			continue
		}
		inheritedRoles = append(inheritedRoles, grants.Roles...)
		for k, values := range grants.Traits {
			inheritedTraits[k] = append(inheritedTraits[k], values...)
		}
	}
	return inheritedRoles, inheritedTraits, nil
}

func selectGrants(acl *accesslist.AccessList, owner bool) accesslist.Grants {
	if owner {
		return acl.Spec.OwnerGrants
	}
	return acl.Spec.Grants
}

// grantRolesAndTraits will append the roles and traits from the provided Grants to the UserLoginState,
// returning inherited roles and traits if AccessListUserAssignmentType is inherited.
func (g *Generator) grantRolesAndTraits(grants accesslist.Grants, state *userloginstate.UserLoginState) {
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

	// Make sure all the roles exist. If they don't, error out.
	for _, role := range state.Spec.Roles {
		if _, err := g.access.GetRole(ctx, role); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// emitUsageEvent will emit the usage event for user state generation.
func (g *Generator) emitUsageEvent(ctx context.Context, user types.User, state *userloginstate.UserLoginState, inheritedRoles []string, inheritedTraits map[string][]string) error {
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

	inheritedRoles = utils.Deduplicate(inheritedRoles)
	for k, v := range inheritedTraits {
		inheritedTraits[k] = utils.Deduplicate(v)
	}

	countInheritedRolesGranted := len(inheritedRoles)
	countInheritedTraitsGranted := 0
	for _, values := range inheritedTraits {
		countInheritedTraitsGranted += len(values)
	}

	countRolesGranted := stateRoleCount - staticRoleCount
	countTraitsGranted := stateTraitCount - staticTraitCount

	// No roles or traits were granted or inherited, so skip emitting the event.
	if countRolesGranted+countTraitsGranted+countInheritedRolesGranted+countInheritedTraitsGranted == 0 {
		return nil
	}

	grantsToUser := &usageeventsv1.AccessListGrantsToUser{
		UserName:                    user.GetName(),
		CountRolesGranted:           int32(countRolesGranted),
		CountTraitsGranted:          int32(countTraitsGranted),
		CountInheritedRolesGranted:  int32(countInheritedRolesGranted),
		CountInheritedTraitsGranted: int32(countInheritedTraitsGranted),
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

// UpdatePreservedAttributes retrieves attributes that can be preserved in user
// login state cross logins.
func UpdatePreservedAttributes(ctx context.Context, user services.UserState, ulsService services.UserLoginStates) error {
	// Use the new/existing GitHubIdentities.
	// TODO(greedy52) implement a way to remove the identity or find a way to
	// avoid keeping the identity forever in user login state.
	if len(user.GetGithubIdentities()) > 0 {
		return nil
	}

	// Find the old state if exists.
	old, err := ulsService.GetUserLoginState(ctx, user.GetName())
	if err != nil {
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	if githubIdentities := old.GetGithubIdentities(); len(githubIdentities) > 0 {
		user.SetGithubIdentities(githubIdentities)
	}
	return nil
}

// Refresh will take the user and update the user login state in the backend.
func (g *Generator) Refresh(ctx context.Context, user types.User, ulsService services.UserLoginStates) (*userloginstate.UserLoginState, error) {
	uls, err := g.Generate(ctx, user, ulsService)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set the full state directly on the user object.
	// We cannot rely Upsert/Get User login state because the next webhook that calls GetUserLogin
	// might receive stale data.
	// Instead of using Upsert/Get calls to pass the object through the login hooks chain,
	// the login hook updates the user object directly.
	// TODO(smallinsky): Consider removing the user login state approach entirely, as it currently only mirrors the user state.
	user.SetRoles(uls.GetRoles())
	user.SetTraits(uls.GetTraits())

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
		g.log.WarnContext(ctx, "Failed to emit access list skipped warning audit event", "error", err)
	}
}
