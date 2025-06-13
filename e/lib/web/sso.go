package web

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	saml2 "github.com/russellhaering/gosaml2"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/saml"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/app"
)

func (p *Plugin) oidcLoginWeb(w http.ResponseWriter, r *http.Request, params httprouter.Params) string {
	logger := p.Logger.With("auth", "oidc")
	logger.DebugContext(r.Context(), "Web login start")

	req, err := web.ParseSSORequestParams(r)
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to extract SSO parameters from request", "error", err)
		return client.LoginFailedRedirectURL
	}

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to parse request remote address", "error", err)
		return client.LoginFailedRedirectURL
	}

	codeVerifier := oauth2.GenerateVerifier()

	http.SetCookie(w, &http.Cookie{
		Name:     pkceCodeVerifierCookieName,
		Value:    codeVerifier,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		MaxAge:   300, // 5 minutes
	})

	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.CreateOIDCAuthRequest(r.Context(), types.OIDCAuthRequest{
		CSRFToken:         req.CSRFToken,
		ConnectorID:       req.ConnectorID,
		PkceVerifier:      codeVerifier,
		CreateWebSession:  true,
		ClientRedirectURL: req.ClientRedirectURL,
		CheckUser:         true,
		ProxyAddress:      r.Host,
		ClientLoginIP:     remoteAddr,
		ClientUserAgent:   r.UserAgent(),
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "Error creating auth request", "error", err)
		// TODO(camh): Consider redirecting to license expired URL for license expiry
		// errors so we can present a nicer error to the user.
		return client.LoginFailedRedirectURL
	}

	return response.RedirectURL
}

func (p *Plugin) oidcLoginConsole(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	logger := p.Logger.With("auth", "oidc")
	logger.DebugContext(r.Context(), "Console login start")

	req := new(client.SSOLoginConsoleReq)
	if err := httplib.ReadJSON(r, req); err != nil {
		logger.ErrorContext(r.Context(), "Error reading json", "error", err)
		return nil, trace.AccessDenied("%s", web.SSOLoginFailureMessage)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		logger.ErrorContext(r.Context(), "Missing request parameters", "error", err)
		return nil, trace.AccessDenied("%s", web.SSOLoginFailureMessage)
	}

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to parse request remote address", "error", err)
		return nil, trace.AccessDenied("%s", web.SSOLoginFailureMessage)
	}

	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.CreateOIDCAuthRequest(r.Context(), types.OIDCAuthRequest{
		ConnectorID:             req.ConnectorID,
		ClientRedirectURL:       req.RedirectURL,
		SshPublicKey:            req.SSHPubKey,
		TlsPublicKey:            req.TLSPubKey,
		SshAttestationStatement: req.SSHAttestationStatement.ToProto(),
		TlsAttestationStatement: req.TLSAttestationStatement.ToProto(),
		CertTTL:                 req.CertTTL,
		PkceVerifier:            req.PKCEVerifier,
		CheckUser:               true,
		Compatibility:           req.Compatibility,
		RouteToCluster:          req.RouteToCluster,
		KubernetesCluster:       req.KubernetesCluster,
		ProxyAddress:            r.Host,
		ClientLoginIP:           remoteAddr,
		ClientUserAgent:         r.UserAgent(),
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to create OIDC auth request", "error", err)
		// Keep a license expired error as is rather than returning a generic error message.
		// There is nothing sensitive in the license expired error.
		if errors.Is(err, eauth.ErrLicenseExpired) {
			return nil, trace.Wrap(err)
		}
		if strings.Contains(err.Error(), auth.InvalidClientRedirectErrorMessage) {
			return nil, trace.AccessDenied("%s", web.SSOLoginFailureInvalidRedirect)
		}
		return nil, trace.AccessDenied("%s", web.SSOLoginFailureMessage)
	}

	return &client.SSOLoginConsoleResponse{
		RedirectURL: response.RedirectURL,
	}, nil
}

