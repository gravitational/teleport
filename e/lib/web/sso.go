package web

import (
	"errors"
	"net/http"

	"github.com/gravitational/form"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) oidcLoginWeb(w http.ResponseWriter, r *http.Request, params httprouter.Params) string {
	logger := p.Log.WithField("auth", "oidc")
	logger.Debug("Web login start.")

	req, err := web.ParseSSORequestParams(r)
	if err != nil {
		logger.WithError(err).Error("Failed to extract SSO parameters from request.")
		return client.LoginFailedRedirectURL
	}

	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.CreateOIDCAuthRequest(r.Context(), types.OIDCAuthRequest{
		CSRFToken:         req.CSRFToken,
		ConnectorID:       req.ConnectorID,
		CreateWebSession:  true,
		ClientRedirectURL: req.ClientRedirectURL,
		CheckUser:         true,
		ProxyAddress:      r.Host,
	})
	if err != nil {
		logger.WithError(err).Error("Error creating auth request.")
		return client.LoginFailedRedirectURL
	}

	return response.RedirectURL
}

func (p *Plugin) oidcLoginConsole(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	logger := p.Log.WithField("auth", "oidc")
	logger.Debug("Console login start.")

	req := new(client.SSOLoginConsoleReq)
	if err := httplib.ReadJSON(r, req); err != nil {
		logger.WithError(err).Error("Error reading json.")
		return nil, trace.AccessDenied(web.SSOLoginFailureMessage)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		logger.WithError(err).Error("Missing request parameters.")
		return nil, trace.AccessDenied(web.SSOLoginFailureMessage)
	}

	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.CreateOIDCAuthRequest(r.Context(), types.OIDCAuthRequest{
		ConnectorID:          req.ConnectorID,
		ClientRedirectURL:    req.RedirectURL,
		PublicKey:            req.PublicKey,
		CertTTL:              req.CertTTL,
		CheckUser:            true,
		Compatibility:        req.Compatibility,
		RouteToCluster:       req.RouteToCluster,
		KubernetesCluster:    req.KubernetesCluster,
		ProxyAddress:         r.Host,
		AttestationStatement: req.AttestationStatement.ToProto(),
	})
	if err != nil {
		logger.WithError(err).Error("Failed to create OIDC auth request.")
		return nil, trace.AccessDenied(web.SSOLoginFailureMessage)
	}

	return &client.SSOLoginConsoleResponse{
		RedirectURL: response.RedirectURL,
	}, nil
}

func (p *Plugin) oidcCallback(w http.ResponseWriter, r *http.Request, params httprouter.Params) string {
	logger := p.Log.WithField("auth", "oidc")
	logger.Debug("Callback start.")

	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.ValidateOIDCAuthCallback(r.Context(), r.URL.Query())
	if err != nil {
		logger.WithError(err).Error("Error while processing callback.")

		// try to find the auth request, which bears the original client redirect URL.
		// if found, use it to terminate the flow.
		//
		// this improves the UX by terminating the failed SSO flow immediately, rather than hoping for a timeout.
		if requestID := r.URL.Query().Get("state"); requestID != "" {
			if request, errGet := proxyClient.GetOIDCAuthRequest(r.Context(), requestID); errGet == nil && !request.CreateWebSession {
				if redURL, errEnc := web.RedirectURLWithError(request.ClientRedirectURL, err); errEnc == nil {
					return redURL.String()
				}
			}
		}

		if errors.Is(err, eauth.ErrOIDCNoRoles) {
			return client.LoginFailedUnauthorizedRedirectURL
		}

		return client.LoginFailedBadCallbackRedirectURL
	}

	// if we created web session, set session cookie and redirect to original url
	if response.Req.CreateWebSession {
		logger.Info("Redirecting to web browser.")

		res := &web.SSOCallbackResponse{
			CSRFToken:         response.Req.CSRFToken,
			Username:          response.Username,
			SessionName:       response.Session.GetName(),
			ClientRedirectURL: response.Req.ClientRedirectURL,
		}

		if err := web.SSOSetWebSessionAndRedirectURL(w, r, res, true); err != nil {
			logger.WithError(err).Error("Error setting web session.")
			return client.LoginFailedRedirectURL
		}

		return res.ClientRedirectURL
	}

	logger.Info("Callback redirecting to console login.")
	if len(response.Req.PublicKey) == 0 {
		logger.Error("Not a web or console login request.")
		return client.LoginFailedRedirectURL
	}

	redirectURL, err := web.ConstructSSHResponse(web.AuthParams{
		ClientRedirectURL: response.Req.ClientRedirectURL,
		Username:          response.Username,
		Identity:          response.Identity,
		Session:           response.Session,
		Cert:              response.Cert,
		TLSCert:           response.TLSCert,
		HostSigners:       response.HostSigners,
	})
	if err != nil {
		logger.WithError(err).Error("Error constructing ssh response")
		return client.LoginFailedRedirectURL
	}

	return redirectURL.String()
}

