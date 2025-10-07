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
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"

	"connectrpc.com/connect"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1/devicetrustv1connect"
	"github.com/gravitational/teleport/lib/web/app"
)

// deviceWebConfirm is the last step in device web authentication, where the
// "authenticator process" (aka Connect) forwards the DeviceConfirmationToken
// back to the Auth Server, via the Proxy.
//
// GET /webapi/devices/webconfirm?id=a&token=b
//
// - id: ID of the confirmation token.
// - token: raw confirmation token.
//
// The result of this call is a redirect to "/web", regardless of the outcome of
// the ConfirmDeviceWebAuthentication RPC.
func (h *Handler) deviceWebConfirm(w http.ResponseWriter, r *http.Request, _ httprouter.Params, sessionCtx *SessionContext) (any, error) {
	query := r.URL.Query()

	// Read input parameters.
	confirmToken := &devicepb.DeviceConfirmationToken{}
	confirmToken.Id = query.Get("id")
	confirmToken.Token = query.Get("token")
	unsafeRedirectURI := query.Get("redirect_uri")

	switch {
	case confirmToken.Id == "":
		return nil, trace.BadParameter("parameter id required")
	case confirmToken.Token == "":
		return nil, trace.BadParameter("parameter token required")
	}

	// Use the Proxy identity for this call. Only the Proxy is allowed to do it.
	devicesClient := h.GetProxyClient().DevicesClient()
	ctx := r.Context()

	_, err := devicesClient.ConfirmDeviceWebAuthentication(ctx, &devicepb.ConfirmDeviceWebAuthenticationRequest{
		ConfirmationToken:   confirmToken,
		CurrentWebSessionId: sessionCtx.GetSessionID(),
	})
	switch {
	case err != nil:
		h.logger.WarnContext(ctx, "Device web authentication confirm failed",
			"error", err,
			"user", sessionCtx.GetUser(),
		)
		// err swallowed on purpose.
	default:
		// Preemptively release session from cache, as its certificates are now
		// updated.
		// The WebSession watcher takes care of this in other proxy instances
		// (see [sessionCache.watchWebSessions]).
		h.auth.releaseResources(r.Context(), sessionCtx.GetUser(), sessionCtx.GetSessionID())
	}

	// Always redirect back to the dashboard, regardless of outcome.
	app.SetRedirectPageHeaders(w.Header(), "" /* nonce */)

	redirectTo, err := h.getRedirectURL(r.Host, unsafeRedirectURI)
	if err != nil {
		h.logger.DebugContext(ctx, "Unable to parse redirectURI",
			"error", err,
			"redirect_uri", unsafeRedirectURI,
		)
		http.Error(w, http.StatusText(trace.ErrorToCode(err)), trace.ErrorToCode(err))
		return nil, nil
	}
	http.Redirect(w, r, redirectTo, http.StatusSeeOther)

	return nil, nil
}

// getRedirectPath tries to parse the given unsafeRedirectURI.
// It returns a full URL if the unsafeRedirectURI points to SAML IdP SSO endpoint.
// In any other case, as long as the redirect URL is parsable, it returns
// a path ensuring its prefixed with "/web".
func (h *Handler) getRedirectURL(host, unsafeRedirectURI string) (string, error) {
	const (
		basePath                = "/web"
		samlSPInitiatedSSOPath  = "/enterprise/saml-idp/sso"
		samlIDPInitiatedSSOPath = "/enterprise/saml-idp/login"
	)

	if unsafeRedirectURI == "" {
		return basePath, nil
	}

	parsedURL, err := url.Parse(unsafeRedirectURI)
	if err != nil {
		return basePath, trace.BadParameter("invalid redirect URL")
	}

	cleanPath := path.Clean(parsedURL.Path)
	// helps in situations where there is no path such as https://example.com
	if cleanPath == "." || cleanPath == ".." {
		cleanPath = "/"
	} else if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	// IDP initiated SSO path format: "/enterprise/saml-idp/login/<service provider name>"
	isIdpInitiatedSSOPath := strings.HasPrefix(cleanPath, samlIDPInitiatedSSOPath) && len(strings.Split(cleanPath, "/")) == 5
	if cleanPath == samlSPInitiatedSSOPath || isIdpInitiatedSSOPath {
		if parsedURL.Host != host {
			return "", trace.BadParameter("host mismatch")
		}
		path := samlSPInitiatedSSOPath
		if isIdpInitiatedSSOPath {
			path = cleanPath
		}
		ensuredURL := &url.URL{
			Scheme:   "https",
			Host:     host,
			Path:     path,
			RawQuery: parsedURL.RawQuery,
		}
		return ensuredURL.String(), nil
	}

	// Prepend "/web" only if it's not already present
	if !strings.HasPrefix(cleanPath, basePath) {
		return path.Join(basePath, cleanPath), nil
	}
	return cleanPath, nil
}