func (p *Plugin) oidcCallback(w http.ResponseWriter, r *http.Request, params httprouter.Params) string {
	queryValues := r.URL.Query()
	if codeVerifier, err := r.Cookie(pkceCodeVerifierCookieName); err == nil {
		queryValues.Add("code_verifier", codeVerifier.Value)
	}

	logger := p.Logger.With("auth", "oidc")
	logger.DebugContext(r.Context(), "Callback start")

	// remove our codeVerifier cookie regardless if we succeed or not. It should be one use
	// and will be set again if the login fails
	http.SetCookie(w, &http.Cookie{
		Name:     pkceCodeVerifierCookieName,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.ValidateOIDCAuthCallback(r.Context(), queryValues)
	if err != nil {
		logger.ErrorContext(r.Context(), "Error while processing callback", "error", err)

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
		logger.InfoContext(r.Context(), "Redirecting to web browser")

		res := &web.SSOCallbackResponse{
			CSRFToken:         response.Req.CSRFToken,
			Username:          response.Username,
			SessionName:       response.Session.GetName(),
			ClientRedirectURL: response.Req.ClientRedirectURL,
		}

		if err := web.SSOSetWebSessionAndRedirectURL(w, r, res, true); err != nil {
			logger.ErrorContext(r.Context(), "Error setting web session", "error", err)
			return client.LoginFailedRedirectURL
		}

		if dwt := response.Session.GetDeviceWebToken(); dwt != nil {
			logger.DebugContext(r.Context(), "OIDC WebSession created with device web token")
			// if a device web token is present, we must send the user to the device authorize page
			// to upgrade the session.
			redirectPath, err := web.BuildDeviceWebRedirectPath(dwt, res.ClientRedirectURL)
			if err != nil {
				logger.DebugContext(r.Context(), "Invalid device web token", "error", err)
			}
			return redirectPath
		}
		return res.ClientRedirectURL
	}

	logger.InfoContext(r.Context(), "Callback redirecting to console login")
	if len(response.Req.SSHPubKey) == 0 && len(response.Req.TLSPubKey) == 0 && response.MFAToken == "" {
		logger.ErrorContext(r.Context(), "Not a web or console login request")
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
		MFAToken:          response.MFAToken,
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "Error constructing ssh response", "error", err)
		return client.LoginFailedRedirectURL
	}

	return redirectURL.String()
}

func (p *Plugin) samlSSO(w http.ResponseWriter, r *http.Request, params httprouter.Params) (resp any, err error) {
	logger := p.Logger.With("auth", "saml")
	logger.DebugContext(r.Context(), "Web login start")

	req, err := web.ParseSSORequestParams(r)
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to extract SSO parameters from request", "error", err)
		metaRedirect(r.Context(), w, client.LoginFailedRedirectURL, logger)
		return
	}

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to parse request remote address", "error", err)
		metaRedirect(r.Context(), w, client.LoginFailedRedirectURL, logger)
		return
	}

	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.CreateSAMLAuthRequest(r.Context(), types.SAMLAuthRequest{
		ConnectorID:       req.ConnectorID,
		CSRFToken:         req.CSRFToken,
		CreateWebSession:  true,
		ClientRedirectURL: req.ClientRedirectURL,
		ClientLoginIP:     remoteAddr,
		ClientUserAgent:   r.UserAgent(),
		ClientVersion:     teleport.Version,
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "Error creating auth request", "error", err)
		// TODO(camh): Consider redirecting to license expired URL for license expiry
		// errors so we can present a nicer error to the user.
		metaRedirect(r.Context(), w, client.LoginFailedRedirectURL, logger)
		return
	}
	if len(response.PostForm) > 0 {
		if err := saml.WriteSAMLPostRequestWithHeaders(w, response.PostForm); err != nil {
			metaRedirect(r.Context(), w, client.LoginFailedRedirectURL, logger)
		}
		return
	}

	if !web.IsValidRedirectURL(response.RedirectURL) {
		response.RedirectURL = client.LoginFailedRedirectURL
	}
	metaRedirect(r.Context(), w, response.RedirectURL, logger)
	return
}

func (p *Plugin) samlSSOConsole(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	logger := p.Logger.With("auth", "saml")
	logger.DebugContext(r.Context(), "Console login start")

	req := new(client.SSOLoginConsoleReq)
	if err := httplib.ReadJSON(r, req); err != nil {
		logger.ErrorContext(r.Context(), "Error reading json", "error", err)
		return nil, trace.AccessDenied("%s", web.SSOLoginFailureMessage)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		logger.ErrorContext(r.Context(), "Missing request parameters", "error", err)
		return nil, trace.AccessDenied("%s", web.SSOLoginFailureMessage)
	}

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to parse request remote address", "error", err)
		return nil, trace.AccessDenied("%s", web.SSOLoginFailureMessage)
	}

	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.CreateSAMLAuthRequest(r.Context(), types.SAMLAuthRequest{
		ConnectorID:             req.ConnectorID,
		ClientRedirectURL:       req.RedirectURL,
		SshPublicKey:            req.SSHPubKey,
		TlsPublicKey:            req.TLSPubKey,
		CertTTL:                 req.CertTTL,
		Compatibility:           req.Compatibility,
		RouteToCluster:          req.RouteToCluster,
		KubernetesCluster:       req.KubernetesCluster,
		SshAttestationStatement: req.SSHAttestationStatement.ToProto(),
		TlsAttestationStatement: req.TLSAttestationStatement.ToProto(),
		ClientLoginIP:           remoteAddr,
		ClientUserAgent:         r.UserAgent(),
		ClientVersion:           req.ClientVersion,
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to create SAML auth request", "error", err)
		// Keep a license expired error as is rather than returning a generic error message.
		// There is nothing sensitive in the license expired error.
		if errors.Is(err, eauth.ErrLicenseExpired) {
			return nil, trace.Wrap(err)
		}
		if strings.Contains(err.Error(), auth.InvalidClientRedirectErrorMessage) {
			return nil, trace.AccessDenied("%s", web.SSOLoginFailureInvalidRedirect)
		}
		if errors.Is(err, &eauth.ErrNoHTTPPostBinding) {
			return nil, trace.Wrap(err)
		}
		return nil, trace.AccessDenied("%s", web.SSOLoginFailureMessage)
	}

	return &client.SSOLoginConsoleResponse{
		RedirectURL: response.RedirectURL,
		PostForm:    base64.StdEncoding.EncodeToString(response.PostForm),
	}, nil
}

