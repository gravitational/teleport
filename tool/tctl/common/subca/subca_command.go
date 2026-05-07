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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/utils/pkixname"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/subca"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// SubCAClientSource is the subset of *authclient.Client used by Command.
type SubCAClientSource interface {
	SubCAClient() subcav1.SubCAServiceClient
}

// InitFunc mimics commonclient.InitFunc, but types the client to the narrower
// SubCAClientSource interface for testing.
type InitFunc func(ctx context.Context) (_ SubCAClientSource, closeFn func(context.Context), _ error)

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

	createOverrideCSR createOverrideCSRCommand
	pubKeyHash        pubKeyHashCommand
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

	c.createOverrideCSR.CmdClause = parent.Command(
		"create-override-csr", "Create a CSR in preparation for CA certificate override")
	// Don't over-validate CA types on the client. That's the server's responsibility.
	createCSRHelp := fmt.Sprintf(
		"CA type (%s)",
		strings.Join(subca.SupportedCATypes(), ", "),
	)
	c.createOverrideCSR.
		Flag("type", createCSRHelp). // --type mimics other "tctl auth" commands
		Required().
		StringVar(&c.createOverrideCSR.caType)
	c.createOverrideCSR.
		Flag("out", "If set writes CSRs to files using --out as the path prefix").
		StringVar(&c.createOverrideCSR.out)
	c.createOverrideCSR.
		Flag("public-key", "Public key hash of CA certificate to be targeted").
		StringVar(&c.createOverrideCSR.publicKey)
	c.createOverrideCSR.
		Flag("subject", `Customized certificate subject. Example: "O=MyClusterName,OU=MyOrgUnit,CN=MyCommonName"`).
		StringVar(&c.createOverrideCSR.subject)

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
	clientFunc InitFunc,
) (match bool, err error) {
	for _, cmd := range []subCommand{
		&c.createOverrideCSR,
		&c.pubKeyHash,
	} {
		if selectedCommand == cmd.FullCommand() {
			return true, trace.Wrap(cmd.Run(ctx, clientFunc, c.state()))
		}
	}
	return false, nil
}

func (c *Command) state() *commandState {
	return &commandState{
		Stdin:  c.Stdin,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	}
}

type commandState struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

type subCommand interface {
	FullCommand() string
	Run(ctx context.Context, clientFunc InitFunc, s *commandState) error
}

type createOverrideCSRCommand struct {
	*kingpin.CmdClause

	caType    string
	out       string // Output path prefix.
	publicKey string
	subject   string
}

func (c *createOverrideCSRCommand) Run(
	ctx context.Context,
	clientFunc InitFunc,
	s *commandState,
) error {
	// Defensive, shouldn't happen.
	if c.caType == "" {
		return trace.BadParameter("type required")
	}

	var pubKey *subcav1.PublicKeyHash
	if c.publicKey != "" {
		pubKey = &subcav1.PublicKeyHash{
			Value: c.publicKey,
		}
	}

	var customSubject *subcav1.DistinguishedName
	if c.subject != "" {
		dn, err := pkixname.ParseDistinguishedName(c.subject)
		if err != nil {
			return trace.Wrap(err, "parse custom subject")
		}
		customSubject, err = subca.RDNSequenceToDistinguishedNameProto(dn.ToRDNSequence())
		if err != nil {
			return trace.Wrap(err, "convert custom subject to protobuf")
		}
	}

	// Create gRPC client.
	authClient, closeFn, err := clientFunc(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeFn(ctx)
	subCA := authClient.SubCAClient()

	// Request CSRs.
	resp, err := subCA.CreateCSR(ctx, &subcav1.CreateCSRRequest{
		CaType:        c.caType,
		PublicKeyHash: pubKey,
		CustomSubject: customSubject,
	})
	if err != nil {
		return trace.Wrap(err, "create CSRs")
	}
	// Defensive. Should fail server-side if that's the case.
	if len(resp.Csrs) == 0 {
		return trace.BadParameter("no CSRs created")
	}

	// If writing to stdout there's no need to parse the PEMs or form filenames.
	if c.out == "" {
		for _, csr := range resp.Csrs {
			fmt.Fprintln(s.Stdout, csr.Pem)
		}
		return nil
	}

	// Parse CSRs and calculate public key hashes.
	publicKeyHashes := make([]string, len(resp.Csrs))
	for i, csr := range resp.Csrs {
		block, _ := pem.Decode([]byte(csr.Pem))
		if block == nil {
			return trace.BadParameter("csrs[%d]: CSR is not a valid PEM", i)
		}
		parsedCSR, err := x509.ParseCertificateRequest(block.Bytes)
		if err != nil {
			return trace.Wrap(err, "csrs[%d]: parse CSR", i)
		}
		publicKeyHashes[i] = subca.HashPublicKey(parsedCSR.RawSubjectPublicKeyInfo)
	}

	// Find smaller hashes so the filenames aren't always 73+ characters.
	minHashes := findMinHashes(publicKeyHashes)

	// Write output files.
	for i, csr := range resp.Csrs {
		name := c.out + c.caType + "-" + minHashes[i] + "-csr.pem"
		if err := os.WriteFile(name, []byte(csr.Pem), 0644); err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintf(s.Stdout, "Wrote %s\n", name)
	}
	return nil
}

// findMinHashes finds a smaller, non-conflicting prefix of hashes.
// Returns a slice containing the prefix hashes, all with the same length.
func findMinHashes(hashes []string) []string {
	if len(hashes) == 0 {
		return nil
	}

	minHashes := make([]string, len(hashes))

	const startLen = 8
	for minLen := startLen; true; minLen++ {
		seenHashes := make(map[string]struct{})
		trimmed := false

		// Attempt to trim all entries to minLen.
		for i, h := range hashes {
			if minLen < len(h) {
				minHashes[i] = h[:minLen]
				trimmed = true
			} else {
				minHashes[i] = h
			}
		}

		// If no hashes could be trimmed stop and return original slice.
		if !trimmed {
			break
		}

		// Look for a repeated hash. If there is none, return.
		collision := false
		for _, mh := range minHashes {
			if _, seen := seenHashes[mh]; seen {
				collision = true
				break
			}
			seenHashes[mh] = struct{}{}
		}
		if !collision {
			return minHashes
		}
	}

	return hashes
}

type pubKeyHashCommand struct {
	*kingpin.CmdClause

	certFile string
}

func (c *pubKeyHashCommand) Run(
	ctx context.Context,
	_ InitFunc,
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
