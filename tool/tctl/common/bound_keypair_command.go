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
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

type BoundKeypairCommand struct {
	token string

	requestRotation *kingpin.CmdClause
}

func (c *BoundKeypairCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	cmd := app.Command("bound-keypair", "Manage bound-keypair joining tokens")

	c.requestRotation = cmd.Command("request-rotation", "Request a keypair rotation on the next join attempt.")
	c.requestRotation.Arg("name", "The name of the token").Required().StringVar(&c.token)
}

func (c *BoundKeypairCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error

	switch cmd {
	case c.requestRotation.FullCommand():
		commandFunc = c.RequestRotation
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

func (c *BoundKeypairCommand) RequestRotation(ctx context.Context, client *authclient.Client) error {
	// Perform MFA checks now since we'll otherwise need to prompt twice.
	if _, err := mfa.MFAResponseFromContext(ctx); err == nil {
		// Nothing to do.
	} else {
		mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/)
		if err == nil {
			ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
		} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
			return trace.Wrap(err)
		}
	}

	token, err := client.GetToken(ctx, c.token)
	if err != nil {
		return trace.Wrap(err)
	}

	if token.GetJoinMethod() != types.JoinMethodBoundKeypair {
		return trace.BadParameter(
			"token %s is of type %s, not %s",
			c.token, token.GetJoinMethod(), types.JoinMethodBoundKeypair,
		)
	}

	v2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("unsupported token type %T", token)
	}

	now := time.Now()
	v2.Spec.BoundKeypair.RotateAfter = &now

	if err := client.UpsertToken(ctx, v2); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Token rotation flag has been set, rotation will be required during the next authentication attempt.\n")

	return nil
}
