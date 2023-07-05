// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/keygen"
	"github.com/gravitational/teleport/lib/auth/windows"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/db"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/defaults"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// AuthCommand implements `tctl auth` group of commands
type AuthCommand struct {
	config                     *service.Config
	authType                   string
	genPubPath                 string
	genPrivPath                string
	genUser                    string
	genHost                    string
	format                     string
	genTTL                     time.Duration
	exportAuthorityFingerprint string
	exportPrivateKeys          bool
	output                     string
	outputFormat               identityfile.Format
	compatVersion              string
	compatibility              string
	proxyAddr                  string
	leafCluster                string
	kubeCluster                string
	appName                    string
	dbService                  string
	dbName                     string
	dbUser                     string
	windowsUser                string
	windowsDomain              string
	windowsSID                 string
	signOverwrite              bool
	password                   string
	caType                     string

	rotateGracePeriod time.Duration
	rotateType        string
	rotateManualMode  bool
	rotateTargetPhase string

	authGenerate *kingpin.CmdClause
	authExport   *kingpin.CmdClause
	authSign     *kingpin.CmdClause
	authRotate   *kingpin.CmdClause
	authLS       *kingpin.CmdClause
	authCRL      *kingpin.CmdClause
	// testInsecureSkipVerify is used to skip TLS verification during tests
	// when connecting to the proxy ping address.
	testInsecureSkipVerify bool
}

// Initialize allows TokenCommand to plug itself into the CLI parser
func (a *AuthCommand) Initialize(app *kingpin.Application, config *service.Config) {
	a.config = config
	// operations with authorities
	auth := app.Command("auth", "Operations with user and host certificate authorities (CAs).").Hidden()
	a.authExport = auth.Command("export", "Export public cluster (CA) keys to stdout.")
	a.authExport.Flag("keys", "if set, will print private keys").BoolVar(&a.exportPrivateKeys)
	a.authExport.Flag("fingerprint", "filter authority by fingerprint").StringVar(&a.exportAuthorityFingerprint)
	a.authExport.Flag("compat", "export certificates compatible with specific version of Teleport").StringVar(&a.compatVersion)
	a.authExport.Flag("type",
		fmt.Sprintf("export certificate type (%v)", strings.Join(allowedCertificateTypes, ", "))).
		EnumVar(&a.authType, allowedCertificateTypes...)

	a.authGenerate = auth.Command("gen", "Generate a new SSH keypair.").Hidden()
	a.authGenerate.Flag("pub-key", "path to the public key").Required().StringVar(&a.genPubPath)
	a.authGenerate.Flag("priv-key", "path to the private key").Required().StringVar(&a.genPrivPath)

	a.authSign = auth.Command("sign", "Create an identity file(s) for a given user.")
	a.authSign.Flag("user", "Teleport user name").StringVar(&a.genUser)
	a.authSign.Flag("host", "Teleport host name").StringVar(&a.genHost)
	a.authSign.Flag("out", "Identity output").Short('o').Required().StringVar(&a.output)
	a.authSign.Flag("format", fmt.Sprintf("Identity format: %s. %s is the default.",
		identityfile.KnownFileFormats.String(), identityfile.DefaultFormat)).
		Default(string(identityfile.DefaultFormat)).
		StringVar((*string)(&a.outputFormat))
	a.authSign.Flag("ttl", "TTL (time to live) for the generated certificate.").
		Default(fmt.Sprintf("%v", apidefaults.CertDuration)).
		DurationVar(&a.genTTL)
	a.authSign.Flag("compat", "OpenSSH compatibility flag").StringVar(&a.compatibility)
	a.authSign.Flag("proxy", `Address of the Teleport proxy. When --format is set to "kubernetes", this address will be set as cluster address in the generated kubeconfig file`).StringVar(&a.proxyAddr)
	a.authSign.Flag("overwrite", "Whether to overwrite existing destination files. When not set, user will be prompted before overwriting any existing file.").BoolVar(&a.signOverwrite)
	// --kube-cluster was an unfortunately chosen flag name, before teleport
	// supported kubernetes_service and registered kubernetes clusters that are
	// not trusted teleport clusters.
	// It's kept as an alias for --leaf-cluster for backwards-compatibility,
	// but hidden.
	a.authSign.Flag("kube-cluster", `Leaf cluster to generate identity file for when --format is set to "kubernetes"`).Hidden().StringVar(&a.leafCluster)
	a.authSign.Flag("leaf-cluster", `Leaf cluster to generate identity file for when --format is set to "kubernetes"`).StringVar(&a.leafCluster)
	a.authSign.Flag("kube-cluster-name", `Kubernetes cluster to generate identity file for when --format is set to "kubernetes"`).StringVar(&a.kubeCluster)
	a.authSign.Flag("app-name", `Application to generate identity file for. Mutually exclusive with "--db-service".`).StringVar(&a.appName)
	a.authSign.Flag("db-service", `Database to generate identity file for. Mutually exclusive with "--app-name".`).StringVar(&a.dbService)
	a.authSign.Flag("db-user", `Database user placed on the identity file. Only used when "--db-service" is set.`).StringVar(&a.dbUser)
	a.authSign.Flag("db-name", `Database name placed on the identity file. Only used when "--db-service" is set.`).StringVar(&a.dbName)
	a.authSign.Flag("windows-user", `Window user placed on the identity file. Only used when --format is set to "windows"`).StringVar(&a.windowsUser)
	a.authSign.Flag("windows-domain", `Active Directory domain for which this cert is valid. Only used when --format is set to "windows"`).StringVar(&a.windowsDomain)
	a.authSign.Flag("windows-sid", `Optional Security Identifier to embed in the certificate. Only used when --format is set to "windows"`).StringVar(&a.windowsSID)

	a.authRotate = auth.Command("rotate", "Rotate certificate authorities in the cluster.")
	a.authRotate.Flag("grace-period", "Grace period keeps previous certificate authorities signatures valid, if set to 0 will force users to re-login and nodes to re-register.").
		Default(fmt.Sprintf("%v", defaults.RotationGracePeriod)).
		DurationVar(&a.rotateGracePeriod)
	a.authRotate.Flag("manual", "Activate manual rotation , set rotation phases manually").BoolVar(&a.rotateManualMode)
	a.authRotate.Flag("type", fmt.Sprintf("Certificate authority to rotate, one of: %s", strings.Join(getCertAuthTypes(), ", "))).EnumVar(&a.rotateType, getCertAuthTypes()...)
	a.authRotate.Flag("phase", fmt.Sprintf("Target rotation phase to set, used in manual rotation, one of: %v", strings.Join(types.RotatePhases, ", "))).StringVar(&a.rotateTargetPhase)

	a.authLS = auth.Command("ls", "List connected auth servers.")
	a.authLS.Flag("format", "Output format: 'yaml', 'json' or 'text'").Default(teleport.YAML).StringVar(&a.format)

	a.authCRL = auth.Command("crl", "Export empty certificate revocation list (CRL) for certificate authorities.")
	a.authCRL.Flag("type", "certificate authority type").EnumVar(&a.caType, allowedCRLCertificateTypes...)
}

