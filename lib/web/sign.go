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
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/exp/slices"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
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
	req := &signCertKeyPairReq{}
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	virtualFS := identityfile.NewInMemoryConfigWriter()

	mTLSReq := srv.GenerateMTLSFilesRequest{
		ClusterAPI:          h.auth.proxyClient,
		Principals:          []string{req.Hostname},
		OutputFormat:        req.Format,
		OutputCanOverwrite:  true,
		OutputLocation:      "server",
		IdentityFileWriter:  virtualFS,
		TTL:                 req.TTL,
		HelperMessageWriter: nil,
	}
	filesWritten, err := srv.GenerateMTLSFiles(r.Context(), mTLSReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	archiveName := fmt.Sprintf("teleport_mTLS_%s.tar.gz", req.Hostname)

	// https://www.postgresql.org/docs/current/libpq-ssl.html
	// On Unix systems, the permissions on the private key file must disallow any access to world or group;
	//  achieve this by a command such as chmod 0600 ~/.postgresql/postgresql.key.
	// Alternatively, the file can be owned by root and have group read access (that is, 0640 permissions).
	fileMode := fs.FileMode(teleport.FileMaskOwnerOnly) // 0600
	archiveBytes, err := utils.CompressTarGzArchive(filesWritten, virtualFS, fileMode)
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

type signCertKeyPairReq struct {
	Hostname  string `json:"hostname,omitempty"`
	FormatRaw string `json:"format,omitempty"`
	TTLRaw    string `json:"ttl,omitempty"`
	Format    identityfile.Format
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

	if s.FormatRaw == "" {
		return trace.BadParameter("missing format")
	}
	s.Format = identityfile.Format(s.FormatRaw)
	if !slices.Contains(supportedFormats, s.Format) {
		return trace.BadParameter("provided format '%s' is not valid, supported formats are: %q", s.Format, supportedFormats)
	}

	if s.TTLRaw == "" {
		s.TTLRaw = apidefaults.CertDuration.String()
	}
	ttl, err := time.ParseDuration(s.TTLRaw)
	if err != nil {
		return trace.BadParameter("invalid ttl '%s', use https://pkg.go.dev/time#ParseDuration format (example: 2190h)", s.TTLRaw)
	}
	s.TTL = ttl

	return nil
}
