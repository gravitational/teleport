// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package web

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/client"
)

// authExportPublic returns the CA Certs that can be used to set up a chain of trust which includes the current Teleport Cluster
//
// GET /webapi/sites/:site/auth/export?type=<auth type>
// GET /webapi/auth/export?type=<auth type>
func (h *Handler) authExportPublic(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	if err := h.authExportPublicError(w, r, p); err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}

	// Success output handled by authExportPublicError.
}

// authExportPublicError implements authExportPublic, except it returns an error
// in case of failure. Output is only written on success.
func (h *Handler) authExportPublicError(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	err := rateLimitRequest(r, h.limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	query := r.URL.Query()
	caType := query.Get("type") // validated by ExportAllAuthorities

	ctx := r.Context()
	authorities, err := client.ExportAllAuthorities(
		ctx,
		h.GetProxyClient(),
		client.ExportAuthoritiesRequest{
			AuthType: caType,
		},
	)
	if err != nil {
		h.logger.DebugContext(ctx, "Failed to generate CA Certs", "error", err)
		return trace.Wrap(err)
	}

	format := query.Get("format")

	const formatZip = "zip"
	const formatJSON = "json"
	switch format {
	case "":
		break
	case formatZip:
		return h.authExportPublicZip(w, r, authorities)
	case formatJSON:
		return h.authExportPublicJSON(w, r, authorities)
	default:
		return trace.BadParameter("unsupported format %q", format)
	}

	if l := len(authorities); l > 1 {
		return trace.BadParameter("found %d authorities to export, use format=%s or format=%s to export all", l, formatZip, formatJSON)
	}

	// ServeContent sets the correct headers: Content-Type, Content-Length and Accept-Ranges.
	// It also handles the Range negotiation
	reader := bytes.NewReader(authorities[0].Data)
	http.ServeContent(w, r, "authorized_hosts.txt", time.Now(), reader)
	return nil
}

func (h *Handler) authExportPublicZip(
	w http.ResponseWriter,
	r *http.Request,
	authorities []*client.ExportedAuthority,
) error {
	now := h.clock.Now().UTC()

	// Write authorities to a zip buffer as files named "ca$i.cert".
	out := &bytes.Buffer{}
	zipWriter := zip.NewWriter(out)
	for i, authority := range authorities {
		fh := &zip.FileHeader{
			Name:     fmt.Sprintf("ca%d.cer", i),
			Method:   zip.Deflate,
			Modified: now,
		}
		fh.SetMode(0644)

		fileWriter, err := zipWriter.CreateHeader(fh)
		if err != nil {
			return trace.Wrap(err)
		}
		fileWriter.Write(authority.Data)
	}
	if err := zipWriter.Close(); err != nil {
		return trace.Wrap(err)
	}

	const zipName = "Teleport_CA.zip"
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment;filename="%s"`, zipName))
	http.ServeContent(w, r, zipName, now, bytes.NewReader(out.Bytes()))
	return nil
}

func (h *Handler) authExportPublicJSON(
	w http.ResponseWriter,
	r *http.Request,
	authorities []*client.ExportedAuthority,
) error {
	marshalledAuthorities, err := json.Marshal(authorities)
	if err != nil {
		return trace.Wrap(err, "failed to JSON marshal authorities")
	}

	// File name is not critical here. It is only used by `ServeContent` to determine the value of the
	// `Content-Type` header.
	http.ServeContent(w, r, "export.json", time.Now(), bytes.NewReader(marshalledAuthorities))
	return nil
}
