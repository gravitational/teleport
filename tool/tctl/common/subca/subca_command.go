// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package subca

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/subca"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// Command is the subset of "tctl auth" commands that deal with Sub CAs.
//
// Namely:
//   - tctl auth create-override-csr
//   - tctl auth create-override
//   - tctl auth update-override
//   - tctl auth delete-override
//   - tctl auth pub-key-hash
type Command struct {
	// Stdin is a test-friendly pointer to stdin.
	// Initialized by the Initialize function.
	Stdin io.Reader
	// Stdout and Stderr are test-friendly pointers to stdout/stderr.
	// Initialized by the Initialize function.
	Stdout, Stderr io.Writer

	pubKeyHash pubKeyHashCommand
}

// Initialize initializes all Sub CA commands. Parent is expected to be "tctl
// auth".
//
// Mimics CLICommand.Initialize.
func (c *Command) Initialize(
	parent *kingpin.CmdClause,
	_ *tctlcfg.GlobalCLIFlags,
	_ *servicecfg.Config,
) {
	if c.Stdin == nil {
		c.Stdin = os.Stdin
	}
	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}
	if c.Stderr == nil {
		c.Stderr = os.Stderr
	}

	c.pubKeyHash.CmdClause = parent.Command(
		"pub-key-hash", "Extract and print the public key hash of a PEM certificate")
	c.pubKeyHash.Flag("cert", "Certificate file in PEM format. Use '-' to read from stdin.").
		Required().
		StringVar(&c.pubKeyHash.certFile)
}

// TryRun attempts to run the selected command. Returns true if the commands
// matches, false otherwise.
//
// Mimics tool/tctl/common.CLICommand.TryRun.
func (c *Command) TryRun(
	ctx context.Context,
	selectedCommand string,
	clientFunc commonclient.InitFunc,
) (match bool, err error) {
	if selectedCommand == c.pubKeyHash.FullCommand() {
		return true, trace.Wrap(c.pubKeyHash.Run(ctx, clientFunc, c.state()))
	}
	return false, nil
}

func (c *Command) state() *commandState {
	return &commandState{
		Stdin:  c.Stdin,
		Stdout: c.Stdout,
		Sdterr: c.Stderr,
	}
}

type commandState struct {
	Stdin          io.Reader
	Stdout, Sdterr io.Writer
}

type pubKeyHashCommand struct {
	*kingpin.CmdClause

	certFile string
}

func (c *pubKeyHashCommand) Run(
	ctx context.Context,
	_ commonclient.InitFunc,
	s *commandState,
) error {
	// Defensive, shouldn't happen.
	if c.certFile == "" {
		return trace.BadParameter("certificate required")
	}

	var source io.Reader
	if c.certFile == "-" {
		source = s.Stdin
	} else {
		f, err := os.Open(c.certFile)
		if err != nil {
			return trace.Wrap(err, "open file")
		}
		defer f.Close()
		source = f
	}

	certPEM, err := io.ReadAll(source)
	if err != nil {
		return trace.Wrap(err, "read certificate")
	}
	cert, err := tlsutils.ParseCertificatePEM(certPEM)
	if err != nil {
		return trace.Wrap(err, "parse certificate PEM")
	}

	fmt.Fprintln(s.Stdout, subca.HashCertificatePublicKey(cert))
	return nil
}
