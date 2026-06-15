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

package local

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/join/oracle"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// ProvisioningService governs adding new nodes to the cluster
type ProvisioningService struct {
	backend.Backend
}

// NewProvisioningService returns a new instance of provisioning service
func NewProvisioningService(backend backend.Backend) *ProvisioningService {
	return &ProvisioningService{Backend: backend}
}

// UpsertToken adds provisioning tokens for the auth server
func (s *ProvisioningService) UpsertToken(ctx context.Context, p types.ProvisionToken) error {
	if err := validateProvisionToken(p); err != nil {
		return trace.Wrap(err)
	}

	actions, err := s.AppendPutProvisionTokenActions(nil, p, backend.Whatever())
	if err != nil {
		return err
	}

	if _, err := s.AtomicWrite(ctx, actions); err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return trace.AlreadyExists("token could not be created due to name conflict with an existing scoped or unscoped token, please try again with a different name or delete the conflicting token")
		}
		return trace.Wrap(err)
	}

	return nil
}

// AppendPutProvisionTokenActions adds conditional actions to an atomic write to
// create or update a provision token.
func (s *ProvisioningService) AppendPutProvisionTokenActions(
	actions []backend.ConditionalAction,
	p types.ProvisionToken,
	condition backend.Condition,
) ([]backend.ConditionalAction, error) {
	item, err := itemFromProvisionToken(p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append(actions,
		backend.ConditionalAction{
			Key:       backend.NewKey(tokensPrefix, p.GetName()),
			Condition: condition,
			Action:    backend.Put(*item),
		},
		backend.ConditionalAction{
			Key:       backend.NewKey(scopedTokenPrefix, p.GetName()),
			Condition: backend.NotExists(),
			// the second action is a no-op because we only need to
			// execute a single action to create the token,
			// but both conditions must be met
			Action: backend.Nop(),
		},
	), nil
}

// AppendDeleteProvisionTokenActions adds conditional actions to an atomic
// write to delete a provision token.
func (s *ProvisioningService) AppendDeleteProvisionTokenActions(
	actions []backend.ConditionalAction,
	token string,
	condition backend.Condition,
) ([]backend.ConditionalAction, error) {
	return append(actions, backend.ConditionalAction{
		Key:       backend.NewKey(tokensPrefix, token),
		Condition: condition,
		Action:    backend.Delete(),
	}), nil
}

// PatchToken uses the supplied function to attempt to patch a token resource.
// Up to 3 update attempts will be made if the conditional update fails due to
// a revision comparison failure.
func (s *ProvisioningService) PatchToken(
	ctx context.Context,
	tokenName string,
	updateFn func(types.ProvisionToken) (types.ProvisionToken, error),
) (types.ProvisionToken, error) {
	const iterLimit = 3

	for i := 0; i < iterLimit; i++ {
		existing, err := s.GetToken(ctx, tokenName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Note: CloneProvisionToken only supports ProvisionTokenV2.
		clone, err := services.CloneProvisionToken(existing)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		updated, err := updateFn(clone)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		updatedMetadata := updated.GetMetadata()
		existingMetadata := existing.GetMetadata()

		switch {
		case updatedMetadata.GetName() != existingMetadata.GetName():
			return nil, trace.BadParameter("metadata.name: cannot be patched")
		case updatedMetadata.GetRevision() != existingMetadata.GetRevision():
			return nil, trace.BadParameter("metadata.revision: cannot be patched")
		}

		item, err := itemFromProvisionToken(updated)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		lease, err := s.ConditionalUpdate(ctx, *item)
		if trace.IsCompareFailed(err) {
			continue
		} else if err != nil {
			return nil, trace.Wrap(err)
		}

		updated.SetRevision(lease.Revision)
		return updated, nil
	}

	return nil, trace.CompareFailed("failed to update provision token within %v iterations", iterLimit)
}

// CreateToken creates a new token for the auth server
func (s *ProvisioningService) CreateToken(ctx context.Context, p types.ProvisionToken) error {
	if err := validateProvisionToken(p); err != nil {
		return trace.Wrap(err)
	}

	actions, err := s.AppendPutProvisionTokenActions(nil, p, backend.NotExists())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := s.AtomicWrite(ctx, actions); err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return trace.AlreadyExists("token could not be created due to name conflict with an existing scoped or unscoped token, please try again with a different name or delete the conflicting token")
		}
		return trace.Wrap(err)
	}

	return nil
}

