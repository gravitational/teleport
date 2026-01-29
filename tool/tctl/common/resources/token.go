// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	services "github.com/gravitational/teleport/lib/services"
)

type tokenCollection struct {
	tokens []types.ProvisionToken
}

func (c *tokenCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.tokens))
	for _, resource := range c.tokens {
		r = append(r, resource)
	}
	return r
}

func (c *tokenCollection) WriteText(w io.Writer, verbose bool) error {
	for _, token := range c.tokens {
		if _, err := w.Write([]byte(token.String())); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func tokenHandler() Handler {
	return Handler{
		getHandler:    getToken,
		createHandler: createToken,
		deleteHandler: deleteToken,
		singleton:     false,
		mfaRequired:   true,
		description:   "Allows instances, bots, and human users to join the Teleport cluster.",
	}
}

func getToken(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		tokens, err := GetAllTokens(ctx, client)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &tokenCollection{tokens: tokens}, nil
	}
	token, err := client.GetToken(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &tokenCollection{tokens: []types.ProvisionToken{token}}, nil
}

// GetAllTokens is a helper that retrieves all kinds of tokens.
// The caller MUST make sure the MFA ceremony has been performed and is stored in the context
// Else this function will cause several MFA prompts.
// The MFA ceremony cannot be done in this function because we don't know if
// the caller already attempted one (e.g. tctl get all)
func GetAllTokens(ctx context.Context, clt *authclient.Client) ([]types.ProvisionToken, error) {
	// There are 3 tokens types:
	// - provision tokens
	// - static tokens
	// - user tokens
	// This endpoint returns all 3 for compatibility reasons.
	// Before, all 3 tokens were returned by the same "GetTokens" RPC, now we are using
	// separate RPCs, with pagination. However, we don't know if the auth we are talking
	// to supports the new RPCs. As the static token one got introduced last, we
	// try to use it.If it works, we consume the two other RPCs. If it doesn't,
	// we fallback to the legacy all-in-one RPC.
	var tokens []types.ProvisionToken

	// Trying to get static tokens
	staticTokens, err := clt.GetStaticTokens(ctx)
	if err != nil && !trace.IsNotImplemented(err) {
		return nil, trace.Wrap(err, "getting static tokens")
	}

	// TODO(hugoShaka): DELETE IN 19.0.0
	if trace.IsNotImplemented(err) {
		// We are connected to an old auth, that doesn't support the per-token type RPCs
		// so we fallback to the legacy all-in-one RPC.
		tokens, err := clt.GetTokens(ctx)
		return tokens, trace.Wrap(err, "getting all tokens through the legacy RPC")
	}

	// We are connected to a modern auth, we must collect all 3 tokens types.
	// Getting the provision tokens.
	provisionTokens, err := stream.Collect(clientutils.Resources(ctx,
		func(ctx context.Context, pageSize int, pageKey string) ([]types.ProvisionToken, string, error) {
			return clt.ListProvisionTokens(ctx, pageSize, pageKey, nil, "")
		},
	))
	if err != nil {
		return nil, trace.Wrap(err, "getting provision tokens")
	}
	tokens = append(staticTokens.GetStaticTokens(), provisionTokens...)

	// Getting the user tokens.
	userTokens, err := stream.Collect(clientutils.Resources(ctx, clt.ListResetPasswordTokens))
	if err != nil && !trace.IsNotImplemented(err) {
		return nil, trace.Wrap(err)
	}
	if err != nil {
		return nil, trace.Wrap(err, "getting user tokens")
	}
	// Converting the user tokens as provision tokens for presentation and
	// backward compatibility.
	for _, t := range userTokens {
		roles := types.SystemRoles{types.RoleSignup}
		tok, err := types.NewProvisionToken(t.GetName(), roles, t.Expiry())
		if err != nil {
			return nil, trace.Wrap(err, "converting user token as a provision token")
		}
		tokens = append(tokens, tok)
	}

	return tokens, nil
}

// createToken implements `tctl create token.yaml` command.
func createToken(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	token, err := services.UnmarshalProvisionToken(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	err = client.UpsertToken(ctx, token)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("provision_token %q has been created\n", token.GetName())
	return nil
}

// deleteToken implements `tctl rm token/foo` command.
func deleteToken(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteToken(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("token %q has been deleted\n", ref.Name)
	return nil
}
