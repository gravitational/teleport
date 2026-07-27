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
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/pkixname"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/subca"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

type pemFileList []string

func (p *pemFileList) Set(val string) error {
	*p = append(*p, val)
	return nil
}

func (p *pemFileList) String() string {
	return ""
}

// IsCumulative tells kingpin that it's OK to call Set multiple times.
func (p *pemFileList) IsCumulative() bool {
	return true
}

// cliCATypes maps subca.SupportedCATypes() into their CLI-friendly counterparts
// (typically by replacing "_" with "-").
//
// Matches "tctl auth export --types" whenever appropriate.
var cliCATypes = makeCATypeNames()

type caTypeNames struct {
	// Names holds all CA type names for --help.
	Names []string
	// NameToCAType maps a "display" CA type to an actual CA type
	// (eg, "db-client" -> "db_client").
	// The map is sparse, ie, CA types that need no mapping aren't present in it.
	NameToCAType map[string]string
}

func makeCATypeNames() caTypeNames {
	nameToCAType := make(map[string]string)
	var names []string

	for _, caType := range subca.SupportedCATypes() {
		name := strings.ReplaceAll(caType, "_", "-")
		if name != caType {
			nameToCAType[name] = caType
		}
		names = append(names, name)
	}

	slices.Sort(names)

	return caTypeNames{
		Names:        names,
		NameToCAType: nameToCAType,
	}
}

// Convert converts a CLI CA type into a subca supported CA type.
// The conversion is pass-through by design: it converts known types and lets
// others pass. The backend is the source of truth for which types are valid or
// not.
func (n caTypeNames) Convert(caType string) string {
	if val, ok := n.NameToCAType[caType]; ok {
		return val
	}
	return caType
}

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
	createOverride    createOverrideCommand
	updateOverride    updateOverrideCommand
	deleteOverride    deleteOverrideCommand
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

	// Don't over-validate CA types on the client. That's the server's responsibility.
	caTypesHelp := fmt.Sprintf(
		"CA type (%s)",
		strings.Join(cliCATypes.Names, ", "),
	)

	c.createOverrideCSR.CmdClause = parent.Command(
		"create-override-csr", "Create a CSR in preparation for CA certificate override")
	c.createOverrideCSR.
		Flag("type", caTypesHelp). // --type mimics other "tctl auth" commands
		Required().
		StringVar(&c.createOverrideCSR.cliCAType)
	c.createOverrideCSR.
		Flag("out", "If set writes CSRs to files using --out as the path prefix").
		StringVar(&c.createOverrideCSR.out)
	c.createOverrideCSR.
		Flag("public-key", "Public key hash of CA certificate to be targeted").
		StringVar(&c.createOverrideCSR.publicKey)
	c.createOverrideCSR.
		Flag("subject", `Customized certificate subject. Example: "O=MyClusterName,OU=MyOrgUnit,CN=MyCommonName"`).
		StringVar(&c.createOverrideCSR.subject)
	c.createOverrideCSR.
		Flag("local-only", "If true only private keys local to the replying Auth server are used.").
		BoolVar(&c.createOverrideCSR.localOnly)

	c.createOverride.CmdClause = parent.Command("create-override", "Add a single certificate override to a CA override resource")
	c.createOverride.Flag("type", caTypesHelp).
		Required().
		StringVar(&c.createOverride.cliCAType)
	c.createOverride.Arg("cert.pem", "CA override certificate file in PEM form").
		Required().
		StringVar(&c.createOverride.certFile)
	c.createOverride.Arg("chain.pem", "CA override trust chain files in PEM form").
		SetValue(&c.createOverride.chainFiles)
	c.createOverride.Flag("disabled", "If true creates a disabled override").
		BoolVar(&c.createOverride.disabled)
	c.createOverride.Flag("force", "If true attempts to force creation, ignoring select state validation").
		BoolVar(&c.createOverride.force)

	c.updateOverride.CmdClause = parent.Command("update-override", "Update a single certificate override in a CA override resource")
	c.updateOverride.Flag("type", caTypesHelp).
		Required().
		StringVar(&c.updateOverride.cliCAType)
	c.updateOverride.Flag("public-key", "Public key hash of the certificate override to be targeted").
		StringVar(&c.updateOverride.publicKey)
	c.updateOverride.Flag("set-cert", "CA override certificate file in PEM form").
		StringVar(&c.updateOverride.certFile)
	c.updateOverride.Flag("set-chain", "CA override trust chain files in PEM form").
		SetValue(&c.updateOverride.chainFiles)
	c.updateOverride.Flag("clear-chain", "Clears existing CA override trust chain").
		BoolVar(&c.updateOverride.chainClear)
	c.updateOverride.Flag("set-disabled", "If true disables the override, if false enables it").
		EnumVar(&c.updateOverride.disabled, "true", "false")
	c.updateOverride.Flag("force", "If true attempts to force the update. May be used to ignore select state validation or disable live overrides.").
		BoolVar(&c.updateOverride.force)

	c.deleteOverride.CmdClause = parent.Command("delete-override", "Delete a single certificate override from a CA override resource")
	c.deleteOverride.Flag("type", caTypesHelp).
		Required().
		StringVar(&c.deleteOverride.cliCAType)
	c.deleteOverride.Flag("public-key", "Public key hash of the certificate override to be targeted").
		Required().
		StringVar(&c.deleteOverride.publicKey)
	c.deleteOverride.Flag("force", "If true attempts to force deletion. May be used to delete live overrides.").
		BoolVar(&c.deleteOverride.force)

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
		&c.createOverride,
		&c.updateOverride,
		&c.deleteOverride,
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

	cliCAType string
	out       string // Output path prefix.
	publicKey string
	subject   string
	localOnly bool
}

