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
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/keygen"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/db"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/defaults"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
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
	signOverwrite              bool
	jksPassword                string

	rotateGracePeriod time.Duration
	rotateType        string
	rotateManualMode  bool
	rotateTargetPhase string

	authGenerate *kingpin.CmdClause
	authExport   *kingpin.CmdClause
	authSign     *kingpin.CmdClause
	authRotate   *kingpin.CmdClause
	authLS       *kingpin.CmdClause
}

// Initialize allows TokenCommand to plug itself into the CLI parser
func (a *AuthCommand) Initialize(app *kingpin.Application, config *service.Config) {
	a.config = config

	// operations with authorities
	auth := app.Command("auth", "Operations with user and host certificate authorities (CAs)").Hidden()
	a.authExport = auth.Command("export", "Export public cluster (CA) keys to stdout.")
	a.authExport.Flag("keys", "if set, will print private keys").BoolVar(&a.exportPrivateKeys)
	a.authExport.Flag("fingerprint", "filter authority by fingerprint").StringVar(&a.exportAuthorityFingerprint)
	a.authExport.Flag("compat", "export certificates compatible with specific version of Teleport").StringVar(&a.compatVersion)
	a.authExport.Flag("type", "export certificate type").EnumVar(&a.authType, allowedCertificateTypes...)

	a.authGenerate = auth.Command("gen", "Generate a new SSH keypair").Hidden()
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
	a.authSign.Flag("ttl", "TTL (time to live) for the generated certificate").
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

	a.authRotate = auth.Command("rotate", "Rotate certificate authorities in the cluster.")
	a.authRotate.Flag("grace-period", "Grace period keeps previous certificate authorities signatures valid, if set to 0 will force users to re-login and nodes to re-register.").
		Default(fmt.Sprintf("%v", defaults.RotationGracePeriod)).
		DurationVar(&a.rotateGracePeriod)
	a.authRotate.Flag("manual", "Activate manual rotation , set rotation phases manually").BoolVar(&a.rotateManualMode)
	a.authRotate.Flag("type", "Certificate authority to rotate, rotates host, user and database CA by default").StringVar(&a.rotateType)
	a.authRotate.Flag("phase", fmt.Sprintf("Target rotation phase to set, used in manual rotation, one of: %v", strings.Join(types.RotatePhases, ", "))).StringVar(&a.rotateTargetPhase)

	a.authLS = auth.Command("ls", "List connected auth servers.")
	a.authLS.Flag("format", "Output format: 'yaml', 'json' or 'text'").Default(teleport.YAML).StringVar(&a.format)
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
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

var allowedCertificateTypes = []string{"user", "host", "tls-host", "tls-user", "tls-user-der", "windows", "db"}

// ExportAuthorities outputs the list of authorities in OpenSSH compatible formats
// If --type flag is given, only prints keys for CAs of this type, otherwise
// prints all keys
func (a *AuthCommand) ExportAuthorities(ctx context.Context, client auth.ClientI) error {
	var typesToExport []types.CertAuthType

	// this means to export TLS authority
	switch a.authType {
	// "tls" is supported for backwards compatibility.
	// "tls-host" and "tls-user" were added later to allow export of the user
	// TLS CA.
	case "tls", "tls-host":
		return a.exportTLSAuthority(ctx, client, types.HostCA, false)
	case "tls-user":
		return a.exportTLSAuthority(ctx, client, types.UserCA, false)
	case "db":
		return a.exportTLSAuthority(ctx, client, types.DatabaseCA, false)
	case "tls-user-der", "windows":
		return a.exportTLSAuthority(ctx, client, types.UserCA, true)
	}

	// if no --type flag is given, export HostCA and UserCA.
	if a.authType == "" {
		typesToExport = []types.CertAuthType{types.HostCA, types.UserCA}
	} else {
		authType := types.CertAuthType(a.authType)
		if err := authType.Check(); err != nil {
			return trace.Wrap(err)
		}
		typesToExport = []types.CertAuthType{authType}
	}
	localAuthName, err := client.GetDomainName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// fetch authorities via auth API (and only take local CAs, ignoring
	// trusted ones)
	var authorities []types.CertAuthority
	for _, at := range typesToExport {
		cas, err := client.GetCertAuthorities(ctx, at, a.exportPrivateKeys)
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
			for _, key := range ca.GetActiveKeys().SSH {
				fingerprint, err := sshutils.PrivateKeyFingerprint(key.PrivateKey)
				if err != nil {
					return trace.Wrap(err)
				}
				if a.exportAuthorityFingerprint != "" && fingerprint != a.exportAuthorityFingerprint {
					continue
				}
				os.Stdout.Write(key.PrivateKey)
				fmt.Fprintf(os.Stdout, "\n")
			}
		} else {
			for _, key := range ca.GetTrustedSSHKeyPairs() {
				fingerprint, err := sshutils.AuthorizedKeyFingerprint(key.PublicKey)
				if err != nil {
					return trace.Wrap(err)
				}
				if a.exportAuthorityFingerprint != "" && fingerprint != a.exportAuthorityFingerprint {
					continue
				}

				// export certificates in the old 1.0 format where host and user
				// certificate authorities were exported in the known_hosts format.
				if a.compatVersion == "1.0" {
					castr, err := hostCAFormat(ca, key.PublicKey, client)
					if err != nil {
						return trace.Wrap(err)
					}

					fmt.Println(castr)
					continue
				}

				// export certificate authority in user or host ca format
				var castr string
				switch ca.GetType() {
				case types.UserCA:
					castr, err = userCAFormat(ca, key.PublicKey)
				case types.HostCA:
					castr, err = hostCAFormat(ca, key.PublicKey, client)
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

func (a *AuthCommand) exportTLSAuthority(ctx context.Context, client auth.ClientI, typ types.CertAuthType, unpackPEM bool) error {
	clusterName, err := client.GetDomainName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	certAuthority, err := client.GetCertAuthority(
		ctx,
		types.CertAuthID{Type: typ, DomainName: clusterName},
		a.exportPrivateKeys,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(certAuthority.GetActiveKeys().TLS) != 1 {
		return trace.BadParameter("expected one TLS key pair, got %v", len(certAuthority.GetActiveKeys().TLS))
	}
	keyPair := certAuthority.GetActiveKeys().TLS[0]

	print := func(data []byte) error {
		if !unpackPEM {
			fmt.Println(string(data))
			return nil
		}
		b, _ := pem.Decode(data)
		if b == nil {
			return trace.BadParameter("no PEM data in CA data: %q", data)
		}
		fmt.Println(string(b.Bytes))
		return nil
	}
	if a.exportPrivateKeys {
		if err := print(keyPair.Key); err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(print(keyPair.Cert))
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
		a.jksPassword = jskPass
		return a.generateDatabaseKeys(ctx, clusterAPI)
	case identityfile.FormatSnowflake:
		return a.generateSnowflakeKey(ctx, clusterAPI)
	}
	switch {
	case a.genUser != "" && a.genHost == "":
		log.Info("Generating credentials to allow a machine access to Teleport? We recommend Teleport's Machine ID for this. Find out more at https://goteleport.com/r/machineid-tip")
		return a.generateUserKeys(ctx, clusterAPI)
	case a.genUser == "" && a.genHost != "":
		return a.generateHostKeys(ctx, clusterAPI)
	default:
		return trace.BadParameter("--user or --host must be specified")
	}
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

	key.TrustedCA = []auth.TrustedCerts{{TLSCertificates: services.GetTLSCerts(databaseCA)}}

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
	key.TrustedCA = auth.AuthoritiesToTrustedCerts(hostCAs)

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
		JKSPassword:        a.jksPassword,
	}
	filesWritten, err := db.GenerateDatabaseCertificates(ctx, dbCertReq)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(writeHelperMessageDBmTLS(os.Stdout, filesWritten, a.output, a.outputFormat, a.jksPassword))
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
}

func writeHelperMessageDBmTLS(writer io.Writer, filesWritten []string, output string, outputFormat identityfile.Format, jksPassword string) error {
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
		"files":       strings.Join(filesWritten, ", "),
		"jksPassword": jksPassword,
		"output":      output,
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
   keystore_password: "{{.jksPassword}}"

   require_client_auth: true
   truststore: /path/to/{{.output}}.truststore
   truststore_password: "{{.jksPassword}}"
   protocol: TLS
   algorithm: SunX509
   store_type: JKS
   cipher_suites: [TLS_RSA_WITH_AES_256_CBC_SHA]
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
	key.TrustedCA = auth.AuthoritiesToTrustedCerts(hostCAs)

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
	fmt.Printf("\nThe credentials have been written to %s\n", strings.Join(filesWritten, ", "))

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

		if netConfig.GetProxyListenerMode() == types.ProxyListenerMode_Multiplex {
			u := url.URL{
				Scheme: "https",
				Host:   addr,
			}
			a.proxyAddr = u.String()
			return nil
		}

		uaddr, err := utils.ParseAddr(addr)
		if err != nil {
			log.Warningf("Invalid public address on the proxy %q: %q: %v.", p.GetName(), addr, err)
			continue
		}
		u := url.URL{
			Scheme: "https",
			Host:   net.JoinHostPort(uaddr.Host(), strconv.Itoa(defaults.KubeListenPort)),
		}
		a.proxyAddr = u.String()
		return nil
	}

	return trace.BadParameter("couldn't find registered public proxies, specify --proxy when using --format=%q", identityfile.FormatKubernetes)
}

// userCAFormat returns the certificate authority public key exported as a single
// line that can be placed in ~/.ssh/authorized_keys file. The format adheres to the
// man sshd (8) authorized_keys format, a space-separated list of: options, keytype,
// base64-encoded key, comment.
// For example:
//
//	cert-authority AAA... type=user&clustername=cluster-a
//
// URL encoding is used to pass the CA type and cluster name into the comment field.
func userCAFormat(ca types.CertAuthority, keyBytes []byte) (string, error) {
	return sshutils.MarshalAuthorizedKeysFormat(ca.GetClusterName(), keyBytes)
}

// hostCAFormat returns the certificate authority public key exported as a single line
// that can be placed in ~/.ssh/authorized_hosts. The format adheres to the man sshd (8)
// authorized_hosts format, a space-separated list of: marker, hosts, key, and comment.
// For example:
//
//	@cert-authority *.cluster-a ssh-rsa AAA... type=host
//
// URL encoding is used to pass the CA type and allowed logins into the comment field.
func hostCAFormat(ca types.CertAuthority, keyBytes []byte, client auth.ClientI) (string, error) {
	roles, err := services.FetchRoles(ca.GetRoles(), client, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	allowedLogins, _ := roles.GetLoginsForTTL(apidefaults.MinCertDuration + time.Second)
	return sshutils.MarshalAuthorizedHostsFormat(ca.GetClusterName(), keyBytes, allowedLogins)
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
