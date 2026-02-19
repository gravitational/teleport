/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package common

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
)

// ScopedTokensCommand implements `tctl scoped tokens` group of commands
type ScopedTokensCommand struct {
	config *servicecfg.Config

	withSecrets bool

	// format is the output format, e.g. text or json
	format string

	// tokenType is the type of token. For example, "node".
	tokenType string

	// name of the token. Can be used to either act on a
	// token (for example, delete a token) or used to create a token with a
	// specific name.
	name string

	// assignedScope allows for filtering tokens by the scope they assign
	assignedScope string

	// tokenScope allows for filtering tokens by the scope they belong to
	tokenScope string

	// ttl is how long the token will live for.
	ttl time.Duration

	// mode is the usage mode of a token.
	mode string
	// labels are optional token labels assigned to the token itself
	labels string

	// sshLabels are the ssh labels that should be assigned to a node token
	sshLabels string

	// tokenAdd is used to add a token.
	tokenAdd *kingpin.CmdClause

	// tokenDel is used to delete a token.
	tokenDel *kingpin.CmdClause

	// tokenList is used to view all tokens that Teleport knows about.
	tokenList *kingpin.CmdClause

	// Stdout allows to switch the standard output source. Used in tests.
	Stdout io.Writer
}

// Initialize allows TokenCommand to plug itself into the CLI parser
func (c *ScopedTokensCommand) Initialize(scopedCmd *kingpin.CmdClause, config *servicecfg.Config) {
	c.config = config
	tokens := scopedCmd.Command("tokens", "List or revoke scoped invitation tokens")

	formats := []string{teleport.Text, teleport.JSON, teleport.YAML}

	// tctl scoped tokens add ..."
	c.tokenAdd = tokens.Command("add", "Create a scoped invitation token.")
	c.tokenAdd.Flag("type", "Type(s) of token to add, e.g. --type=node").Required().StringVar(&c.tokenType)
	c.tokenAdd.Flag("name", "Override the default, randomly generated token name with a specified name").StringVar(&c.name)
	c.tokenAdd.Flag("ttl", fmt.Sprintf("Set expiration time for token, default is %v minutes",
		int(defaults.ProvisioningTokenTTL/time.Minute))).
		Default(fmt.Sprintf("%v", defaults.ProvisioningTokenTTL)).
		DurationVar(&c.ttl)
	c.tokenAdd.Flag("format", "Output format, 'text', 'json', or 'yaml'").EnumVar(&c.format, formats...)
	c.tokenAdd.Flag("assign-scope", "Scope that should be applied to resources provisioned by this token").StringVar(&c.assignedScope)
	c.tokenAdd.Flag("scope", "Scope assigned to the token itself").StringVar(&c.tokenScope)
	c.tokenAdd.Flag("mode", "Usage mode of a token (default: unlimited, single_use)").StringVar(&c.mode)
	c.tokenAdd.Flag("labels", "Set token labels, e.g. env=prod,region=us-west").StringVar(&c.labels)
	c.tokenAdd.Flag("ssh-labels", "Set immutable ssh labels the token should assign to provisioned resources, e.g. env=prod,region=us-west").StringVar(&c.sshLabels)

	// "tctl scoped tokens rm ..."
	c.tokenDel = tokens.Command("rm", "Delete/revoke a scoped invitation token.").Alias("del")
	c.tokenDel.Arg("token", "Token to delete").StringVar(&c.name)

	// "tctl scoped tokens ls"
	c.tokenList = tokens.Command("ls", "List invitation tokens.")
	c.tokenList.Flag("format", "Output format, 'text', 'json' or 'yaml'").EnumVar(&c.format, formats...)
	c.tokenList.Flag("with-secrets", "Do not redact join tokens").BoolVar(&c.withSecrets)

	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}
}