func (c *createOverrideCSRCommand) Run(
	ctx context.Context,
	clientFunc InitFunc,
	s *commandState,
) error {
	// Defensive, shouldn't happen.
	if c.cliCAType == "" {
		return trace.BadParameter("type required")
	}
	caType := cliCATypes.Convert(c.cliCAType)

	var pubKey *subcav1.PublicKeyHash
	if c.publicKey != "" {
		pubKey = subcav1.PublicKeyHash_builder{
			Value: c.publicKey,
		}.Build()
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
	resp, err := subCA.CreateCSR(ctx, subcav1.CreateCSRRequest_builder{
		CaType:        caType,
		PublicKeyHash: pubKey,
		CustomSubject: customSubject,
		LocalOnly:     c.localOnly,
	}.Build())
	if err != nil {
		return trace.Wrap(err, "create CSRs")
	}
	// Defensive. Should fail server-side if that's the case.
	if len(resp.GetCsrs()) == 0 {
		return trace.BadParameter("no CSRs created")
	}

	// Print warnings to stderr.
	for _, warn := range resp.GetWarnings() {
		if pkh := warn.GetPublicKeyHash(); pkh != "" {
			fmt.Fprintf(s.Stderr, "public key %q: %s\n", pkh, warn.GetUserMessage())
		} else {
			fmt.Fprintln(s.Stderr, warn.GetUserMessage())
		}
	}

	// If writing to stdout there's no need to parse the PEMs or form filenames.
	if c.out == "" {
		for _, csr := range resp.GetCsrs() {
			fmt.Fprintln(s.Stdout, csr.GetPem())
		}
		return nil
	}

	// Parse CSRs and calculate public key hashes.
	publicKeyHashes := make([]string, len(resp.GetCsrs()))
	for i, csr := range resp.GetCsrs() {
		block, _ := pem.Decode([]byte(csr.GetPem()))
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
	for i, csr := range resp.GetCsrs() {
		name := c.out + caType + "-" + minHashes[i] + "-csr.pem"
		if err := os.WriteFile(name, []byte(csr.GetPem()), 0644); err != nil {
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

type createOverrideCommand struct {
	*kingpin.CmdClause

	cliCAType  string
	certFile   string
	chainFiles pemFileList
	disabled   bool
	force      bool
}

func (c *createOverrideCommand) Run(
	ctx context.Context,
	clientFunc InitFunc,
	s *commandState,
) error {
	caType := cliCATypes.Convert(c.cliCAType)

	certPEM, cert, err := readCertFile(c.certFile)
	if err != nil {
		return trace.Wrap(err, "cert.pem")
	}
	pkh := subca.HashCertificatePublicKey(cert)

	var chain []string
	for i, chainFile := range c.chainFiles {
		// Parse the cert so we know it's valid, but otherwise we only need the PEM.
		chainPEM, _, err := readCertFile(chainFile)
		if err != nil {
			return trace.Wrap(err, "chain%d.pem", i)
		}
		chain = append(chain, string(chainPEM))
	}

	authClient, closeFn, err := clientFunc(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeFn(ctx)

	_, err = authClient.SubCAClient().AddCertificateOverride(ctx, subcav1.AddCertificateOverrideRequest_builder{
		CaId: subcav1.CertAuthorityOverrideID_builder{
			CaType: caType,
		}.Build(),
		CertificateOverride: subcav1.CertificateOverride_builder{
			PublicKey:   pkh,
			Certificate: string(certPEM),
			Chain:       chain,
			Disabled:    c.disabled,
		}.Build(),
		ForceImmediateDisable: c.force,
	}.Build())
	if err != nil {
		return trace.Wrap(err, "create certificate override")
	}
	fmt.Fprintf(s.Stdout, "%s/%s: certificate override %s created\n", types.KindCertAuthorityOverride, caType, pkh)

	return nil
}

type updateOverrideCommand struct {
	*kingpin.CmdClause

	cliCAType  string
	publicKey  string
	certFile   string
	chainFiles pemFileList
	chainClear bool
	disabled   string
	force      bool
}

func (c *updateOverrideCommand) Run(
	ctx context.Context,
	clientFunc InitFunc,
	s *commandState,
) error {
	switch {
	case c.publicKey == "" && c.certFile == "":
		return trace.BadParameter("--public-key required to choose target certificate override")
	case len(c.chainFiles) > 0 && c.chainClear:
		return trace.BadParameter("--set-chain and --clear-chain cannot be set together")
	}

	caType := cliCATypes.Convert(c.cliCAType)

	var builder subcav1.CertificateOverride_builder
	var paths []string // paths for FieldMask
	builder.PublicKey = c.publicKey

	if c.disabled != "" {
		val, err := strconv.ParseBool(c.disabled)
		if err != nil {
			return trace.BadParameter("--disabled: %v", err)
		}
		builder.Disabled = val
		paths = append(paths, "disabled")
	}

	if c.certFile != "" {
		certPEM, cert, err := readCertFile(c.certFile)
		if err != nil {
			return trace.Wrap(err, "cert.pem")
		}
		if builder.PublicKey == "" {
			builder.PublicKey = subca.HashCertificatePublicKey(cert)
		}
		builder.Certificate = string(certPEM)
		paths = append(paths, "certificate")
	} else {
		builder.PublicKey = subca.NormalizePublicKey(builder.PublicKey)
	}

	for i, chainFile := range c.chainFiles {
		// Parse the cert so we know it's valid, but otherwise we only need the PEM.
		chainPEM, _, err := readCertFile(chainFile)
		if err != nil {
			return trace.Wrap(err, "chain%d.pem", i)
		}
		builder.Chain = append(builder.Chain, string(chainPEM))
	}
	if len(builder.Chain) > 0 || c.chainClear {
		paths = append(paths, "chain")
	}

	if len(paths) == 0 {
		return trace.BadParameter("one of --set-* or --clear-* flags is required to update")
	}

	// Backfill a normalized "public_key" as part of the update.
	// There's no harm in doing so.
	paths = append(paths, "public_key")

	certificateOverride := builder.Build()
	updateMask, err := fieldmaskpb.New(certificateOverride, paths...)
	if err != nil {
		return trace.Wrap(err)
	}

	authClient, closeFn, err := clientFunc(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeFn(ctx)

	_, err = authClient.SubCAClient().UpdateCertificateOverride(ctx, subcav1.UpdateCertificateOverrideRequest_builder{
		CaId: subcav1.CertAuthorityOverrideID_builder{
			CaType: caType,
		}.Build(),
		CertificateOverride:   certificateOverride,
		UpdateMask:            updateMask,
		ForceImmediateDisable: c.force,
	}.Build())
	if err != nil {
		return trace.Wrap(err, "update certificate override")
	}
	fmt.Fprintf(s.Stdout,
		"%s/%s: certificate override %s updated\n",
		types.KindCertAuthorityOverride,
		caType, certificateOverride.GetPublicKey(),
	)

	return nil
}

func readCertFile(certFile string) (pem []byte, _ *x509.Certificate, _ error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, nil, trace.Wrap(err, "read certificate file")
	}
	// Trim spaces. Strict server validation doesn't allow it.
	certPEM = bytes.TrimSpace(certPEM)

	cert, err := tlsutils.ParseCertificatePEMStrict(certPEM)
	if err != nil {
		return nil, nil, trace.Wrap(err, "parse certificate")
	}
	return certPEM, cert, nil
}

type deleteOverrideCommand struct {
	*kingpin.CmdClause

	cliCAType string
	publicKey string
	force     bool
}

func (c *deleteOverrideCommand) Run(
	ctx context.Context,
	clientFunc InitFunc,
	s *commandState,
) error {
	caType := cliCATypes.Convert(c.cliCAType)

	authClient, closeFn, err := clientFunc(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeFn(ctx)

	_, err = authClient.SubCAClient().RemoveCertificateOverride(ctx, subcav1.RemoveCertificateOverrideRequest_builder{
		CertificateOverrideId: subcav1.CertificateOverrideID_builder{
			CaType: caType,
			PublicKeyHash: subcav1.PublicKeyHash_builder{
				Value: c.publicKey,
			}.Build(),
		}.Build(),
		ForceImmediateDelete: c.force,
	}.Build())
	if err != nil {
		return trace.Wrap(err, "delete certificate override")
	}
	fmt.Fprintf(s.Stdout, "%s/%s: certificate override %s deleted\n", types.KindCertAuthorityOverride, caType, c.publicKey)

	return nil
}
