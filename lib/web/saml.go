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

	services "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"

	"github.com/gravitational/form"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

func (h *Handler) samlSSO(w http.ResponseWriter, r *http.Request, p httprouter.Params) string {
	logger := h.log.WithField("auth", "saml")
	logger.Debug("Web login start.")

	req, err := parseSSORequestParams(r)
	if err != nil {
		logger.WithError(err).Error("Failed to extract SSO parameters from request.")
		return client.LoginFailedRedirectURL
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
		return client.LoginFailedRedirectURL
	}

	return response.RedirectURL
}

func (h *Handler) samlSSOConsole(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	logger := h.log.WithField("auth", "saml")
	logger.Debug("Console login start.")

	req := new(client.SSOLoginConsoleReq)
	if err := httplib.ReadJSON(r, req); err != nil {
		logger.WithError(err).Error("Error reading json.")
		return nil, trace.AccessDenied(ssoLoginConsoleErr)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		logger.WithError(err).Error("Missing request parameters.")
		return nil, trace.AccessDenied(ssoLoginConsoleErr)
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
		logger.WithError(err).Error("Failed to create SAML auth request.")
		return nil, trace.AccessDenied(ssoLoginConsoleErr)
	}

	return &client.SSOLoginConsoleResponse{RedirectURL: response.RedirectURL}, nil
}

func (h *Handler) samlACS(w http.ResponseWriter, r *http.Request, p httprouter.Params) string {
	logger := h.log.WithField("auth", "saml")
	logger.Debug("Callback start.")

	var samlResponse string
	if err := form.Parse(r, form.String("SAMLResponse", &samlResponse, form.Required())); err != nil {
		logger.WithError(err).Error("Error parsing response.")
		return client.LoginFailedRedirectURL
	}

	response, err := h.cfg.ProxyClient.ValidateSAMLResponse(samlResponse)
	if err != nil {
		logger.WithError(err).Error("Error while processing callback.")
		return client.LoginFailedBadCallbackRedirectURL
	}

	// if we created web session, set session cookie and redirect to original url
	if response.Req.CreateWebSession {
		logger.Debug("Redirecting to web browser.")

		res := &ssoCallbackResponse{
			csrfToken:         response.Req.CSRFToken,
			username:          response.Username,
			sessionName:       response.Session.GetName(),
			clientRedirectURL: response.Req.ClientRedirectURL,
		}

		if err := ssoSetWebSessionAndRedirectURL(w, r, res); err != nil {
			logger.WithError(err).Error("Error setting web session.")
			return client.LoginFailedRedirectURL
		}

		return res.clientRedirectURL
	}

	logger.Debug("Callback redirecting to console login.")
	if len(response.Req.PublicKey) == 0 {
		logger.Error("Not a web or console login request.")
		return client.LoginFailedRedirectURL
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
		return client.LoginFailedRedirectURL
	}

	return redirectURL.String()
}