// TryRun attempts to run subcommands like like "scoped tokens ls".
func (c *ScopedTokensCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.tokenAdd.FullCommand():
		commandFunc = c.Add
	case c.tokenDel.FullCommand():
		commandFunc = c.Del
	case c.tokenList.FullCommand():
		commandFunc = c.List
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

// Add is called to execute "scoped tokens add ..." command.
func (c *ScopedTokensCommand) Add(ctx context.Context, client *authclient.Client) error {
	// Parse string to see if it's a type of role that Teleport supports.
	roles, err := types.ParseTeleportRoles(c.tokenType)
	if err != nil {
		return trace.Wrap(err)
	}

	// If it's Kube, then enable App and Discovery roles automatically so users
	// don't have problems with running Kubernetes App Discovery by default.
	if len(roles) == 1 && roles[0] == types.RoleKube {
		roles = append(roles, types.RoleApp, types.RoleDiscovery)
	}

	tokenName := c.name

	var labels map[string]string
	if c.labels != "" {
		labels, err = libclient.ParseLabelSpec(c.labels)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	var immutableLabels *joiningv1.ImmutableLabels
	if c.sshLabels != "" {
		sshLabels, err := libclient.ParseLabelSpec(c.sshLabels)
		if err != nil {
			return trace.Wrap(err)
		}
		immutableLabels = &joiningv1.ImmutableLabels{
			Ssh: sshLabels,
		}
	}
	expires := time.Now().UTC().Add(c.ttl)
	tok := &joiningv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    tokenName,
			Expires: timestamppb.New(expires),
			Labels:  labels,
		},
		Scope: c.tokenScope,
		Spec: &joiningv1.ScopedTokenSpec{
			Roles:           roles.StringSlice(),
			AssignedScope:   c.assignedScope,
			UsageMode:       cmp.Or(c.mode, joining.TokenUsageModeUnlimited),
			ImmutableLabels: immutableLabels,
		},
	}

	tok, err = client.CreateScopedToken(ctx, tok)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists(
				"failed to create scoped token (%q already exists), please use another name",
				tokenName,
			)
		}
		return trace.Wrap(err, "creating scoped token")
	}

	tokenName = tok.GetMetadata().GetName()
	tokenSecret := tok.GetStatus().GetSecret()
	// Print token information formatted with JSON, YAML, or just print the raw token.
	switch c.format {
	case teleport.JSON, teleport.YAML:
		expires := time.Now().Add(c.ttl)
		tokenInfo := map[string]any{
			"token":        tokenName,
			"token_secret": tokenSecret,
			"roles":        roles,
			"scope":        tok.GetScope(),
			"assign_scope": tok.GetSpec().GetAssignedScope(),
			"expires":      expires,
		}

		var (
			data []byte
			err  error
		)
		if c.format == teleport.JSON {
			data, err = json.MarshalIndent(tokenInfo, "", " ")
		} else {
			data, err = yaml.Marshal(tokenInfo)
		}
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprint(c.Stdout, string(data))

		return nil
	case teleport.Text:
		fmt.Fprintln(c.Stdout, tokenName)
		return nil
	}

	return trace.Wrap(showJoinInstructions(ctx, joinInstructionsInput{
		out:         c.Stdout,
		ttl:         c.ttl,
		roles:       roles,
		tokenName:   tokenName,
		tokenSecret: tokenSecret,
		client:      client,
	}))
}

// Del is called to execute "scoped tokens del ..." command.
func (c *ScopedTokensCommand) Del(ctx context.Context, client *authclient.Client) error {
	if c.name == "" {
		return trace.BadParameter("Need an argument: token")
	}
	if err := client.DeleteScopedToken(ctx, c.name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(c.Stdout, "Token %s has been deleted\n", c.name)
	return nil
}

// List is called to execute "tokens ls" command.
func (c *ScopedTokensCommand) List(ctx context.Context, client *authclient.Client) error {
	tokens, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageKey string) ([]*joiningv1.ScopedToken, string, error) {
		res, err := client.ListScopedTokens(ctx, &joiningv1.ListScopedTokensRequest{
			Limit:       uint32(pageSize),
			Cursor:      pageKey,
			WithSecrets: c.withSecrets,
		})
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		return res.GetTokens(), res.GetCursor(), nil
	}))
	if err != nil {
		return trace.Wrap(err, "listing scoped tokens")
	}

	if len(tokens) == 0 && c.format == teleport.Text {
		fmt.Fprintln(c.Stdout, "No active tokens found.")
		return nil
	}

	// Sort by expire time.
	slices.SortStableFunc(tokens, func(left, right *joiningv1.ScopedToken) int {
		return left.GetMetadata().GetExpires().AsTime().Compare(right.GetMetadata().GetExpires().AsTime())
	})

	secretFunc := func(tok *joiningv1.ScopedToken) string {
		if c.withSecrets {
			return tok.GetStatus().GetSecret()
		}
		return "******"
	}
	switch c.format {
	case teleport.JSON:
		err := utils.WriteJSONArray(c.Stdout, tokens)
		if err != nil {
			return trace.Wrap(err, "failed to marshal tokens")
		}
	case teleport.YAML:
		err := utils.WriteYAML(c.Stdout, tokens)
		if err != nil {
			return trace.Wrap(err, "failed to marshal tokens")
		}
	case teleport.Text:
		for _, token := range tokens {
			fmt.Fprintln(c.Stdout, token.GetMetadata().GetName())
		}
	default:
		tokensView := func() string {
			table := asciitable.MakeTable([]string{"Token", "Secret", "Type", "Scope", "Assigns Scope", "Labels", "Expiry Time (UTC)"})
			now := time.Now()
			for _, t := range tokens {
				expiry := "never"
				expiresAt := t.GetMetadata().GetExpires().AsTime()
				if !expiresAt.IsZero() && expiresAt.Unix() != 0 {
					exptime := expiresAt.Format(time.RFC822)
					expdur := expiresAt.Sub(now).Round(time.Second)
					expiry = fmt.Sprintf("%s (%s)", exptime, expdur.String())
				}
				table.AddRow([]string{t.GetMetadata().GetName(), secretFunc(t), strings.Join(t.GetSpec().GetRoles(), ","), t.GetScope(), t.GetSpec().GetAssignedScope(), printMetadataLabels(t.GetMetadata().Labels), expiry})
			}
			return table.AsBuffer().String()
		}
		fmt.Fprint(c.Stdout, tokensView())
	}
	return nil
}
