package web

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	saml2 "github.com/russellhaering/gosaml2"

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

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.WithError(err).Error("Failed to parse request remote address.")
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
		ClientLoginIP:     remoteAddr,
	})
	if err != nil {
		logger.WithError(err).Error("Error creating auth request.")
		// TODO(camh): Consider redirecting to license expired URL for license expiry
		// errors so we can present a nicer error to the user.
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

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.WithError(err).Error("Failed to parse request remote address.")
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
		ClientLoginIP:        remoteAddr,
	})
	if err != nil {
		logger.WithError(err).Error("Failed to create OIDC auth request.")
		// Keep a license expired error as is rather than returning a generic error message.
		// There is nothing sensitive in the license expired error.
		if errors.Is(err, eauth.ErrLicenseExpired) {
			return nil, trace.Wrap(err)
		}
		// TODO(espadolini): replace with auth.InvalidClientRedirectErrorMessage
		// and web.SSOLoginFailureInvalidRedirect after
		// https://github.com/gravitational/teleport-private/pull/1433 is merged
		// in OSS
		if strings.Contains(err.Error(), "invalid or disallowed client redirect URL") {
			return nil, trace.AccessDenied("Failed to login due to a disallowed callback URL. Please check Teleport's log for more details.")
		}
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

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.WithError(err).Error("Failed to parse request remote address.")
		return client.LoginFailedRedirectURL
	}

	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.CreateSAMLAuthRequest(r.Context(), types.SAMLAuthRequest{
		ConnectorID:       req.ConnectorID,
		CSRFToken:         req.CSRFToken,
		CreateWebSession:  true,
		ClientRedirectURL: req.ClientRedirectURL,
		ClientLoginIP:     remoteAddr,
	})
	if err != nil {
		logger.WithError(err).Error("Error creating auth request.")
		// TODO(camh): Consider redirecting to license expired URL for license expiry
		// errors so we can present a nicer error to the user.
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

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.WithError(err).Error("Failed to parse request remote address.")
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
		ClientLoginIP:        remoteAddr,
	})
	if err != nil {
		logger.WithError(err).Error("Failed to create SAML auth request.")
		// Keep a license expired error as is rather than returning a generic error message.
		// There is nothing sensitive in the license expired error.
		if errors.Is(err, eauth.ErrLicenseExpired) {
			return nil, trace.Wrap(err)
		}
		// TODO(espadolini): replace with auth.InvalidClientRedirectErrorMessage
		// and web.SSOLoginFailureInvalidRedirect after
		// https://github.com/gravitational/teleport-private/pull/1433 is merged
		// in OSS
		if strings.Contains(err.Error(), "invalid or disallowed client redirect URL") {
			return nil, trace.AccessDenied("Failed to login due to a disallowed callback URL. Please check Teleport's log for more details.")
		}
		return nil, trace.AccessDenied(web.SSOLoginFailureMessage)
	}

	return &client.SSOLoginConsoleResponse{RedirectURL: response.RedirectURL}, nil
}

func (p *Plugin) samlACSHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) string {
	logger := p.Log.WithField("auth", "saml")
	logger.Debug("Callback start.")

	samlResponse := r.FormValue("SAMLResponse")
	if samlResponse == "" {
		logger.Error("Missing SAMLResponse form value in request")
		return client.LoginFailedRedirectURL
	}

	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.WithError(err).Errorf("Failed to parse request remote address %q", r.RemoteAddr)
		return client.LoginFailedRedirectURL
	}
	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.ValidateSAMLResponse(r.Context(), samlResponse, params.ByName("connector"), clientIP)
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

// samlSLOHandle handles the `LogoutResponse` from a SAML IdP after single logout.
func (p *Plugin) samlSLOHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) string {
	logger := p.Log.WithField("auth", "saml")
	logger.Debug("Single logout start.")

	relayState := r.FormValue("RelayState")
	username, connectorName, _ := strings.Cut(relayState, ",")

	errorRedirectURL := client.SAMLSingleLogoutFailedRedirectURL + fmt.Sprintf("?connectorName=%s", url.QueryEscape(connectorName))

	samlResponse := r.FormValue("SAMLResponse")
	if samlResponse == "" {
		logger.Error("Missing SAMLResponse form value in request")
		return errorRedirectURL
	}

	logoutResponse, err := saml2.DecodeUnverifiedLogoutResponse(samlResponse)
	if err != nil {
		logger.Error(err, "Failed to decode LogoutResponse.")
		return errorRedirectURL
	}

	if !strings.Contains(logoutResponse.Status.StatusCode.Value, "Success") {
		logger.Error(fmt.Sprintf("SAML Single Logout for user '%s' failed with status code '%s'", username, logoutResponse.Status.StatusCode.Value))
		return errorRedirectURL
	}

	return client.DefaultLoginURL
}
