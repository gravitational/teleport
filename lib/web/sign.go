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

package web

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/x509/pkix"
	"fmt"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/tlsca"
)

// signCertKeyPair returns the necessary files to set up mTLS for other services
// URL template: GET /webapi/sites/:site/sign?hostname=<hostname>&ttl=<ttl>&format=<format>
//
// As an example, requesting:
//    GET /webapi/sites/:site/sign?hostname=pg.example.com&ttl=2190h&format=db
// should be equivalent to running:
//    tctl auth sign --host=pg.example.com --ttl=2190h --format=db
//
// This endpoint returns a zip compressed archive containing the required files to setup mTLS for the service.
// As an example, for db format it returns an archive with 3 files: server.cas, server.crt and server.key
func (h *Handler) signCertKeyPair(w http.ResponseWriter, r *http.Request, p httprouter.Params, site reversetunnel.RemoteSite) (interface{}, error) {
	ctx := r.Context()

	req := signCertKeyPairReq{
		Hostname:     r.URL.Query().Get("hostname"),
		FormatString: r.URL.Query().Get("format"),
		TTLString:    r.URL.Query().Get("ttl"),
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := client.NewKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	subject := pkix.Name{CommonName: req.Hostname}
	csr, err := tlsca.GenerateCertificateRequestPEM(subject, key.Priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterAPI := h.auth.proxyClient
	resp, err := clusterAPI.GenerateDatabaseCert(
		ctx,
		&proto.DatabaseCertRequest{
			CSR: csr,
			// Important to include SANs since CommonName has been deprecated
			ServerNames: []string{req.Hostname},
			// Include legacy ServerName for compatibility.
			ServerName:    req.Hostname,
			TTL:           proto.Duration(req.TTL),
			RequesterName: proto.DatabaseCertRequest_TCTL,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.TLSCert = resp.Cert
	key.TrustedCA = []auth.TrustedCerts{{TLSCertificates: resp.CACerts}}

	virtualFS := identityfile.NewInMemoryConfigWriter()

	filesWritten, err := identityfile.Write(identityfile.WriteConfig{
		OutputPath:           "server",
		Key:                  key,
		Format:               req.Format,
		OverwriteDestination: true,
		Writer:               virtualFS,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	archiveName := fmt.Sprintf("teleport_mTLS_%s.tar.gz", req.Hostname)

	archiveBytes, err := archiveFromFiles(filesWritten, virtualFS)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set file name
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment;filename="%v"`, archiveName))

	// ServeContent sets the correct headers: Content-Type, Content-Length and Accept-Ranges.
	// It also handles the Range negotiation
	http.ServeContent(w, r, archiveName, time.Now(), bytes.NewReader(archiveBytes.Bytes()))

	return nil, nil
}

// archiveFromFiles builds a Tar Gzip archive in memory, reading the files from the virtual FS
func archiveFromFiles(files []string, virtualFS identityfile.InMemoryConfigWriter) (*bytes.Buffer, error) {
	archiveBytes := &bytes.Buffer{}

	gzipWriter := gzip.NewWriter(archiveBytes)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for _, filename := range files {
		bs, err := virtualFS.Read(filename)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := tarWriter.WriteHeader(&tar.Header{
			Name: filename,
			Size: int64(len(bs)),
		}); err != nil {
			return nil, trace.Wrap(err)
		}

		if _, err := tarWriter.Write(bs); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return archiveBytes, nil
}

type signCertKeyPairReq struct {
	Hostname string

	FormatString string
	Format       identityfile.Format

	TTLString string
	TTL       time.Duration
}

// TODO(marco): only format db is supported
var supportedFormats = []identityfile.Format{
	identityfile.FormatDatabase,
}

func (s *signCertKeyPairReq) CheckAndSetDefaults() error {
	if s.Hostname == "" {
		return trace.BadParameter("missing hostname")
	}

	if s.FormatString == "" {
		return trace.BadParameter("missing format")
	}
	s.Format = identityfile.Format(s.FormatString)
	if !slices.Contains(supportedFormats, s.Format) {
		return trace.BadParameter("invalid format")
	}

	if s.TTLString == "" {
		s.TTLString = apidefaults.CertDuration.String()
	}
	ttl, err := time.ParseDuration(s.TTLString)
	if err != nil {
		return trace.BadParameter("invalid ttl (please use https://pkg.go.dev/time#ParseDuration notation)")
	}
	s.TTL = ttl

	return nil
}
