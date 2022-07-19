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
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/tlsca"
)

/* signCertKeyPair returns the necessary files to set up mTLS for other services
This is the equivalent of running the tctl command
As an example, requesting:
POST /webapi/sites/mycluster/sign
{
	"hostname": "pg.example.com",
	"ttl": "2190h",
	"format": "db"
}

Should be equivalent to running:
   tctl auth sign --host=pg.example.com --ttl=2190h --format=db

This endpoint returns a tar.gz compressed archive containing the required files to setup mTLS for the service.
*/
func (h *Handler) signCertKeyPair(w http.ResponseWriter, r *http.Request, p httprouter.Params, site reversetunnel.RemoteSite) (interface{}, error) {
	ctx := r.Context()
	req, err := parseSignCertKeyPair(r)
	if err != nil {
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
	Format   identityfile.Format
	TTL      time.Duration
}

// TODO(marco): only format db is supported
var supportedFormats = []identityfile.Format{
	identityfile.FormatDatabase,
}

func parseSignCertKeyPair(r *http.Request) (*signCertKeyPairReq, error) {
	reqRaw := struct {
		Hostname string `json:"hostname,omitempty"`
		Format   string `json:"format,omitempty"`
		TTL      string `json:"ttl,omitempty"`
	}{}
	if err := httplib.ReadJSON(r, &reqRaw); err != nil {
		return nil, trace.Wrap(err)
	}

	ret := &signCertKeyPairReq{}

	ret.Hostname = reqRaw.Hostname

	if reqRaw.Format == "" {
		return nil, trace.BadParameter("missing format")
	}
	ret.Format = identityfile.Format(reqRaw.Format)
	if !slices.Contains(supportedFormats, ret.Format) {
		return nil, trace.BadParameter("invalid format")
	}

	if reqRaw.TTL == "" {
		reqRaw.TTL = apidefaults.CertDuration.String()
	}
	ttl, err := time.ParseDuration(reqRaw.TTL)
	if err != nil {
		return nil, trace.BadParameter("invalid ttl (please use https://pkg.go.dev/time#ParseDuration notation)")
	}
	ret.TTL = ttl

	return ret, nil
}
