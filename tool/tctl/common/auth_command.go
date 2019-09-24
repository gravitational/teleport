package common

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
)

// AuthCommand implements `tctl auth` group of commands
type AuthCommand struct {
	config                     *service.Config
	authType                   string
	genPubPath                 string
	genPrivPath                string
	genUser                    string
	genHost                    string
	genTTL                     time.Duration
	exportAuthorityFingerprint string
	exportPrivateKeys          bool
	output                     string
	outputFormat               client.IdentityFileFormat
	compatVersion              string
	compatibility              string

	rotateGracePeriod time.Duration
	rotateType        string
	rotateManualMode  bool
	rotateTargetPhase string

	authGenerate *kingpin.CmdClause
	authExport   *kingpin.CmdClause
	authSign     *kingpin.CmdClause
	authRotate   *kingpin.CmdClause
}

// Initialize allows TokenCommand to plug itself into the CLI parser
func (a *AuthCommand) Initialize(app *kingpin.Application, config *service.Config) {
	a.config = config

	// operations with authorities
	auth := app.Command("auth", "Operations with user and host certificate authorities (CAs)").Hidden()
	a.authExport = auth.Command("export", "Export public cluster (CA) keys to stdout")
	a.authExport.Flag("keys", "if set, will print private keys").BoolVar(&a.exportPrivateKeys)
	a.authExport.Flag("fingerprint", "filter authority by fingerprint").StringVar(&a.exportAuthorityFingerprint)
	a.authExport.Flag("compat", "export cerfiticates compatible with specific version of Teleport").StringVar(&a.compatVersion)
	a.authExport.Flag("type", "certificate type: 'user', 'host' or 'tls'").StringVar(&a.authType)

	a.authGenerate = auth.Command("gen", "Generate a new SSH keypair").Hidden()
	a.authGenerate.Flag("pub-key", "path to the public key").Required().StringVar(&a.genPubPath)
	a.authGenerate.Flag("priv-key", "path to the private key").Required().StringVar(&a.genPrivPath)

	a.authSign = auth.Command("sign", "Create an identity file(s) for a given user")
	a.authSign.Flag("user", "Teleport user name").StringVar(&a.genUser)
	a.authSign.Flag("host", "Teleport host name").StringVar(&a.genHost)
	a.authSign.Flag("out", "identity output").Short('o').StringVar(&a.output)
	a.authSign.Flag("format", fmt.Sprintf("identity format: %q (default) or %q", client.IdentityFormatFile, client.IdentityFormatOpenSSH)).Default(string(client.DefaultIdentityFormat)).StringVar((*string)(&a.outputFormat))
	a.authSign.Flag("ttl", "TTL (time to live) for the generated certificate").Default(fmt.Sprintf("%v", defaults.CertDuration)).DurationVar(&a.genTTL)
	a.authSign.Flag("compat", "OpenSSH compatibility flag").StringVar(&a.compatibility)

	a.authRotate = auth.Command("rotate", "Rotate certificate authorities in the cluster")
	a.authRotate.Flag("grace-period", "Grace period keeps previous certificate authorities signatures valid, if set to 0 will force users to relogin and nodes to re-register.").Default(fmt.Sprintf("%v", defaults.RotationGracePeriod)).DurationVar(&a.rotateGracePeriod)
	a.authRotate.Flag("manual", "Activate manual rotation , set rotation phases manually").BoolVar(&a.rotateManualMode)
	a.authRotate.Flag("type", "Certificate authority to rotate, rotates both host and user CA by default").StringVar(&a.rotateType)
	a.authRotate.Flag("phase", fmt.Sprintf("Target rotation phase to set, used in manual rotation, one of: %v", strings.Join(services.RotatePhases, ", "))).StringVar(&a.rotateTargetPhase)
}

