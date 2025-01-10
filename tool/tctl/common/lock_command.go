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

package common

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// LockCommand implements `tctl lock` group of commands.
type LockCommand struct {
	config  *servicecfg.Config
	mainCmd *kingpin.CmdClause
	spec    types.LockSpecV2
	expires string
	ttl     time.Duration
}

// Initialize allows LockCommand to plug itself into the CLI parser.
func (c *LockCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config

	c.mainCmd = app.Command("lock", "Create a new lock.")
	c.mainCmd.Flag("user", "Name of a Teleport user to disable.").StringVar(&c.spec.Target.User)
	c.mainCmd.Flag("role", "Name of a Teleport role to disable.").StringVar(&c.spec.Target.Role)
	c.mainCmd.Flag("login", "Name of a local UNIX user to disable.").StringVar(&c.spec.Target.Login)
	c.mainCmd.Flag("mfa-device", "UUID of a user MFA device to disable.").StringVar(&c.spec.Target.MFADevice)
	c.mainCmd.Flag("windows-desktop", "Name of a Windows desktop to disable.").StringVar(&c.spec.Target.WindowsDesktop)
	c.mainCmd.Flag("access-request", "UUID of an access request to disable.").StringVar(&c.spec.Target.AccessRequest)
	c.mainCmd.Flag("device", "UUID of a trusted device to disable.").StringVar(&c.spec.Target.Device)
	c.mainCmd.Flag("message", "Message to display to locked-out users.").StringVar(&c.spec.Message)
	c.mainCmd.Flag("expires", "Time point (RFC3339) when the lock expires.").StringVar(&c.expires)
	c.mainCmd.Flag("ttl", "Time duration after which the lock expires.").DurationVar(&c.ttl)
	c.mainCmd.Flag("server-id", "UUID of a Teleport server to disable.").StringVar(&c.spec.Target.ServerID)
}

// TryRun attempts to run subcommands.
func (c *LockCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.mainCmd.FullCommand():
		commandFunc = c.CreateLock
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

// CreateLock creates a lock for the main `tctl lock` command.
func (c *LockCommand) CreateLock(ctx context.Context, client *authclient.Client) error {
	lockExpiry, err := computeLockExpiry(c.expires, c.ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	c.spec.Expires = lockExpiry

	lock, err := types.NewLock(uuid.New().String(), c.spec)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.UpsertLock(ctx, lock); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Created a lock with name %q.\n", lock.GetName())
	return nil
}

func computeLockExpiry(expires string, ttl time.Duration) (*time.Time, error) {
	if expires != "" && ttl != 0 {
		return nil, trace.BadParameter("use only one of --expires and --ttl")
	}
	if expires != "" {
		t, err := time.Parse(time.RFC3339, expires)
		return &t, trace.Wrap(err)
	}
	if ttl != 0 {
		t := time.Now().UTC().Add(ttl)
		return &t, nil
	}
	return nil, nil
}