// TryRun takes the CLI command as an argument (like "auth gen") and executes it
// or returns match=false if 'cmd' does not belong to it
func (a *AuthCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case a.authGenerate.FullCommand():
		err = a.GenerateKeys(ctx)
	case a.authExport.FullCommand():
		err = a.ExportAuthorities(ctx, client)
	case a.authSign.FullCommand():
		err = a.GenerateAndSignKeys(ctx, client)
	case a.authRotate.FullCommand():
		err = a.RotateCertAuthority(ctx, client)
	case a.authLS.FullCommand():
		err = a.ListAuthServers(ctx, client)
	case a.authCRL.FullCommand():
		err = a.GenerateCRLForCA(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

var allowedCertificateTypes = []string{
	"user",
	"host",
	"tls-host",
	"tls-user",
	"tls-user-der",
	"windows",
	"db",
	"db-der",
	"openssh",
	"saml-idp",
}

// allowedCRLCertificateTypes list of certificate authorities types that can
// have a CRL exported.
var allowedCRLCertificateTypes = []string{
	string(types.HostCA),
	string(types.DatabaseCA),
	string(types.UserCA),
}

// ExportAuthorities outputs the list of authorities in OpenSSH compatible formats
// If --type flag is given, only prints keys for CAs of this type, otherwise
// prints all keys
func (a *AuthCommand) ExportAuthorities(ctx context.Context, clt auth.ClientI) error {
	exportFunc := client.ExportAuthorities
	if a.exportPrivateKeys {
		exportFunc = client.ExportAuthoritiesSecrets
	}

	authorities, err := exportFunc(
		ctx,
		clt,
		client.ExportAuthoritiesRequest{
			AuthType:                   a.authType,
			ExportAuthorityFingerprint: a.exportAuthorityFingerprint,
			UseCompatVersion:           a.compatVersion == "1.0",
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println(authorities)

	return nil
}

// GenerateKeys generates a new keypair
func (a *AuthCommand) GenerateKeys(ctx context.Context) error {
	keygen := keygen.New(ctx)
	defer keygen.Close()
	privBytes, pubBytes, err := keygen.GenerateKeyPair()
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.WriteFile(a.genPubPath, pubBytes, 0o600)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.WriteFile(a.genPrivPath, privBytes, 0o600)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("wrote public key to: %v and private key to: %v\n", a.genPubPath, a.genPrivPath)
	return nil
}

// GenerateAndSignKeys generates a new keypair and signs it for role
func (a *AuthCommand) GenerateAndSignKeys(ctx context.Context, clusterAPI auth.ClientI) error {
	switch a.outputFormat {
	case identityfile.FormatDatabase, identityfile.FormatMongo, identityfile.FormatCockroach,
		identityfile.FormatRedis, identityfile.FormatElasticsearch:
		return a.generateDatabaseKeys(ctx, clusterAPI)
	case identityfile.FormatCassandra, identityfile.FormatScylla:
		jskPass, err := utils.CryptoRandomHex(32)
		if err != nil {
			return trace.Wrap(err)
		}
		a.password = jskPass
		return a.generateDatabaseKeys(ctx, clusterAPI)
	case identityfile.FormatSnowflake:
		return a.generateSnowflakeKey(ctx, clusterAPI)
	case identityfile.FormatWindows:
		return a.generateWindowsCert(ctx, clusterAPI)
	case identityfile.FormatOracle:
		oracleWalletPass, err := utils.CryptoRandomHex(32)
		if err != nil {
			return trace.Wrap(err)
		}
		a.password = oracleWalletPass
		return a.generateDBOracleCert(ctx, clusterAPI)

	}
	switch {
	case a.genUser != "" && a.genHost == "":
		return a.generateUserKeys(ctx, clusterAPI)
	case a.genUser == "" && a.genHost != "":
		return a.generateHostKeys(ctx, clusterAPI)
	default:
		return trace.BadParameter("--user or --host must be specified")
	}
}

func (a *AuthCommand) generateWindowsCert(ctx context.Context, clusterAPI auth.ClientI) error {
	var missingFlags []string
	if len(a.windowsUser) == 0 {
		missingFlags = append(missingFlags, "--windows-user")
	}
	if len(a.windowsDomain) == 0 {
		missingFlags = append(missingFlags, "--windows-domain")
	}
	if len(missingFlags) > 0 {
		return trace.BadParameter("the following flags are missing: %v",
			strings.Join(missingFlags, ", "))
	}

	cn, err := clusterAPI.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	certDER, _, err := windows.GenerateWindowsDesktopCredentials(ctx, &windows.GenerateCredentialsRequest{
		Username:           a.windowsUser,
		Domain:             a.windowsDomain,
		ActiveDirectorySID: a.windowsSID,
		TTL:                a.genTTL,
		ClusterName:        cn.GetClusterName(),
		LDAPConfig:         windows.LDAPConfig{Domain: a.windowsDomain},
		AuthClient:         clusterAPI,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = identityfile.Write(ctx, identityfile.WriteConfig{
		OutputPath: a.output,
		Key: &client.Key{
			// the godocs say the map key is the desktop server name,
			// but in this case we're just generating a cert that's not
			// specific to a particular desktop
			WindowsDesktopCerts: map[string][]byte{a.windowsUser: certDER},
		},
		Format:               a.outputFormat,
		OverwriteDestination: a.signOverwrite,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// generateSnowflakeKey exports DatabaseCA public key in the format required by Snowflake
// Ref: https://docs.snowflake.com/en/user-guide/key-pair-auth.html#step-2-generate-a-public-key
func (a *AuthCommand) generateSnowflakeKey(ctx context.Context, clusterAPI auth.ClientI) error {
	key, err := client.GenerateRSAKey()
	if err != nil {
		return trace.Wrap(err)
	}

	cn, err := clusterAPI.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	certAuthID := types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: cn.GetClusterName(),
	}
	databaseCA, err := clusterAPI.GetCertAuthority(ctx, certAuthID, false)
	if err != nil {
		return trace.Wrap(err)
	}

	key.TrustedCerts = []auth.TrustedCerts{{TLSCertificates: services.GetTLSCerts(databaseCA)}}

	filesWritten, err := identityfile.Write(ctx, identityfile.WriteConfig{
		OutputPath:           a.output,
		Key:                  key,
		Format:               a.outputFormat,
		OverwriteDestination: a.signOverwrite,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(
		writeHelperMessageDBmTLS(os.Stdout, filesWritten, "", a.outputFormat, ""),
	)
}

// RotateCertAuthority starts or restarts certificate authority rotation process
func (a *AuthCommand) RotateCertAuthority(ctx context.Context, client auth.ClientI) error {
	if a.rotateType == "" {
		return trace.BadParameter("required flag --type not provided; previous versions defaulted to --type=all which is deprecated and will be removed in a future version")
	}
	if a.rotateType == string(types.CertAuthTypeAll) {
		fmt.Println("\033[0;31mNOTICE:\033[0m --type=all will be deprecated in a future version")
	}

	req := auth.RotateRequest{
		Type:        types.CertAuthType(a.rotateType),
		GracePeriod: &a.rotateGracePeriod,
		TargetPhase: a.rotateTargetPhase,
	}
	if a.rotateManualMode {
		req.Mode = types.RotationModeManual
	} else {
		req.Mode = types.RotationModeAuto
	}
	if err := client.RotateCertAuthority(ctx, req); err != nil {
		return err
	}
	if a.rotateTargetPhase != "" {
		fmt.Printf("Updated rotation phase to %q. To check status use 'tctl status'\n", a.rotateTargetPhase)
	} else {
		fmt.Printf("Initiated certificate authority rotation. To check status use 'tctl status'\n")
	}

	return nil
}

// ListAuthServers prints a list of connected auth servers
func (a *AuthCommand) ListAuthServers(ctx context.Context, clusterAPI auth.ClientI) error {
	servers, err := clusterAPI.GetAuthServers()
	if err != nil {
		return trace.Wrap(err)
	}

	sc := &serverCollection{servers, false}

	switch a.format {
	case teleport.Text:
		return sc.writeText(os.Stdout)
	case teleport.YAML:
		return writeYAML(sc, os.Stdout)
	case teleport.JSON:
		return writeJSON(sc, os.Stdout)
	}

	return nil
}

// GenerateCRLForCA generates a certificate revocation list for a certificate
// authority.
func (a *AuthCommand) GenerateCRLForCA(ctx context.Context, clusterAPI auth.ClientI) error {
	certType := types.CertAuthType(a.caType)
	if err := certType.Check(); err != nil {
		return trace.Wrap(err)
	}

	crl, err := clusterAPI.GenerateCertAuthorityCRL(ctx, certType)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println(string(crl))
	return nil
}

func (a *AuthCommand) generateHostKeys(ctx context.Context, clusterAPI auth.ClientI) error {
	// only format=openssh is supported
	if a.outputFormat != identityfile.FormatOpenSSH {
		return trace.BadParameter("invalid --format flag %q, only %q is supported", a.outputFormat, identityfile.FormatOpenSSH)
	}

	// split up comma separated list
	principals := strings.Split(a.genHost, ",")

	// generate a keypair
	key, err := client.GenerateRSAKey()
	if err != nil {
		return trace.Wrap(err)
	}

	cn, err := clusterAPI.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	clusterName := cn.GetClusterName()

	key.Cert, err = clusterAPI.GenerateHostCert(ctx, key.MarshalSSHPublicKey(),
		"", "", principals,
		clusterName, types.RoleNode, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	hostCAs, err := clusterAPI.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	key.TrustedCerts = auth.AuthoritiesToTrustedCerts(hostCAs)

	// if no name was given, take the first name on the list of principals
	filePath := a.output
	if filePath == "" {
		filePath = principals[0]
	}

	filesWritten, err := identityfile.Write(ctx, identityfile.WriteConfig{
		OutputPath:           filePath,
		Key:                  key,
		Format:               a.outputFormat,
		OverwriteDestination: a.signOverwrite,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("\nThe credentials have been written to %s\n", strings.Join(filesWritten, ", "))
	return nil
}

// generateDatabaseKeys generates a new unsigned key and signs it with Teleport
// CA for database access.
func (a *AuthCommand) generateDatabaseKeys(ctx context.Context, clusterAPI auth.ClientI) error {
	key, err := client.GenerateRSAKey()
	if err != nil {
		return trace.Wrap(err)
	}
	return a.generateDatabaseKeysForKey(ctx, clusterAPI, key)
}

// generateDatabaseKeysForKey signs the provided unsigned key with Teleport CA
// for database access.
func (a *AuthCommand) generateDatabaseKeysForKey(ctx context.Context, clusterAPI auth.ClientI, key *client.Key) error {
	principals := strings.Split(a.genHost, ",")

	dbCertReq := db.GenerateDatabaseCertificatesRequest{
		ClusterAPI:         clusterAPI,
		Principals:         principals,
		OutputFormat:       a.outputFormat,
		OutputCanOverwrite: a.signOverwrite,
		OutputLocation:     a.output,
		TTL:                a.genTTL,
		Key:                key,
		Password:           a.password,
	}
	filesWritten, err := db.GenerateDatabaseCertificates(ctx, dbCertReq)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(writeHelperMessageDBmTLS(os.Stdout, filesWritten, a.output, a.outputFormat, a.password))
}

var mapIdentityFileFormatHelperTemplate = map[identityfile.Format]*template.Template{
	identityfile.FormatDatabase:      dbAuthSignTpl,
	identityfile.FormatMongo:         mongoAuthSignTpl,
	identityfile.FormatCockroach:     cockroachAuthSignTpl,
	identityfile.FormatRedis:         redisAuthSignTpl,
	identityfile.FormatSnowflake:     snowflakeAuthSignTpl,
	identityfile.FormatElasticsearch: elasticsearchAuthSignTpl,
	identityfile.FormatCassandra:     cassandraAuthSignTpl,
	identityfile.FormatScylla:        scyllaAuthSignTpl,
	identityfile.FormatOracle:        oracleAuthSignTpl,
}

func writeHelperMessageDBmTLS(writer io.Writer, filesWritten []string, output string, outputFormat identityfile.Format, password string) error {
	if writer == nil {
		return nil
	}

	tpl, found := mapIdentityFileFormatHelperTemplate[outputFormat]
	if !found {
		// This format doesn't have a recommended configuration.
		// Consider adding one to ease the installation for the end-user
		return nil
	}
	tplVars := map[string]interface{}{
		"files":    strings.Join(filesWritten, ", "),
		"password": password,
		"output":   output,
	}
	if outputFormat == defaults.ProtocolOracle {
		tplVars["manualOrapkiFlow"] = len(filesWritten) != 1
		tplVars["walletDir"] = filepath.Dir(output)
	}

	return trace.Wrap(tpl.Execute(writer, tplVars))
}

var (
	// dbAuthSignTpl is printed when user generates credentials for a self-hosted database.
	dbAuthSignTpl = template.Must(template.New("").Parse(`Database credentials have been written to {{.files}}.

To enable mutual TLS on your PostgreSQL server, add the following to its postgresql.conf configuration file:

ssl = on
ssl_cert_file = '/path/to/{{.output}}.crt'
ssl_key_file = '/path/to/{{.output}}.key'
ssl_ca_file = '/path/to/{{.output}}.cas'

To enable mutual TLS on your MySQL server, add the following to its mysql.cnf configuration file:

[mysqld]
require_secure_transport=ON
ssl-cert=/path/to/{{.output}}.crt
ssl-key=/path/to/{{.output}}.key
ssl-ca=/path/to/{{.output}}.cas
`))
	// mongoAuthSignTpl is printed when user generates credentials for a MongoDB database.
	mongoAuthSignTpl = template.Must(template.New("").Parse(`Database credentials have been written to {{.files}}.

To enable mutual TLS on your MongoDB server, add the following to its
mongod.yaml configuration file:

net:
  tls:
    mode: requireTLS
    certificateKeyFile: /path/to/{{.output}}.crt
    CAFile: /path/to/{{.output}}.cas
`))
	cockroachAuthSignTpl = template.Must(template.New("").Parse(`Database credentials have been written to {{.files}}.

To enable mutual TLS on your CockroachDB server, point it to the certs
directory using --certs-dir flag:

cockroach start \
  --certs-dir={{.output}} \
  # other flags...
`))

	redisAuthSignTpl = template.Must(template.New("").Parse(`Database credentials have been written to {{.files}}.

To enable mutual TLS on your Redis server, add the following to your redis.conf:

tls-ca-cert-file /path/to/{{.output}}.cas
tls-cert-file /path/to/{{.output}}.crt
tls-key-file /path/to/{{.output}}.key
tls-protocols "TLSv1.2 TLSv1.3"
`))

	snowflakeAuthSignTpl = template.Must(template.New("").Parse(`Database credentials have been written to {{.files}}.

Please add the generated key to the Snowflake users as described here:
https://docs.snowflake.com/en/user-guide/key-pair-auth.html#step-4-assign-the-public-key-to-a-snowflake-user
`))

	elasticsearchAuthSignTpl = template.Must(template.New("").Parse(`Database credentials have been written to {{.files}}.

To enable mutual TLS on your Elasticsearch server, add the following to your elasticsearch.yml:

xpack.security.http.ssl:
  certificate_authorities: /path/to/{{.output}}.cas
  certificate: /path/to/{{.output}}.crt
  key: /path/to/{{.output}}.key
  enabled: true
  client_authentication: required
  verification_mode: certificate

xpack.security.authc.realms.pki.pki1:
  order: 1
  enabled: true
  certificate_authorities: /path/to/{{.output}}.cas

For more information on configuring security settings in Elasticsearch, see:
https://www.elastic.co/guide/en/elasticsearch/reference/current/security-settings.html
`))

	cassandraAuthSignTpl = template.Must(template.New("").Parse(`Database credentials have been written to {{.files}}.
To enable mutual TLS on your Cassandra server, add the following to your
cassandra.yaml configuration file:
client_encryption_options:
   enabled: true
   optional: false
   keystore: /path/to/{{.output}}.keystore
   keystore_password: "{{.password}}"
   require_client_auth: true
   truststore: /path/to/{{.output}}.truststore
   truststore_password: "{{.password}}"
   protocol: TLS
   algorithm: SunX509
   store_type: JKS
   cipher_suites: [TLS_RSA_WITH_AES_256_CBC_SHA]
`))

	oracleAuthSignTpl = template.Must(template.New("").Parse(`
{{if .manualOrapkiFlow}}
Orapki binary was not found. Please create oracle wallet file manually by running the following commands on the Oracle server:

orapki wallet create -wallet {{.walletDir}} -auto_login_only
orapki wallet import_pkcs12 -wallet {{.walletDir}} -auto_login_only -pkcs12file {{.output}}.p12 -pkcs12pwd {{.password}}
orapki wallet add -wallet {{.walletDir}} -trusted_cert -auto_login_only -cert {{.output}}.crt
{{else}}
Oracle wallet stored in {{.output}} directory created with Oracle Orapki.

{{end}}
To enable mutual TLS on your Oracle server, add the following settings to Oracle sqlnet.ora configuration file:

WALLET_LOCATION = (SOURCE = (METHOD = FILE)(METHOD_DATA = (DIRECTORY = /path/to/oracleWalletDir)))
SSL_CLIENT_AUTHENTICATION = TRUE
SQLNET.AUTHENTICATION_SERVICES = (TCPS)


To enable mutual TLS on your Oracle server, add the following TCPS entries to listener.ora configuration file:

LISTENER =
  (DESCRIPTION_LIST =
    (DESCRIPTION =
      (ADDRESS = (PROTOCOL = TCPS)(HOST = 0.0.0.0)(PORT = 2484))
    )
  )

WALLET_LOCATION = (SOURCE = (METHOD = FILE)(METHOD_DATA = (DIRECTORY = /path/to/oracleWalletDir)))
SSL_CLIENT_AUTHENTICATION = TRUE
`))

	scyllaAuthSignTpl = template.Must(template.New("").Parse(`Database credentials have been written to {{.files}}.

To enable mutual TLS on your Scylla server, add the following to your
scylla.yaml configuration file:

client_encryption_options:
   enabled: true
   certificate: /path/to/{{.output}}.crt
   keyfile: /path/to/{{.output}}.key
   truststore:  /path/to/{{.output}}.cas
   require_client_auth: True
`))
)

func (a *AuthCommand) generateUserKeys(ctx context.Context, clusterAPI auth.ClientI) error {
	// Validate --proxy flag.
	if err := a.checkProxyAddr(ctx, clusterAPI); err != nil {
		return trace.Wrap(err)
	}
	// parse compatibility parameter
	certificateFormat, err := utils.CheckCertificateFormatFlag(a.compatibility)
	if err != nil {
		return trace.Wrap(err)
	}

	// generate a keypair:
	key, err := client.GenerateRSAKey()
	if err != nil {
		return trace.Wrap(err)
	}

	if a.leafCluster != "" {
		if err := a.checkLeafCluster(clusterAPI); err != nil {
			return trace.Wrap(err)
		}
	} else {
		cn, err := clusterAPI.GetClusterName()
		if err != nil {
			return trace.Wrap(err)
		}
		a.leafCluster = cn.GetClusterName()
	}
	key.ClusterName = a.leafCluster

	if err := a.checkKubeCluster(ctx, clusterAPI); err != nil {
		return trace.Wrap(err)
	}

	var (
		routeToApp      proto.RouteToApp
		routeToDatabase proto.RouteToDatabase
		certUsage       proto.UserCertsRequest_CertUsage
	)

	// `appName` and `db` are mutually exclusive.
	if a.appName != "" && a.dbService != "" {
		return trace.BadParameter("only --app-name or --db-service can be set, not both")
	}

	switch {
	case a.appName != "":
		server, err := getApplicationServer(ctx, clusterAPI, a.appName)
		if err != nil {
			return trace.Wrap(err)
		}

		appSession, err := clusterAPI.CreateAppSession(ctx, types.CreateAppSessionRequest{
			Username:    a.genUser,
			PublicAddr:  server.GetApp().GetPublicAddr(),
			ClusterName: a.leafCluster,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		routeToApp = proto.RouteToApp{
			Name:        a.appName,
			PublicAddr:  server.GetApp().GetPublicAddr(),
			ClusterName: a.leafCluster,
			SessionID:   appSession.GetName(),
		}
		certUsage = proto.UserCertsRequest_App
	case a.dbService != "":
		server, err := getDatabaseServer(ctx, clusterAPI, a.dbService)
		if err != nil {
			return trace.Wrap(err)
		}

		routeToDatabase = proto.RouteToDatabase{
			ServiceName: a.dbService,
			Protocol:    server.GetDatabase().GetProtocol(),
			Database:    a.dbName,
			Username:    a.dbUser,
		}
		certUsage = proto.UserCertsRequest_Database
	}

	reqExpiry := time.Now().UTC().Add(a.genTTL)
	// Request signed certs from `auth` server.
	certs, err := clusterAPI.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey:         key.MarshalSSHPublicKey(),
		Username:          a.genUser,
		Expires:           reqExpiry,
		Format:            certificateFormat,
		RouteToCluster:    a.leafCluster,
		KubernetesCluster: a.kubeCluster,
		RouteToApp:        routeToApp,
		Usage:             certUsage,
		RouteToDatabase:   routeToDatabase,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	key.Cert = certs.SSH
	key.TLSCert = certs.TLS

	hostCAs, err := clusterAPI.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	key.TrustedCerts = auth.AuthoritiesToTrustedCerts(hostCAs)

	// Is TLS routing enabled?
	proxyListenerMode := types.ProxyListenerMode_Separate
	if a.config != nil && a.config.Auth.NetworkingConfig != nil {
		proxyListenerMode = a.config.Auth.NetworkingConfig.GetProxyListenerMode()
	}
	if networkConfig, err := clusterAPI.GetClusterNetworkingConfig(ctx); err == nil {
		proxyListenerMode = networkConfig.GetProxyListenerMode()
	}

	// If we're in multiplexed mode get SNI name for kube from single multiplexed proxy addr
	kubeTLSServerName := ""
	if proxyListenerMode == types.ProxyListenerMode_Multiplex {
		log.Debug("Using Proxy SNI for kube TLS server name")
		u, err := parseURL(a.proxyAddr)
		if err != nil {
			return trace.Wrap(err)
		}
		// extract host part if present
		split := strings.Split(u.Host, ":")
		kubeTLSServerName = client.GetKubeTLSServerName(split[0])
	}

	expires, err := key.TeleportTLSCertValidBefore()
	if err != nil {
		log.WithError(err).Warn("Failed to check TTL validity")
		// err swallowed on purpose
	} else if reqExpiry.Sub(expires) > time.Minute {
		maxAllowedTTL := time.Until(expires).Round(time.Second)
		return trace.BadParameter(`The credential was not issued because the requested TTL of %s exceeded the maximum allowed value of %s. To successfully request a credential, please reduce the requested TTL.`,
			a.genTTL,
			maxAllowedTTL)
	}

	// write the cert+private key to the output:
	filesWritten, err := identityfile.Write(ctx, identityfile.WriteConfig{
		OutputPath:           a.output,
		Key:                  key,
		Format:               a.outputFormat,
		KubeProxyAddr:        a.proxyAddr,
		KubeClusterName:      a.kubeCluster,
		KubeTLSServerName:    kubeTLSServerName,
		OverwriteDestination: a.signOverwrite,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// Print a tip guiding people towards Machine ID. We use stderr here in case
	// someone is programatically parsing stdout.
	_, _ = fmt.Fprintln(
		os.Stderr,
		"\nGenerating credentials to allow a machine access to Teleport? We recommend Teleport's Machine ID! Find out more at https://goteleport.com/r/machineid-tip",
	)
	fmt.Printf("The credentials have been written to %s\n", strings.Join(filesWritten, ", "))

	return nil
}

func (a *AuthCommand) checkLeafCluster(clusterAPI auth.ClientI) error {
	if a.outputFormat != identityfile.FormatKubernetes && a.leafCluster != "" {
		// User set --cluster but it's not actually used for the chosen --format.
		// Print a warning but continue.
		fmt.Printf("Note: --cluster is only used with --format=%q, ignoring for --format=%q\n", identityfile.FormatKubernetes, a.outputFormat)
	}

	if a.outputFormat != identityfile.FormatKubernetes {
		return nil
	}

	clusters, err := clusterAPI.GetRemoteClusters()
	if err != nil {
		return trace.WrapWithMessage(err, "couldn't load leaf clusters")
	}

	for _, cluster := range clusters {
		if cluster.GetMetadata().Name == a.leafCluster {
			return nil
		}
	}

	return trace.BadParameter("couldn't find leaf cluster named %q", a.leafCluster)
}

func (a *AuthCommand) checkKubeCluster(ctx context.Context, clusterAPI auth.ClientI) error {
	if a.outputFormat != identityfile.FormatKubernetes && a.kubeCluster != "" {
		// User set --kube-cluster-name but it's not actually used for the chosen --format.
		// Print a warning but continue.
		fmt.Printf("Note: --kube-cluster-name is only used with --format=%q, ignoring for --format=%q\n", identityfile.FormatKubernetes, a.outputFormat)
	}
	if a.outputFormat != identityfile.FormatKubernetes {
		return nil
	}

	localCluster, err := clusterAPI.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	if localCluster.GetClusterName() != a.leafCluster {
		// Skip validation on remote clusters, since we don't know their
		// registered kube clusters.
		return nil
	}

	a.kubeCluster, err = kubeutils.CheckOrSetKubeCluster(ctx, clusterAPI, a.kubeCluster, a.leafCluster)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (a *AuthCommand) checkProxyAddr(ctx context.Context, clusterAPI auth.ClientI) error {
	if a.outputFormat != identityfile.FormatKubernetes && a.proxyAddr != "" {
		// User set --proxy but it's not actually used for the chosen --format.
		// Print a warning but continue.
		fmt.Printf("Note: --proxy is only used with --format=%q, ignoring for --format=%q\n", identityfile.FormatKubernetes, a.outputFormat)
		return nil
	}
	if a.outputFormat != identityfile.FormatKubernetes {
		return nil
	}
	if a.proxyAddr != "" {
		// User set --proxy. Validate it and set its scheme to https in case it was omitted.
		u, err := parseURL(a.proxyAddr)
		if err != nil {
			return trace.WrapWithMessage(err, "specified --proxy URL is invalid")
		}
		switch u.Scheme {
		case "":
			u.Scheme = "https"
			a.proxyAddr = u.String()
			return nil
		case "http", "https":
			return nil
		default:
			return trace.BadParameter("expected --proxy URL with http or https scheme")
		}
	}

	// User didn't specify --proxy for kubeconfig. Let's try to guess it.
	//
	// Is the auth server also a proxy?
	if a.config.Proxy.Kube.Enabled {
		var err error
		if a.config.Auth.NetworkingConfig != nil &&
			a.config.Auth.NetworkingConfig.GetProxyListenerMode() == types.ProxyListenerMode_Multiplex {
			a.proxyAddr, err = a.config.Proxy.WebPublicAddr()
			return trace.Wrap(err)
		}
		a.proxyAddr, err = a.config.Proxy.KubeAddr()
		return trace.Wrap(err)
	}
	netConfig, err := clusterAPI.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.WrapWithMessage(err, "couldn't load cluster network configuration, try setting --proxy manually")
	}
	// Fetch proxies known to auth server and try to find a public address.
	proxies, err := clusterAPI.GetProxies()
	if err != nil {
		return trace.WrapWithMessage(err, "couldn't load registered proxies, try setting --proxy manually")
	}
	for _, p := range proxies {
		addr := p.GetPublicAddr()
		if addr == "" {
			continue
		}
		// if the proxy is multiplexing, the public address is the web proxy address.
		if netConfig.GetProxyListenerMode() == types.ProxyListenerMode_Multiplex {
			u := url.URL{
				Scheme: "https",
				Host:   addr,
			}
			a.proxyAddr = u.String()
			return nil
		}

		_, err := utils.ParseAddr(addr)
		if err != nil {
			log.Warningf("Invalid public address on the proxy %q: %q: %v.", p.GetName(), addr, err)
			continue
		}

		ping, err := webclient.Ping(
			&webclient.Config{
				Context:   ctx,
				ProxyAddr: addr,
				Timeout:   5 * time.Second,
				Insecure:  a.testInsecureSkipVerify,
			},
		)
		if err != nil {
			log.Warningf("Unable to ping proxy public address on the proxy %q: %q: %v.", p.GetName(), addr, err)
			continue
		}

		if !ping.Proxy.Kube.Enabled || ping.Proxy.Kube.PublicAddr == "" {
			continue
		}

		u := url.URL{
			Scheme: "https",
			Host:   ping.Proxy.Kube.PublicAddr,
		}
		a.proxyAddr = u.String()
		return nil
	}

	return trace.BadParameter("couldn't find registered public proxies, specify --proxy when using --format=%q", identityfile.FormatKubernetes)
}

func (a *AuthCommand) generateDBOracleCert(ctx context.Context, api auth.ClientI) error {
	key, err := client.GenerateRSAKey()
	if err != nil {
		return trace.Wrap(err)
	}
	return a.generateDatabaseKeysForKey(ctx, api, key)
}

func parseURL(rawurl string) (*url.URL, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// If no scheme is provided url.Parse fails the parsing and considers the host
	// as scheme, leaving the host empty.
	if u.Host == "" {
		return &url.URL{
			Host: rawurl,
		}, nil
	}

	return u, nil
}

func getApplicationServer(ctx context.Context, clusterAPI auth.ClientI, appName string) (types.AppServer, error) {
	servers, err := clusterAPI.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, s := range servers {
		if s.GetName() == appName {
			return s, nil
		}
	}
	return nil, trace.NotFound("app %q not found", appName)
}

// getDatabaseServer fetches a single `DatabaseServer` by name using the
// provided `auth.ClientI`.
func getDatabaseServer(ctx context.Context, clientAPI auth.ClientI, dbName string) (types.DatabaseServer, error) {
	servers, err := clientAPI.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, server := range servers {
		if server.GetName() == dbName {
			return server, nil
		}
	}

	return nil, trace.NotFound("database %q not found", dbName)
}

func getCertAuthTypes() []string {
	t := make([]string, 0, len(types.CertAuthTypes)+1)
	for _, at := range types.CertAuthTypes {
		t = append(t, string(at))
	}
	t = append(t, string(types.CertAuthTypeAll))
	return t
}
