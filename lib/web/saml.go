/*
Copyright 2015-2019 Gravitational, Inc.

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
	"net/http"
	"net/url"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/form"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

func (m *Handler) samlSSO(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	log.Debugf("samlSSO start")

	query := r.URL.Query()
	clientRedirectURL := query.Get("redirect_url")
	if clientRedirectURL == "" {
		return nil, trace.BadParameter("missing redirect_url query parameter")
	}
	connectorID := query.Get("connector_id")
	if connectorID == "" {
		return nil, trace.BadParameter("missing connector_id query parameter")
	}

	csrfToken, err := csrf.ExtractTokenFromCookie(r)
	if err != nil {
		log.Warningf("unable to extract CSRF token from cookie %v", err)
		return nil, trace.AccessDenied("access denied")
	}

	response, err := m.cfg.ProxyClient.CreateSAMLAuthRequest(
		services.SAMLAuthRequest{
			ConnectorID:       connectorID,
			CSRFToken:         csrfToken,
			CreateWebSession:  true,
			ClientRedirectURL: clientRedirectURL,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	http.Redirect(w, r, response.RedirectURL, http.StatusFound)
	return nil, nil
}

func (m *Handler) samlSSOConsole(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	log.Debugf("samlSSOConsole start")
	var req client.SSOLoginConsoleReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := m.cfg.ProxyClient.CreateSAMLAuthRequest(
		services.SAMLAuthRequest{
			ConnectorID:       req.ConnectorID,
			ClientRedirectURL: req.RedirectURL,
			PublicKey:         req.PublicKey,
			CertTTL:           req.CertTTL,
			Compatibility:     req.Compatibility,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &client.SSOLoginConsoleResponse{RedirectURL: response.RedirectURL}, nil
}

func (m *Handler) samlACS(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var samlResponse string
	err := form.Parse(r, form.String("SAMLResponse", &samlResponse, form.Required()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	l := log.WithFields(log.Fields{trace.Component: "SAML"})

	response, err := m.cfg.ProxyClient.ValidateSAMLResponse(samlResponse)
	if err != nil {
		log.Warningf("error while processing callback: %v", err)

		message := "Unable to process callback from SAML provider."
		// for not implemented errors it's ok to provide a more specific
		// message as it could give more guidance on what's not enabled
		if trace.IsNotImplemented(err) {
			message = utils.Capitalize(err.Error()) + "."
		}

		// redirect to an error page
		pathToError := url.URL{
			Path:     "/web/msg/error/login_failed",
			RawQuery: url.Values{"details": []string{message}}.Encode(),
		}
		http.Redirect(w, r, pathToError.String(), http.StatusFound)
		return nil, nil
	}

	// if we created web session, set session cookie and redirect to original url
	if response.Req.CreateWebSession {
		log.Debugf("redirecting to web browser")
		err = csrf.VerifyToken(response.Req.CSRFToken, r)
		if err != nil {
			l.Warningf("unable to verify CSRF token: %v", err)
			return nil, trace.AccessDenied("access denied")
		}

		if err := SetSession(w, response.Username, response.Session.GetName()); err != nil {
			return nil, trace.Wrap(err)
		}
		return nil, httplib.SafeRedirect(w, r, response.Req.ClientRedirectURL)
	}
	l.Debugf("samlCallback redirecting to console login")
	if len(response.Req.PublicKey) == 0 {
		return nil, trace.BadParameter("not a web or console oidc login request")
	}
	redirectURL, err := ConstructSSHResponse(AuthParams{
		ClientRedirectURL: response.Req.ClientRedirectURL,
		Username:          response.Username,
		Identity:          response.Identity,
		Session:           response.Session,
		Cert:              response.Cert,
		TLSCert:           response.TLSCert,
		HostSigners:       response.HostSigners,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
	return nil, nil
}