type deviceTrustServer struct {
	devicetrustv1connect.UnimplementedDeviceTrustServiceHandler
	logger *slog.Logger
	// devicesClient is a client to auth's Device Trust service authenticated as the proxy.
	devicesClient devicepb.DeviceTrustServiceClient
}

func (s *deviceTrustServer) CreateDeviceEnrollToken(ctx context.Context, req *connect.Request[devicepb.CreateDeviceEnrollTokenRequest]) (*connect.Response[devicepb.DeviceEnrollToken], error) {
	token, err := s.devicesClient.CreateDeviceEnrollToken(ctx, req.Msg)
	return connect.NewResponse(token), trace.Wrap(err)
}

func (s *deviceTrustServer) EnrollDevice(ctx context.Context, clientStream *connect.BidiStream[devicepb.EnrollDeviceRequest, devicepb.EnrollDeviceResponse]) error {
	s.logger.InfoContext(ctx, "EnrollDevice has started")
	defer s.logger.InfoContext(ctx, "EnrollDevice has ended")
	serverStream, err := s.devicesClient.EnrollDevice(ctx)
	if err != nil {
		return trace.Wrap(err, "starting server stream")
	}

	errChan := make(chan error, 2)

	// Forward messages from client to server.
	go func() {
		defer s.logger.InfoContext(ctx, "Finished forwarding from client to server")
		defer func() {
			// CloseSend always returns nil error.
			_ = serverStream.CloseSend()
		}()

		for {
			s.logger.InfoContext(ctx, "Waiting for client message")
			clientMsg, err := clientStream.Receive()
			s.logger.InfoContext(ctx, "Got client message", "message", clientMsg, "error", err)
			if err != nil {
				if errors.Is(err, io.EOF) {
					// Client is done sending messages.
					errChan <- nil
					return
				}
				errChan <- trace.Wrap(err, "receiving message from client")
				return
			}

			if err := serverStream.Send(clientMsg); err != nil {
				errChan <- trace.Wrap(err, "sending message from client to server")
				return
			}
		}
	}()

	// Forward messages from server to client.
	go func() {
		defer s.logger.InfoContext(ctx, "Finished forwarding from server to client")
		for {
			s.logger.InfoContext(ctx, "Waiting for server message")
			serverMsg, err := serverStream.Recv()
			s.logger.InfoContext(ctx, "Got server message", "message", serverMsg, "error", err)
			if err != nil {
				if errors.Is(err, io.EOF) {
					// Server stream has terminated with an OK status.
					errChan <- nil
					return
				}
				// Do not add a message to trace.Wrap here. If the server returns an error in response to a
				// message from the client, the error needs to be proxied with no changes to its structure.
				// Any message added to trace.Wrap here would be appended to the error delivered to the
				// client.
				errChan <- trace.Wrap(err)
				return
			}

			if err := clientStream.Send(serverMsg); err != nil {
				errChan <- trace.Wrap(err, "sending message from server to client")
				return
			}
		}
	}()

	// Return immediately on the first value. Since we're within a handler for a bidi stream,
	// returning an error from the handler is the only way of passing the error back to the client.
	return trace.Wrap(<-errChan)
}

func (s *deviceTrustServer) Ping(ctx context.Context, req *connect.Request[devicepb.PingRequest]) (*connect.Response[devicepb.PingResponse], error) {
	return connect.NewResponse(&devicepb.PingResponse{}), nil
}