// TryRun takes the CLI command as an argument (like "auth gen") and executes it
// or returns match=false if 'cmd' does not belong to it
func (a *AuthCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case a.authGenerate.FullCommand():
		err = a.GenerateKeys()
	case a.authExport.FullCommand():
		err = a.ExportAuthorities(client)
	case a.authSign.FullCommand():
		err = a.GenerateAndSignKeys(client)
	case a.authRotate.FullCommand():
		err = a.RotateCertAuthority(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// ExportAuthorities outputs the list of authorities in OpenSSH compatible formats
// If --type flag is given, only prints keys for CAs of this type, otherwise
// prints all keys
func (a *AuthCommand) ExportAuthorities(client auth.ClientI) error {
	var typesToExport []services.CertAuthType

	// this means to export TLS authority
	if a.authType == "tls" {
		clusterName, err := client.GetDomainName()
		if err != nil {
			return trace.Wrap(err)
		}
		certAuthority, err := client.GetCertAuthority(
			services.CertAuthID{Type: services.HostCA, DomainName: clusterName},
			a.exportPrivateKeys)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(certAuthority.GetTLSKeyPairs()) != 1 {
			return trace.BadParameter("expected one TLS key pair, got %v", len(certAuthority.GetTLSKeyPairs()))
		}
		keyPair := certAuthority.GetTLSKeyPairs()[0]
		if a.exportPrivateKeys {
			fmt.Println(string(keyPair.Key))
		}
		fmt.Println(string(keyPair.Cert))
		return nil
	}

	// if no --type flag is given, export all types
	if a.authType == "" {
		typesToExport = []services.CertAuthType{services.HostCA, services.UserCA}
	} else {
		authType := services.CertAuthType(a.authType)
		if err := authType.Check(); err != nil {
			return trace.Wrap(err)
		}
		typesToExport = []services.CertAuthType{authType}
	}
	localAuthName, err := client.GetDomainName()
	if err != nil {
		return trace.Wrap(err)
	}

	// fetch authorities via auth API (and only take local CAs, ignoring
	// trusted ones)
	var authorities []services.CertAuthority
	for _, at := range typesToExport {
		cas, err := client.GetCertAuthorities(at, a.exportPrivateKeys)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, ca := range cas {
			if ca.GetClusterName() == localAuthName {
				authorities = append(authorities, ca)
			}
		}
	}

	// print:
	for _, ca := range authorities {
		if a.exportPrivateKeys {
			for _, key := range ca.GetSigningKeys() {
				fingerprint, err := sshutils.PrivateKeyFingerprint(key)
				if err != nil {
					return trace.Wrap(err)
				}
				if a.exportAuthorityFingerprint != "" && fingerprint != a.exportAuthorityFingerprint {
					continue
				}
				os.Stdout.Write(key)
				fmt.Fprintf(os.Stdout, "\n")
			}
		} else {
			for _, keyBytes := range ca.GetCheckingKeys() {
				fingerprint, err := sshutils.AuthorizedKeyFingerprint(keyBytes)
				if err != nil {
					return trace.Wrap(err)
				}
				if a.exportAuthorityFingerprint != "" && fingerprint != a.exportAuthorityFingerprint {
					continue
				}

				// export certificates in the old 1.0 format where host and user
				// certificate authorities were exported in the known_hosts format.
				if a.compatVersion == "1.0" {
					castr, err := hostCAFormat(ca, keyBytes, client)
					if err != nil {
						return trace.Wrap(err)
					}

					fmt.Println(castr)
					continue
				}

				// export certificate authority in user or host ca format
				var castr string
				switch ca.GetType() {
				case services.UserCA:
					castr, err = userCAFormat(ca, keyBytes)
				case services.HostCA:
					castr, err = hostCAFormat(ca, keyBytes, client)
				default:
					return trace.BadParameter("unknown user type: %q", ca.GetType())
				}
				if err != nil {
					return trace.Wrap(err)
				}

				// print the export friendly string
				fmt.Println(castr)
			}
		}
	}
	return nil
}

// GenerateKeys generates a new keypair
func (a *AuthCommand) GenerateKeys() error {
	keygen, err := native.New(native.PrecomputeKeys(0))
	if err != nil {
		return trace.Wrap(err)
	}
	defer keygen.Close()
	privBytes, pubBytes, err := keygen.GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}
	err = ioutil.WriteFile(a.genPubPath, pubBytes, 0600)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ioutil.WriteFile(a.genPrivPath, privBytes, 0600)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("wrote public key to: %v and private key to: %v\n", a.genPubPath, a.genPrivPath)
	return nil
}

// GenerateAndSignKeys generates a new keypair and signs it for role
func (a *AuthCommand) GenerateAndSignKeys(clusterApi auth.ClientI) error {
	switch {
	case a.genUser != "" && a.genHost == "":
		return a.generateUserKeys(clusterApi)
	case a.genUser == "" && a.genHost != "":
		return a.generateHostKeys(clusterApi)
	default:
		return trace.BadParameter("--user or --host must be specified")
	}
}

// RotateCertAuthority starts or restarts certificate authority rotation process
func (a *AuthCommand) RotateCertAuthority(client auth.ClientI) error {
	req := auth.RotateRequest{
		Type:        services.CertAuthType(a.rotateType),
		GracePeriod: &a.rotateGracePeriod,
		TargetPhase: a.rotateTargetPhase,
	}
	if a.rotateManualMode {
		req.Mode = services.RotationModeManual
	} else {
		req.Mode = services.RotationModeAuto
	}
	if err := client.RotateCertAuthority(req); err != nil {
		return err
	}
	if a.rotateTargetPhase != "" {
		fmt.Printf("Updated rotation phase to %q. To check status use 'tctl status'\n", a.rotateTargetPhase)
	} else {
		fmt.Printf("Initiated certificate authority rotation. To check status use 'tctl status'\n")
	}

	return nil
}

func (a *AuthCommand) generateHostKeys(clusterApi auth.ClientI) error {
	// only format=openssh is supported
	if a.outputFormat != client.IdentityFormatOpenSSH {
		return trace.BadParameter("invalid --format flag %q, only %q is supported", a.outputFormat, client.IdentityFormatOpenSSH)
	}

	// split up comma separated list
	principals := strings.Split(a.genHost, ",")

	// generate a keypair
	key, err := client.NewKey()
	if err != nil {
		return trace.Wrap(err)
	}

	cn, err := clusterApi.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	clusterName := cn.GetClusterName()

	key.Cert, err = clusterApi.GenerateHostCert(key.Pub,
		"", "", principals,
		clusterName, teleport.Roles{teleport.RoleNode}, 0)
	if err != nil {
		return trace.Wrap(err)
	}

	// if no name was given, take the first name on the list of principals
	filePath := a.output
	if filePath == "" {
		filePath = principals[0]
	}

	err = client.MakeIdentityFile(filePath, key, a.outputFormat, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	if a.output != "" {
		fmt.Printf("\nThe certificate has been written to %s\n", a.output)
	}
	return nil
}

func (a *AuthCommand) generateUserKeys(clusterApi auth.ClientI) error {
	// parse compatibility parameter
	certificateFormat, err := utils.CheckCertificateFormatFlag(a.compatibility)
	if err != nil {
		return trace.Wrap(err)
	}

	// generate a keypair:
	key, err := client.NewKey()
	if err != nil {
		return trace.Wrap(err)
	}

	// sign it and produce a cert:
	key.Cert, key.TLSCert, err = clusterApi.GenerateUserCerts(key.Pub, a.genUser, a.genTTL, certificateFormat)
	if trace.IsNotFound(err) {
		return trace.BadParameter("server does not support exporting TLS identity, upgrade Teleport server components and try again")
	}
	if err != nil {
		return trace.Wrap(err)
	}

	var certAuthorities []services.CertAuthority
	if a.outputFormat == client.IdentityFormatFile {
		certAuthorities, err = clusterApi.GetCertAuthorities(services.HostCA, false)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// write the cert+private key to the output:
	err = client.MakeIdentityFile(a.output, key, a.outputFormat, certAuthorities)
	if err != nil {
		return trace.Wrap(err)
	}
	if a.output != "" {
		fmt.Printf("\nThe certificate has been written to %s\n", a.output)
	}
	return nil
}

// userCAFormat returns the certificate authority public key exported as a single
// line that can be placed in ~/.ssh/authorized_keys file. The format adheres to the
// man sshd (8) authorized_keys format, a space-separated list of: options, keytype,
// base64-encoded key, comment.
// For example:
//
//    cert-authority AAA... type=user&clustername=cluster-a
//
// URL encoding is used to pass the CA type and cluster name into the comment field.
func userCAFormat(ca services.CertAuthority, keyBytes []byte) (string, error) {
	return sshutils.MarshalAuthorizedKeysFormat(ca.GetClusterName(), keyBytes)
}

// hostCAFormat returns the certificate authority public key exported as a single line
// that can be placed in ~/.ssh/authorized_hosts. The format adheres to the man sshd (8)
// authorized_hosts format, a space-separated list of: marker, hosts, key, and comment.
// For example:
//
//    @cert-authority *.cluster-a ssh-rsa AAA... type=host
//
// URL encoding is used to pass the CA type and allowed logins into the comment field.
func hostCAFormat(ca services.CertAuthority, keyBytes []byte, client auth.ClientI) (string, error) {
	roles, err := services.FetchRoles(ca.GetRoles(), client, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	allowedLogins, _ := roles.CheckLoginDuration(defaults.MinCertDuration + time.Second)
	return sshutils.MarshalAuthorizedHostsFormat(ca.GetClusterName(), keyBytes, allowedLogins)
}