func (p *Plugin) samlACSHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) string {
	logger := p.Logger.With("auth", "saml")
	logger.DebugContext(r.Context(), "Callback start")

	samlResponse := r.FormValue("SAMLResponse")
	if samlResponse == "" {
		logger.ErrorContext(r.Context(), "Missing SAMLResponse form value in request")
		return client.LoginFailedRedirectURL
	}

	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to parse request remote address", "address", r.RemoteAddr, "error", err)
		return client.LoginFailedRedirectURL
	}
	proxyClient := p.h.GetProxyClient()
	response, err := proxyClient.ValidateSAMLResponse(r.Context(), samlResponse, params.ByName("connector"), clientIP)
	if err != nil {
		logger.ErrorContext(r.Context(), "Error while processing callback", "error", err)

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
		logger.DebugContext(r.Context(), "Redirecting to web browser")

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
			logger.ErrorContext(r.Context(), "Error setting web session", "error", err)
			return client.LoginFailedRedirectURL
		}

		if dwt := response.Session.GetDeviceWebToken(); dwt != nil {
			logger.DebugContext(r.Context(), "SAML WebSession created with device web token")
			// if a device web token is present, we must send the user to the device authorize page
			// to upgrade the session.
			redirectPath, err := web.BuildDeviceWebRedirectPath(dwt, res.ClientRedirectURL)
			if err != nil {
				logger.DebugContext(r.Context(), "Invalid device web token", "error", err)
			}
			return redirectPath
		}
		return res.ClientRedirectURL
	}

	logger.DebugContext(r.Context(), "Callback redirecting to console login")
	if len(response.Req.SSHPubKey) == 0 && len(response.Req.TLSPubKey) == 0 && response.MFAToken == "" {
		logger.ErrorContext(r.Context(), "Not a web or console login request")
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
		MFAToken:          response.MFAToken,
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "Error constructing ssh response", "error,", err)
		return client.LoginFailedRedirectURL
	}

	return redirectURL.String()
}

// samlSLOHandle handles the `LogoutResponse` from a SAML IdP after single logout.
func (p *Plugin) samlSLOHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) string {
	logger := p.Logger.With("auth", "saml")
	logger.DebugContext(r.Context(), "Single logout start")

	relayState := r.FormValue("RelayState")
	username, connectorName, _ := strings.Cut(relayState, ",")

	errorRedirectURL := client.SAMLSingleLogoutFailedRedirectURL + fmt.Sprintf("?connectorName=%s", url.QueryEscape(connectorName))

	samlResponse := r.FormValue("SAMLResponse")
	if samlResponse == "" {
		logger.ErrorContext(r.Context(), "Missing SAMLResponse form value in request")
		return errorRedirectURL
	}

	logoutResponse, err := saml2.DecodeUnverifiedLogoutResponse(samlResponse)
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to decode LogoutResponse", "error", err)
		return errorRedirectURL
	}

	if !strings.Contains(logoutResponse.Status.StatusCode.Value, "Success") {
		logger.ErrorContext(r.Context(), "SAML Single Logout for user failed", "user", username, "status_code", logoutResponse.Status.StatusCode.Value)
		return errorRedirectURL
	}

	return client.DefaultLoginURL
}

func metaRedirect(ctx context.Context, w http.ResponseWriter, redirectURI string, logger *slog.Logger) {
	if err := app.MetaRedirect(w, redirectURI); err != nil {
		logger.WarnContext(ctx, "Failed to issue a redirect", "error", err)
	}
}
