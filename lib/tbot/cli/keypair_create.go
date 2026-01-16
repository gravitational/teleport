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

package cli

import (
	"io"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// KeypairCreateCommand handles `tbot keypair create`
type KeypairCreateCommand struct {
	*genericExecutorHandler[KeypairCreateCommand]

	ProxyServer   string
	Storage       string
	Overwrite     bool
	Format        string
	Static        bool
	StaticKeyPath string

	// GetSuite is an optional function to fetch crypto suites. Used to override
	// in tests, may be nil.
	GetSuite cryptosuites.GetSuiteFunc

	// Writer is an optional writer to which output from this command should be
	// written. If nil, stdout is used. Used in tests.
	Writer io.Writer
}

// NewKeypairCreateCommand initializes the `keypair create` command and returns
// a struct to contain the parse result.
func NewKeypairCreateCommand(parentCmd KingpinClause, action func(*KeypairCreateCommand) error) *KeypairCreateCommand {
	cmd := parentCmd.Command("create", "Creates a keypair to preregister for bound-keypair joining.")

	c := &KeypairCreateCommand{}
	c.genericExecutorHandler = newGenericExecutorHandler(cmd, c, action)

	cmd.Flag("storage", "The internal storage URI to write the keypair to, such as `file:///var/lib/teleport/bot`.").StringVar(&c.Storage)
	cmd.Flag("proxy-server", "The proxy server, which will be pinged to determine the current cryptographic suite in use.").Required().StringVar(&c.ProxyServer)
	cmd.Flag("overwrite", "If set, overwrite any existing keypair. If unset and a keypair already exists, its key will be printed for use.").BoolVar(&c.Overwrite)
	cmd.Flag("format", "Output format, one of: text, json").Default(teleport.Text).EnumVar(&c.Format, teleport.Text, teleport.JSON)
	cmd.Flag("static", "If set, creates a static private key instead of writing a mutable key into bot storage. If --static-key-path is unset, the key will be printed to the terminal.").BoolVar(&c.Static)
	cmd.Flag("static-key-path", "If set, writes the static private key to a file.").StringVar(&c.StaticKeyPath)

	return c
}
