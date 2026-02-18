// Teleport
// Copyright (C) 2026  Gravitational, Inc.
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
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gravitational/trace"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type scopedTokenCollection struct {
	tokens []*joiningv1.ScopedToken
}

func NewScopedTokenCollection(tokens []*joiningv1.ScopedToken) Collection {
	return &scopedTokenCollection{
		tokens: tokens,
	}
}

func (c *scopedTokenCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.tokens))
	for i, resource := range c.tokens {
		r[i] = types.ProtoResource153ToLegacy(resource)
	}
	return r
}

func (c *scopedTokenCollection) WriteText(w io.Writer, verbose bool) error {
	// when calling the getScopedToken, the secrets would already have been properly hidden or exposed.
	_, err := ScopedTokenTextHelper(c.tokens, false).WriteTo(w)
	return trace.Wrap(err)
}

func scopedTokenHandler() Handler {
	return Handler{
		getHandler:    getScopedToken,
		createHandler: createScopedToken,
		deleteHandler: deleteScopedToken,
		description:   "Scoped invitation tokens that can be used to provision resources at a limited scope",
	}
}

func createScopedToken(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	verb := "created"
	r, err := services.UnmarshalProtoResource[*joiningv1.ScopedToken](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	var token *joiningv1.ScopedToken
	if opts.Force {
		token, err = client.UpsertScopedToken(ctx, r)
		verb = "updated"
	} else {
		token, err = client.CreateScopedToken(ctx, r)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		"%v %q has been %s\n",
		types.KindScopedToken,
		token.GetMetadata().GetName(),
		verb,
	)

	return nil
}

func getScopedToken(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	// If a specific token name is requested, filter the results
	if ref.Name != "" {
		token, err := client.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
			Name:       ref.Name,
			WithSecret: opts.WithSecrets,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !opts.WithSecrets {
			token.GetStatus().Secret = "******"
		}
		return &scopedTokenCollection{[]*joiningv1.ScopedToken{token}}, nil
	}

	tokens, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageKey string) ([]*joiningv1.ScopedToken, string, error) {
		res, err := client.ListScopedTokens(ctx, &joiningv1.ListScopedTokensRequest{
			Limit:       uint32(pageSize),
			Cursor:      pageKey,
			WithSecrets: opts.WithSecrets,
		})
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		if !opts.WithSecrets {
			for _, token := range res.GetTokens() {
				token.GetStatus().Secret = "******"
			}
		}

		return res.GetTokens(), res.GetCursor(), nil
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &scopedTokenCollection{tokens: tokens}, nil
}

func deleteScopedToken(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteScopedToken(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf(
		"%v %q has been deleted\n",
		types.KindScopedToken,
		ref.Name,
	)
	return nil
}

func ScopedTokenTextHelper(tokens []*joiningv1.ScopedToken, withSecrets bool) *bytes.Buffer {
	table := asciitable.MakeTable([]string{"Token", "Secret", "Type", "Scope", "Assigns Scope", "Labels", "Expiry Time (UTC)"})

	secretFunc := func(t *joiningv1.ScopedToken) string {
		if withSecrets {
			return t.GetStatus().GetSecret()
		}
		return "******"
	}

	now := time.Now()
	for _, t := range tokens {
		expiry := "never"
		expiresAt := t.GetMetadata().GetExpires().AsTime()
		if !expiresAt.IsZero() && expiresAt.Unix() != 0 {
			exptime := expiresAt.Format(time.RFC822)
			expdur := expiresAt.Sub(now).Round(time.Second)
			expiry = fmt.Sprintf("%s (%s)", exptime, expdur.String())
		}
		table.AddRow([]string{t.GetMetadata().GetName(), secretFunc(t), strings.Join(t.GetSpec().GetRoles(), ","), t.GetScope(), t.GetSpec().GetAssignedScope(), PrintMetadataLabels(t.GetMetadata().Labels), expiry})
	}
	return table.AsBuffer()
}
