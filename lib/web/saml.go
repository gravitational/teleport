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

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/form"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

func (h *Handler) samlSSO(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	logger := h.log.WithField("auth", "saml")
	logger.Debug("Web login start.")

	req, err := parseSSORequestParams(r)
	if err != nil {
		logger.Error(err)
		http.Redirect(w, r, loginFailedURL, http.StatusFound)
		return nil, nil
	}

	response, err := h.cfg.ProxyClient.CreateSAMLAuthRequest(
		services.SAMLAuthRequest{
			ConnectorID:       req.connectorID,
			CSRFToken:         req.csrfToken,
			CreateWebSession:  true,
			ClientRedirectURL: req.clientRedirectURL,
		})
	if err != nil {
		logger.WithError(err).Error("Error creating auth request.")
		http.Redirect(w, r, loginFailedURL, http.StatusFound)
		return nil, nil
	}

	http.Redirect(w, r, response.RedirectURL, http.StatusFound)
	return nil, nil
}

func (h *Handler) samlSSOConsole(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	h.log.WithField("auth", "saml").Debug("SSO console start.")
	req := new(client.SSOLoginConsoleReq)
	if err := httplib.ReadJSON(r, req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := h.cfg.ProxyClient.CreateSAMLAuthRequest(
		services.SAMLAuthRequest{
			ConnectorID:       req.ConnectorID,
			ClientRedirectURL: req.RedirectURL,
			PublicKey:         req.PublicKey,
			CertTTL:           req.CertTTL,
			Compatibility:     req.Compatibility,
			RouteToCluster:    req.RouteToCluster,
			KubernetesCluster: req.KubernetesCluster,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &client.SSOLoginConsoleResponse{RedirectURL: response.RedirectURL}, nil
}

func (h *Handler) samlACS(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	logger := h.log.WithField("auth", "saml")
	logger.Debug("Callback start.")

	var samlResponse string
	if err := form.Parse(r, form.String("SAMLResponse", &samlResponse, form.Required())); err != nil {
		logger.WithError(err).Error("Error parsing response.")
		http.Redirect(w, r, loginFailedURL, http.StatusFound)
		return nil, nil
	}

	response, err := h.cfg.ProxyClient.ValidateSAMLResponse(samlResponse)
	if err != nil {
		logger.WithError(err).Error("Error while processing callback.")
		http.Redirect(w, r, loginFailedBadCallbackURL, http.StatusFound)
		return nil, nil
	}

	// if we created web session, set session cookie and redirect to original url
	if response.Req.CreateWebSession {
		logger.Debug("Redirecting to web browser.")
		if err := csrf.VerifyToken(response.Req.CSRFToken, r); err != nil {
			logger.WithError(err).Error("Unable to verify CSRF token.")
			http.Redirect(w, r, loginFailedURL, http.StatusFound)
			return nil, nil
		}
		if err := SetSessionCookie(w, response.Username, response.Session.GetName()); err != nil {
			logger.WithError(err).Error("Unable to set session cookie.")
			http.Redirect(w, r, loginFailedURL, http.StatusFound)
			return nil, nil
		}

		if err := httplib.SafeRedirect(w, r, response.Req.ClientRedirectURL); err != nil {
			logger.WithError(err).Error("Error parsing redirect URL.")
			http.Redirect(w, r, loginFailedURL, http.StatusFound)
		}

		return nil, nil
	}

	logger.Debug("Callback redirecting to console login.")
	if len(response.Req.PublicKey) == 0 {
		logger.Error("Not a web or console login request.")
		http.Redirect(w, r, loginFailedURL, http.StatusFound)
		return nil, nil
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
		logger.WithError(err).Error("Error constructing ssh response.")
		http.Redirect(w, r, loginFailedURL, http.StatusFound)
		return nil, nil
	}

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
	return nil, nil
}
