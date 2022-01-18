/*
Copyright 2022 Gravitational, Inc.

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

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

// CacheCommand implements `tctl cache` group of commands
type CacheCommand struct {
	config *service.Config

	cacheReset *kingpin.CmdClause
}

// Initialize allows CacheCommand to plug itself into the CLI parser
func (c *CacheCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	// add cache command
	cache := app.Command("cache", "Manage teleport cache")

	c.cacheReset = cache.Command("reset", "Reset auth cache and all downstream caches")
}

// TryRun takes the CLI command as an argument (like "nodes ls") and executes it.
func (c *CacheCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.cacheReset.FullCommand():
		err = c.Reset(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

func (c *CacheCommand) Reset(client auth.ClientI) error {
	return trace.Wrap(client.ResetCache(context.Background()))
}
