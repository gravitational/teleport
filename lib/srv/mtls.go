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

package srv

import (
	"context"
	"crypto/x509/pkix"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
)

type GenerateMTLSFilesRequest struct {
	ClusterAPI          auth.ClientI
	Principals          []string
	OutputFormat        identityfile.Format
	OutputCanOverwrite  bool
	OutputLocation      string
	IdentityFileWriter  identityfile.ConfigWriter
	TTL                 time.Duration
	Key                 *client.Key
	HelperMessageWriter io.Writer
}

func GenerateMTLSFiles(ctx context.Context, req GenerateMTLSFilesRequest) ([]string, error) {
	if req.OutputFormat != identityfile.FormatSnowflake && len(req.Principals) == 1 && req.Principals[0] == "" {
		return nil, trace.BadParameter("at least one hostname must be specified")
	}

	// For CockroachDB node certificates, CommonName must be "node":
	//
	// https://www.cockroachlabs.com/docs/v21.1/cockroach-cert#node-key-and-certificates
	if req.OutputFormat == identityfile.FormatCockroach {
		req.Principals = append([]string{"node"}, req.Principals...)
	}

	subject := pkix.Name{CommonName: req.Principals[0]}

	if req.OutputFormat == identityfile.FormatMongo {
		// Include Organization attribute in MongoDB certificates as well.
		//
		// When using X.509 member authentication, MongoDB requires O or OU to
		// be non-empty so this will make the certs we generate compatible:
		//
		// https://docs.mongodb.com/manual/core/security-internal-authentication/#x.509
		//
		// The actual O value doesn't matter as long as it matches on all
		// MongoDB cluster members so set it to the Teleport cluster name
		// to avoid hardcoding anything.

		clusterNameType, err := req.ClusterAPI.GetClusterName()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		subject.Organization = []string{clusterNameType.GetClusterName()}
	}

	if req.Key == nil {
		key, err := client.NewKey()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req.Key = key
	}

	csr, err := tlsca.GenerateCertificateRequestPEM(subject, req.Key.Priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := req.ClusterAPI.GenerateDatabaseCert(ctx,
		&proto.DatabaseCertRequest{
			CSR: csr,
			// Important to include SANs since CommonName has been deprecated
			// since Go 1.15:
			//   https://golang.org/doc/go1.15#commonname
			ServerNames: req.Principals,
			// Include legacy ServerName for compatibility.
			ServerName:    req.Principals[0],
			TTL:           proto.Duration(req.TTL),
			RequesterName: proto.DatabaseCertRequest_TCTL,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.Key.TLSCert = resp.Cert
	req.Key.TrustedCA = []auth.TrustedCerts{{TLSCertificates: resp.CACerts}}
	filesWritten, err := identityfile.Write(identityfile.WriteConfig{
		OutputPath:           req.OutputLocation,
		Key:                  req.Key,
		Format:               req.OutputFormat,
		OverwriteDestination: req.OutputCanOverwrite,
		Writer:               req.IdentityFileWriter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := WriteHelperMessageDBmTLS(req.HelperMessageWriter, filesWritten, req.OutputLocation, req.OutputFormat); err != nil {
		return nil, trace.Wrap(err)
	}

	return filesWritten, nil
}

var mapIdentityFileFormatHelperTemplate = map[identityfile.Format]*template.Template{
	identityfile.FormatDatabase:  dbAuthSignTpl,
	identityfile.FormatMongo:     mongoAuthSignTpl,
	identityfile.FormatCockroach: cockroachAuthSignTpl,
	identityfile.FormatRedis:     redisAuthSignTpl,
	identityfile.FormatSnowflake: snowflakeAuthSignTpl,
}

func WriteHelperMessageDBmTLS(writer io.Writer, filesWritten []string, output string, outputFormat identityfile.Format) error {
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
		"files":  strings.Join(filesWritten, ", "),
		"output": output,
	}

	if outputFormat == identityfile.FormatSnowflake {
		delete(tplVars, "output")
	}

	return trace.Wrap(tpl.Execute(writer, tplVars))
}

var (
	// dbAuthSignTpl is printed when user generates credentials for a self-hosted database.
	dbAuthSignTpl = template.Must(template.New("").Parse(`Database credentials have been written to {{.files}}.

To enable mutual TLS on your PostgreSQL server, add the following to its
postgresql.conf configuration file:

ssl = on
ssl_cert_file = '/path/to/{{.output}}.crt'
ssl_key_file = '/path/to/{{.output}}.key'
ssl_ca_file = '/path/to/{{.output}}.cas'

To enable mutual TLS on your MySQL server, add the following to its
mysql.cnf configuration file:

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
)