// DeleteAllTokens deletes all provisioning tokens
func (s *ProvisioningService) DeleteAllTokens() error {
	startKey := backend.NewKey(tokensPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// GetToken finds and returns token by ID
func (s *ProvisioningService) GetToken(ctx context.Context, token string) (types.ProvisionToken, error) {
	if token == "" {
		return nil, trace.BadParameter("missing parameter token")
	}
	item, err := s.Get(ctx, backend.NewKey(tokensPrefix, token))
	if trace.IsNotFound(err) {
		return nil, trace.NotFound("provisioning token(%s) not found", backend.MaskKeyName(token))
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	return services.UnmarshalProvisionToken(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// DeleteToken deletes a token by ID
func (s *ProvisioningService) DeleteToken(ctx context.Context, token string) error {
	if token == "" {
		return trace.BadParameter("missing parameter token")
	}
	err := s.Delete(ctx, backend.NewKey(tokensPrefix, token))
	if trace.IsNotFound(err) {
		return trace.NotFound("provisioning token(%s) not found", backend.MaskKeyName(token))
	}
	return trace.Wrap(err)
}

// GetTokens returns all active (non-expired) provisioning tokens
// Deprecated: use [ListProvisionTokens] instead.
// TODO(hugoShaka): DELETE IN 19.0.0
func (s *ProvisioningService) GetTokens(ctx context.Context) ([]types.ProvisionToken, error) {
	startKey := backend.ExactKey(tokensPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tokens := make([]types.ProvisionToken, len(result.Items))
	for i, item := range result.Items {
		t, err := services.UnmarshalProvisionToken(
			item.Value,
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling token (key: %q)", item.Key)
		}
		tokens[i] = t
	}
	return tokens, nil
}

// ListProvisionTokens returns a paginated list of provision tokens. Items can
// be filtered by role and bot name. Tokens with ANY of the provided roles are
// returned. If a bot name is provided, only tokens having a role of Bot are
// returned.
func (s *ProvisioningService) ListProvisionTokens(ctx context.Context, pageSize int, pageToken string, anyRoles types.SystemRoles, botName string) ([]types.ProvisionToken, string, error) {
	// Bound page size (0 - 1_000)
	if pageSize <= 0 || pageSize > int(defaults.MaxIterationLimit) {
		pageSize = int(defaults.MaxIterationLimit)
	}

	prefix := backend.NewKey(tokensPrefix)
	var out []types.ProvisionToken
	for item, err := range s.Items(ctx, backend.ItemsParams{
		StartKey: prefix.AppendKey(backend.KeyFromString(pageToken)),
		EndKey:   backend.RangeEnd(prefix.ExactKey()),
	}) {
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		t, err := services.UnmarshalProvisionToken(
			item.Value,
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision),
		)
		if err != nil {
			return nil, "", trace.Wrap(err, "unmarshaling token (key: %q)", item.Key)
		}

		if len(out) == pageSize {
			nextKey := strings.TrimPrefix(
				item.Key.TrimPrefix(backend.ExactKey(tokensPrefix)).String(),
				string(backend.Separator),
			)
			return out, nextKey, nil
		}

		if !MatchToken(t, anyRoles, botName) {
			continue
		}

		out = append(out, t)
	}

	return out, "", nil
}

// MatchToken validates a token against a set of roles and a bot name filters.
// If a bot name is provided, it additionally checks if the token has a role of bot.
func MatchToken(t types.ProvisionToken, anyRoles types.SystemRoles, botName string) bool {
	if len(anyRoles) > 0 && !t.GetRoles().IncludeAny(anyRoles...) {
		return false
	}

	if botName != "" && (!t.GetRoles().Include(types.RoleBot) || t.GetBotName() != botName) {
		return false
	}

	return true
}

const tokensPrefix = "tokens"

func validateProvisionToken(token types.ProvisionToken) error {
	switch token.GetJoinMethod() {
	case types.JoinMethodOracle:
		return validateOracleJoinToken(token)

	case types.JoinMethodEC2:
		return validateEC2Token(token)

	case types.JoinMethodIAM:
		return validateIAMToken(token)
	}

	return nil
}

func validateEC2Token(token types.ProvisionToken) error {
	for _, allowRule := range token.GetAllowRules() {
		// EC2 join method does not support AWS Organizational Unit matchers, so we return an
		// error if any of the token rules contain them.
		if tokenRuleHasAWSOrganizationalUnitMatchers(allowRule) {
			return trace.BadParameter(`the %q join method does not support the "aws_organizational_units" parameter`, types.JoinMethodEC2)
		}
	}
	return nil
}

func tokenRuleHasAWSOrganizationalUnitMatchers(tokenRule *types.TokenRule) bool {
	return tokenRule.AWSOrganizationalUnits != nil &&
		(len(tokenRule.AWSOrganizationalUnits.Include) > 0 || len(tokenRule.AWSOrganizationalUnits.Exclude) > 0)
}

func validateIAMToken(token types.ProvisionToken) error {
	for _, allowRule := range token.GetAllowRules() {
		if err := validateIAMOrganizationRule(allowRule); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func validateIAMOrganizationRule(tokenRule *types.TokenRule) error {
	// In order to use Organizational Unit matchers, the token must specify the AWS Organization ID.
	if tokenRule.AWSOrganizationID == "" && tokenRuleHasAWSOrganizationalUnitMatchers(tokenRule) {
		return trace.BadParameter(`allow rule with "aws_organizational_units" matchers must also specify "aws_organization_id" when using the %q join method`, types.JoinMethodIAM)
	}

	// Return early if no OU matchers are specified.
	if !tokenRuleHasAWSOrganizationalUnitMatchers(tokenRule) {
		return nil
	}

	if len(tokenRule.AWSOrganizationalUnits.Include) == 0 {
		return trace.BadParameter(`at least one entry in "aws_organizational_units.include" must be specified`)
	}

	if slices.Contains(tokenRule.AWSOrganizationalUnits.Include, types.Wildcard) && len(tokenRule.AWSOrganizationalUnits.Include) > 1 {
		return trace.BadParameter(`when using wildcard for "aws_organizational_units.include", no other values are allowed`)
	}
	if slices.Contains(tokenRule.AWSOrganizationalUnits.Exclude, types.Wildcard) {
		return trace.BadParameter(`using wildcard in "aws_organizational_units.exclude" is not allowed`)
	}

	return nil
}

// validateOracleJoinToken validates the fields in a token using the Oracle
// join method. It's done here instead of in the client so the client doesn't
// have to import the Oracle SDK.
func validateOracleJoinToken(token types.ProvisionToken) error {
	tokenV2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("%v join method requires ProvisionTokenV2", types.JoinMethodOracle)
	}
	oracleSpec := tokenV2.Spec.Oracle
	if oracleSpec == nil {
		return trace.BadParameter("missing spec")
	}
	for _, allow := range oracleSpec.Allow {
		if _, err := oracle.ParseRegionFromOCID(allow.Tenancy); err != nil {
			return trace.BadParameter("invalid tenant: %v", allow.Tenancy)
		}
		for _, compartment := range allow.ParentCompartments {
			if _, err := oracle.ParseRegionFromOCID(compartment); err != nil {
				return trace.BadParameter("invalid compartment: %v", compartment)
			}
		}
		for _, region := range allow.Regions {
			if canonicalRegion, _ := oracle.ParseRegion(region); canonicalRegion == "" {
				return trace.BadParameter("invalid region: %v", region)
			}
		}
		for _, instanceID := range allow.Instances {
			if _, err := oracle.ParseRegionFromOCID(instanceID); err != nil {
				return trace.BadParameter("invalid instance OCID: %s", instanceID)
			}
		}
	}
	return nil
}
