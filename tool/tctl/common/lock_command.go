/*
Copyright 2021 Gravitational, Inc.

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

package common

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service/servicecfg"
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
func (c *LockCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	c.config = config

	c.mainCmd = app.Command("lock", "Create a new lock.")
	c.mainCmd.Flag("user", "Name of a Teleport user to disable.").StringVar(&c.spec.Target.User)
	c.mainCmd.Flag("role", "Name of a Teleport role to disable.").StringVar(&c.spec.Target.Role)
	c.mainCmd.Flag("login", "Name of a local UNIX user to disable.").StringVar(&c.spec.Target.Login)
	// Locking a node is now deprecated, but we still support it for backwards compatibility.
	// Previously, locking a node would lock only the `ssh_service` from that node to
	// access Teleport but didn't prevent any other roles that the same instance could run.
	// Now, `tctl lock --server-id` should be used instead to lock the entire server
	// and all roles that it runs (including the `ssh_service`) from accessing Teleport.
	// TODO: DELETE IN 15.0.0
	c.mainCmd.Flag("node", "UUID of a Teleport node to disable.").Hidden().StringVar(&c.spec.Target.Node)
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
func (c *LockCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.mainCmd.FullCommand():
		err = c.CreateLock(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// CreateLock creates a lock for the main `tctl lock` command.
func (c *LockCommand) CreateLock(ctx context.Context, client auth.ClientI) error {
	// Locking a node is now deprecated, but we still support it for backwards compatibility.
	// Previously, locking a node would lock only the `ssh_service` from that node to
	// access Teleport but didn't prevent any other roles that the same instance could run.
	// Now, `tctl lock --server-id` should be used instead to lock the entire server.
	// TODO: DELETE IN 15.0.0
	if c.spec.Target.Node != "" {
		c.config.Log.Warnf("`tctl lock --node <id>` is now deprecated. Please use `tctl lock --server-id <id>` instead.")
	}

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
