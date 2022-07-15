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
	"archive/zip"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

	outputDir, err := os.MkdirTemp("", "teleport-auth-sign")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer os.RemoveAll(outputDir)

	key.TLSCert = resp.Cert
	key.TrustedCA = []auth.TrustedCerts{{TLSCertificates: resp.CACerts}}
	filesWritten, err := identityfile.Write(identityfile.WriteConfig{
		OutputPath:           outputDir + "/server",
		Key:                  key,
		Format:               req.Format,
		OverwriteDestination: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	archiveBaseName := fmt.Sprintf("teleport_mTLS_%s.zip", req.Hostname)
	archiveFullPath := fmt.Sprintf("%s/%s", outputDir, archiveBaseName)

	if err := buildArchiveFromFiles(filesWritten, archiveFullPath); err != nil {
		return nil, trace.Wrap(err)
	}

	// Set file name
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment;filename="%v"`, archiveBaseName))

	// ServeFile sets the correct headers: Content-Type, Content-Length and Accept-Ranges.
	// It also handles the Range negotiation
	http.ServeFile(w, r, archiveFullPath)

	return nil, nil
}

func buildArchiveFromFiles(files []string, zipLocation string) error {
	// We remove the entire directory above.
	// No need to remove each file seperatly.
	archive, err := os.Create(zipLocation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer archive.Close()

	zipWriter := zip.NewWriter(archive)
	defer zipWriter.Close()

	for _, fullFilename := range files {
		baseFilename := filepath.Base(fullFilename)
		if err := addFileToZipWriter(fullFilename, baseFilename, zipWriter); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func addFileToZipWriter(fullFilename string, baseFilename string, zipWriter *zip.Writer) error {
	f, err := os.Open(fullFilename)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	zipFileWriter, err := zipWriter.Create(baseFilename)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := io.Copy(zipFileWriter, f); err != nil {
		return trace.Wrap(err)
	}

	return nil
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
