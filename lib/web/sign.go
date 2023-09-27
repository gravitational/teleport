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
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/db"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils"
)

/*
signDatabaseCertificate returns the necessary files to set up mTLS using the `db` format
This is the equivalent of running the tctl command
As an example, requesting:
POST /webapi/sites/mycluster/sign/db

	{
		"hostname": "pg.example.com",
		"ttl": "2190h"
	}

Should be equivalent to running:

	tctl auth sign --host=pg.example.com --ttl=2190h --format=db

This endpoint returns a tar.gz compressed archive containing the required files to setup mTLS for the database.
*/
func (h *Handler) signDatabaseCertificate(w http.ResponseWriter, r *http.Request, p httprouter.Params, site reversetunnelclient.RemoteSite, token types.ProvisionToken) (interface{}, error) {
	if !token.GetRoles().Include(types.RoleDatabase) {
		return nil, trace.AccessDenied("required '%s' role was not provided by the token", types.RoleDatabase)
	}

	req := &signDatabaseCertificateReq{}
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	virtualFS := identityfile.NewInMemoryConfigWriter()

	dbCertReq := db.GenerateDatabaseCertificatesRequest{
		ClusterAPI:         h.auth.proxyClient,
		Principals:         []string{req.Hostname},
		OutputFormat:       identityfile.FormatDatabase,
		OutputCanOverwrite: true,
		OutputLocation:     "server",
		IdentityFileWriter: virtualFS,
		TTL:                req.TTL,
	}
	filesWritten, err := db.GenerateDatabaseCertificates(r.Context(), dbCertReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	archiveName := fmt.Sprintf("teleport_mTLS_%s.tar.gz", req.Hostname)
	archiveBytes, err := utils.CompressTarGzArchive(filesWritten, virtualFS)
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

type signDatabaseCertificateReq struct {
	Hostname string `json:"hostname,omitempty"`
	TTLRaw   string `json:"ttl,omitempty"`

	TTL time.Duration `json:"-"`
}

// CheckAndSetDefaults will validate and convert the received values
// Hostname must not be empty
// TTL must either be a valid time.Duration or empty (inherits apidefaults.CertDuration)
func (s *signDatabaseCertificateReq) CheckAndSetDefaults() error {
	if s.Hostname == "" {
		return trace.BadParameter("missing hostname")
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
