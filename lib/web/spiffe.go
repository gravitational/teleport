// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// getSPIFFEBundle returns the SPIFFE-compatible trust bundle which allows other
// trust domains to federate with this Teleport cluster.
//
// Mounted at /webapi/spiffe/bundle.json
//
// Must abide by the standard for a "https_web" profile as described in
// https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Federation.md#5-serving-and-consuming-a-spiffe-bundle-endpoint
func (h *Handler) getSPIFFEBundle(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	cn, err := h.GetAccessPoint().GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err, "fetching cluster name")
	}

	td, err := spiffeid.TrustDomainFromString(cn.GetClusterName())
	if err != nil {
		return nil, trace.Wrap(err, "creating trust domain")
	}
	bundle := spiffebundle.New(td)

	spiffeCA, err := h.GetAccessPoint().GetCertAuthority(r.Context(), types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: cn.GetClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err, "fetching SPIFFE CA")
	}

	for _, certPEM := range services.GetTLSCerts(spiffeCA) {
		cert, err := tlsca.ParseCertificatePEM(certPEM)
		if err != nil {
			return nil, trace.Wrap(err, "parsing certificate")
		}
		bundle.AddX509Authority(cert)
	}

	bundleBytes, err := bundle.Marshal()
	if err != nil {
		return nil, trace.Wrap(err, "marshalling bundle")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(bundleBytes)
	if err != nil {
		h.logger.DebugContext(h.cfg.Context, "Failed to write SPIFFE bundle response", "error", err)
	}
	return nil, nil
}