func (p *Plugin) samlSSO(w http.ResponseWriter, r *http.Request, params httprouter.Params) string {
	logger := p.Log.WithField("auth", "saml")
	logger.Debug("Web login start.")

	req, err := web.ParseSSORequestParams(r)
	if err != nil {
		logger.WithError(err).Error("Failed to extract SSO parameters from request.")
		return client.LoginFailedRedirectURL
	}

	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.CreateSAMLAuthRequest(r.Context(), types.SAMLAuthRequest{
		ConnectorID:       req.ConnectorID,
		CSRFToken:         req.CSRFToken,
		CreateWebSession:  true,
		ClientRedirectURL: req.ClientRedirectURL,
	})
	if err != nil {
		logger.WithError(err).Error("Error creating auth request.")
		return client.LoginFailedRedirectURL
	}

	return response.RedirectURL
}

func (p *Plugin) samlSSOConsole(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	logger := p.Log.WithField("auth", "saml")
	logger.Debug("Console login start.")

	req := new(client.SSOLoginConsoleReq)
	if err := httplib.ReadJSON(r, req); err != nil {
		logger.WithError(err).Error("Error reading json.")
		return nil, trace.AccessDenied(web.SSOLoginFailureMessage)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		logger.WithError(err).Error("Missing request parameters.")
		return nil, trace.AccessDenied(web.SSOLoginFailureMessage)
	}

	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.CreateSAMLAuthRequest(r.Context(), types.SAMLAuthRequest{
		ConnectorID:          req.ConnectorID,
		ClientRedirectURL:    req.RedirectURL,
		PublicKey:            req.PublicKey,
		CertTTL:              req.CertTTL,
		Compatibility:        req.Compatibility,
		RouteToCluster:       req.RouteToCluster,
		KubernetesCluster:    req.KubernetesCluster,
		AttestationStatement: req.AttestationStatement.ToProto(),
	})
	if err != nil {
		logger.WithError(err).Error("Failed to create SAML auth request.")
		return nil, trace.AccessDenied(web.SSOLoginFailureMessage)
	}

	return &client.SSOLoginConsoleResponse{RedirectURL: response.RedirectURL}, nil
}

func (p *Plugin) samlACSHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) string {
	logger := p.Log.WithField("auth", "saml")
	logger.Debug("Callback start.")

	var samlResponse string
	if err := form.Parse(r, form.String("SAMLResponse", &samlResponse, form.Required())); err != nil {
		logger.WithError(err).Error("Error parsing response.")
		return client.LoginFailedRedirectURL
	}

	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.ValidateSAMLResponse(r.Context(), samlResponse, params.ByName("connector"))

	if err != nil {
		logger.WithError(err).Error("Error while processing callback.")

		// try to find the auth request, which bears the original client redirect URL.
		// if found, use it to terminate the flow.
		//
		// this improves the UX by terminating the failed SSO flow immediately, rather than hoping for a timeout.
		if requestID, errParse := eauth.ParseSAMLInResponseTo(samlResponse); errParse == nil {
			if request, errGet := proxyClient.GetSAMLAuthRequest(r.Context(), requestID); errGet == nil && !request.CreateWebSession {
				if url, errEnc := web.RedirectURLWithError(request.ClientRedirectURL, err); errEnc == nil {
					return url.String()
				}
			}
		}

		if errors.Is(err, eauth.ErrSAMLNoRoles) {
			return client.LoginFailedUnauthorizedRedirectURL
		}

		return client.LoginFailedBadCallbackRedirectURL
	}

	// if we created web session, set session cookie and redirect to original url
	if response.Req.CreateWebSession {
		logger.Debug("Redirecting to web browser.")

		redirect := response.Req.ClientRedirectURL
		if redirect == "" {
			redirect = "/web/"
		}

		res := &web.SSOCallbackResponse{
			CSRFToken:         response.Req.CSRFToken,
			Username:          response.Username,
			SessionName:       response.Session.GetName(),
			ClientRedirectURL: redirect,
		}

		if err := web.SSOSetWebSessionAndRedirectURL(w, r, res, response.Req.CSRFToken != ""); err != nil {
			logger.WithError(err).Error("Error setting web session.")
			return client.LoginFailedRedirectURL
		}

		return res.ClientRedirectURL
	}

	logger.Debug("Callback redirecting to console login.")
	if len(response.Req.PublicKey) == 0 {
		logger.Error("Not a web or console login request.")
		return client.LoginFailedRedirectURL
	}

	redirectURL, err := web.ConstructSSHResponse(web.AuthParams{
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
