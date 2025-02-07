/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web/ui"
)

// getMFADevicesWithTokenHandle retrieves the list of registered MFA devices for the user defined in token.
func (h *Handler) getMFADevicesWithTokenHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	mfas, err := h.cfg.ProxyClient.GetMFADevices(r.Context(), &proto.GetMFADevicesRequest{
		TokenID: p.ByName("token"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeMFADevices(mfas.GetDevices()), nil
}

// getMFADevicesHandle retrieves the list of registered MFA devices for the user in context (logged in user).
func (h *Handler) getMFADevicesHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mfas, err := clt.GetMFADevices(r.Context(), &proto.GetMFADevicesRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeMFADevices(mfas.GetDevices()), nil
}

// deleteMFADeviceWithTokenHandle deletes a mfa device for the user defined in the `token`, given as a query parameter.
func (h *Handler) deleteMFADeviceWithTokenHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	if err := h.GetProxyClient().DeleteMFADeviceSync(r.Context(), &proto.DeleteMFADeviceSyncRequest{
		TokenID:    p.ByName("token"),
		DeviceName: p.ByName("devicename"),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

type addMFADeviceRequest struct {
	// PrivilegeTokenID is privilege token id.
	PrivilegeTokenID string `json:"tokenId"`
	// DeviceName is the name of new mfa device.
	DeviceName string `json:"deviceName"`
	// SecondFactorToken is the totp code.
	SecondFactorToken string `json:"secondFactorToken"`
	// WebauthnRegisterResponse is a WebAuthn registration challenge response.
	WebauthnRegisterResponse *wantypes.CredentialCreationResponse `json:"webauthnRegisterResponse"`
	// DeviceUsage is the intended usage of the device (MFA, Passwordless, etc).
	// It mimics the proto.DeviceUsage enum.
	// Defaults to MFA.
	DeviceUsage string `json:"deviceUsage"`
}

// addMFADeviceHandle adds a new mfa device for the user defined in the token.
func (h *Handler) addMFADeviceHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req addMFADeviceRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	deviceUsage, err := getDeviceUsage(req.DeviceUsage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	protoReq := &proto.AddMFADeviceSyncRequest{
		TokenID:       req.PrivilegeTokenID,
		NewDeviceName: req.DeviceName,
		DeviceUsage:   deviceUsage,
	}

	switch {
	case req.SecondFactorToken != "":
		protoReq.NewMFAResponse = &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
			TOTP: &proto.TOTPRegisterResponse{Code: req.SecondFactorToken},
		}}
	case req.WebauthnRegisterResponse != nil:
		protoReq.NewMFAResponse = &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_Webauthn{
			Webauthn: wantypes.CredentialCreationResponseToProto(req.WebauthnRegisterResponse),
		}}
	default:
		return nil, trace.BadParameter("missing new mfa credentials")
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := clt.AddMFADeviceSync(r.Context(), protoReq); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

type CreateAuthenticateChallengeRequest struct {
	IsMFARequiredRequest        *IsMFARequiredRequest `json:"is_mfa_required_req"`
	ChallengeScope              int                   `json:"challenge_scope"`
	ChallengeAllowReuse         bool                  `json:"challenge_allow_reuse"`
	UserVerificationRequirement string                `json:"user_verification_requirement"`
}

// createAuthenticateChallengeHandle creates and returns MFA authentication challenges for the user in context (logged in user).
// Used when users need to re-authenticate their second factors.
func (h *Handler) createAuthenticateChallengeHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *SessionContext) (interface{}, error) {
	ctx := r.Context()

	var req CreateAuthenticateChallengeRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var mfaRequiredCheckProto *proto.IsMFARequiredRequest
	if req.IsMFARequiredRequest != nil {
		mfaRequiredCheckProto, err = h.checkAndGetProtoRequest(ctx, c, req.IsMFARequiredRequest)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// If this is an mfa required check for a leaf host, we need to check the requirement through
		// the leaf cluster, rather than through root in the authenticate challenge request below
		//
		// TODO(Joerger): Currently, the only leafs hosts that we check mfa requirements for directly
		// are apps. If we need to check other hosts directly, rather than through websocket flow,
		// we'll need to include their clusterID in the request like we do for apps.
		appReq := mfaRequiredCheckProto.GetApp()
		if appReq != nil && appReq.ClusterName != c.cfg.RootClusterName {
			site, err := h.getSiteByClusterName(c, appReq.ClusterName)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			clusterClient, err := c.GetUserClient(ctx, site)
			if err != nil {
				return false, trace.Wrap(err)
			}

			res, err := clusterClient.IsMFARequired(ctx, mfaRequiredCheckProto)
			if err != nil {
				return false, trace.Wrap(err)
			}

			if !res.Required {
				return &client.MFAAuthenticateChallenge{}, nil
			}

			// We don't want to check again through the root cluster below.
			mfaRequiredCheckProto = nil
		}
	}

	allowReuse := mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO
	if req.ChallengeAllowReuse {
		allowReuse = mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES
	}

	chal, err := clt.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
			ContextUser: &proto.ContextUser{},
		},
		MFARequiredCheck: mfaRequiredCheckProto,
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope:                       mfav1.ChallengeScope(req.ChallengeScope),
			AllowReuse:                  allowReuse,
			UserVerificationRequirement: req.UserVerificationRequirement,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeAuthenticateChallenge(chal), nil
}

// createAuthenticateChallengeWithTokenHandle creates and returns MFA authenticate challenges for the user defined in token.
func (h *Handler) createAuthenticateChallengeWithTokenHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	chal, err := h.cfg.ProxyClient.CreateAuthenticateChallenge(r.Context(), &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_RecoveryStartTokenID{RecoveryStartTokenID: p.ByName("token")},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeAuthenticateChallenge(chal), nil
}

type createRegisterChallengeRequest struct {
	// DeviceType is the type of MFA device to get a register challenge for.
	DeviceType string `json:"deviceType"`
	// DeviceUsage is the intended usage of the device (MFA, Passwordless, etc).
	// It mimics the proto.DeviceUsage enum.
	// Defaults to MFA.
	DeviceUsage string `json:"deviceUsage"`
}

// createRegisterChallengeWithTokenHandle creates and returns MFA register challenges for a new device for the specified device type.
func (h *Handler) createRegisterChallengeWithTokenHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req createRegisterChallengeRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	var deviceType proto.DeviceType
	switch req.DeviceType {
	case "totp":
		deviceType = proto.DeviceType_DEVICE_TYPE_TOTP
	case "webauthn":
		deviceType = proto.DeviceType_DEVICE_TYPE_WEBAUTHN
	default:
		return nil, trace.BadParameter("MFA device type %q unsupported", req.DeviceType)
	}

	deviceUsage, err := getDeviceUsage(req.DeviceUsage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	chal, err := h.cfg.ProxyClient.CreateRegisterChallenge(r.Context(), &proto.CreateRegisterChallengeRequest{
		TokenID:     p.ByName("token"),
		DeviceType:  deviceType,
		DeviceUsage: deviceUsage,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &client.MFARegisterChallenge{}
	switch chal.GetRequest().(type) {
	case *proto.MFARegisterChallenge_TOTP:
		resp.TOTP = &client.TOTPRegisterChallenge{
			QRCode: chal.GetTOTP().GetQRCode(),
		}
	case *proto.MFARegisterChallenge_Webauthn:
		resp.Webauthn = wantypes.CredentialCreationFromProto(chal.GetWebauthn())
	}

	return resp, nil
}

func getDeviceUsage(reqUsage string) (proto.DeviceUsage, error) {
	var deviceUsage proto.DeviceUsage
	switch strings.ToLower(reqUsage) {
	case "", "mfa":
		deviceUsage = proto.DeviceUsage_DEVICE_USAGE_MFA
	case "passwordless":
		deviceUsage = proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS
	default:
		return proto.DeviceUsage_DEVICE_USAGE_UNSPECIFIED, trace.BadParameter("device usage %q unsupported", reqUsage)
	}

	return deviceUsage, nil
}

type isMFARequiredDatabase struct {
	// ServiceName is the database service name.
	ServiceName string `json:"service_name"`
	// Protocol is the type of the database protocol
	// eg: "postgres", "mysql", "mongodb", etc.
	Protocol string `json:"protocol"`
	// Username is an optional database username.
	Username string `json:"username,omitempty"`
	// DatabaseName is an optional database name.
	DatabaseName string `json:"database_name,omitempty"`
}

type isMFARequiredKube struct {
	// ClusterName is the name of the kube cluster.
	ClusterName string `json:"cluster_name"`
}

type isMFARequiredNode struct {
	// NodeName can be node's hostname or UUID.
	NodeName string `json:"node_name"`
	// Login is the OS login name.
	Login string `json:"login"`
}

type isMFARequiredWindowsDesktop struct {
	// DesktopName is the Windows Desktop server name.
	DesktopName string `json:"desktop_name"`
	// Login is the Windows desktop user login.
	Login string `json:"login"`
}

type IsMFARequiredApp struct {
	// ResolveAppParams contains info used to resolve an application
	ResolveAppParams
}

type isMFARequiredAdminAction struct{}

type IsMFARequiredRequest struct {
	// Database contains fields required to check if target database
	// requires MFA check.
	Database *isMFARequiredDatabase `json:"database,omitempty"`
	// Node contains fields required to check if target node
	// requires MFA check.
	Node *isMFARequiredNode `json:"node,omitempty"`
	// WindowsDesktop contains fields required to check if target
	// windows desktop requires MFA check.
	WindowsDesktop *isMFARequiredWindowsDesktop `json:"windows_desktop,omitempty"`
	// Kube is the name of the kube cluster to check if target cluster
	// requires MFA check.
	Kube *isMFARequiredKube `json:"kube,omitempty"`
	// App contains fields required to resolve an application and check if
	// the target application requires MFA check.
	App *IsMFARequiredApp `json:"app,omitempty"`
	// AdminAction is the name of the admin action RPC to check if MFA is required.
	AdminAction *isMFARequiredAdminAction `json:"admin_action,omitempty"`
}

func (h *Handler) checkAndGetProtoRequest(ctx context.Context, scx *SessionContext, r *IsMFARequiredRequest) (*proto.IsMFARequiredRequest, error) {
	numRequests := 0
	var protoReq *proto.IsMFARequiredRequest

	if r.Database != nil {
		numRequests++
		if r.Database.ServiceName == "" {
			return nil, trace.BadParameter("missing service_name for checking database target")
		}
		if r.Database.Protocol == "" {
			return nil, trace.BadParameter("missing protocol for checking database target")
		}

		protoReq = &proto.IsMFARequiredRequest{
			Target: &proto.IsMFARequiredRequest_Database{
				Database: &proto.RouteToDatabase{
					ServiceName: r.Database.ServiceName,
					Protocol:    r.Database.Protocol,
					Database:    r.Database.DatabaseName,
					Username:    r.Database.Username,
				},
			},
		}
	}

	if r.Kube != nil {
		numRequests++
		if r.Kube.ClusterName == "" {
			return nil, trace.BadParameter("missing cluster_name for checking kubernetes cluster target")
		}

		protoReq = &proto.IsMFARequiredRequest{
			Target: &proto.IsMFARequiredRequest_KubernetesCluster{
				KubernetesCluster: r.Kube.ClusterName,
			},
		}
	}

	if r.WindowsDesktop != nil {
		numRequests++
		if r.WindowsDesktop.DesktopName == "" {
			return nil, trace.BadParameter("missing desktop_name for checking windows desktop target")
		}
		if r.WindowsDesktop.Login == "" {
			return nil, trace.BadParameter("missing login for checking windows desktop target")
		}

		protoReq = &proto.IsMFARequiredRequest{
			Target: &proto.IsMFARequiredRequest_WindowsDesktop{
				WindowsDesktop: &proto.RouteToWindowsDesktop{
					WindowsDesktop: r.WindowsDesktop.DesktopName,
					Login:          r.WindowsDesktop.Login,
				},
			},
		}
	}

	if r.Node != nil {
		numRequests++
		if r.Node.Login == "" {
			return nil, trace.BadParameter("missing login for checking node target")
		}
		if r.Node.NodeName == "" {
			return nil, trace.BadParameter("missing node_name for checking node target")
		}

		protoReq = &proto.IsMFARequiredRequest{
			Target: &proto.IsMFARequiredRequest_Node{
				Node: &proto.NodeLogin{
					Login: r.Node.Login,
					Node:  r.Node.NodeName,
				},
			},
		}
	}

	if r.App != nil {
		resolvedApp, err := h.resolveApp(ctx, scx, r.App.ResolveAppParams)
		if err != nil {
			return nil, trace.Wrap(err, "unable to resolve FQDN: %v", r.App.FQDNHint)
		}

		numRequests++
		protoReq = &proto.IsMFARequiredRequest{
			Target: &proto.IsMFARequiredRequest_App{
				App: &proto.RouteToApp{
					Name:        resolvedApp.App.GetName(),
					PublicAddr:  resolvedApp.App.GetPublicAddr(),
					ClusterName: resolvedApp.ClusterName,
				},
			},
		}
	}

	if r.AdminAction != nil {
		numRequests++
		protoReq = &proto.IsMFARequiredRequest{
			Target: &proto.IsMFARequiredRequest_AdminAction{
				AdminAction: &proto.AdminAction{},
			},
		}
	}

	if numRequests > 1 {
		return nil, trace.BadParameter("only one target is allowed for MFA check")
	}

	if protoReq == nil {
		return nil, trace.BadParameter("missing target for MFA check")
	}

	return protoReq, nil
}

type isMfaRequiredResponse struct {
	Required bool `json:"required"`
}

// isMFARequired is the [ClusterHandler] implementer for checking if MFA is required for a given target.
func (h *Handler) isMFARequired(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	var httpReq *IsMFARequiredRequest
	if err := httplib.ReadJSON(r, &httpReq); err != nil {
		return nil, trace.Wrap(err)
	}

	required, err := h.checkMFARequired(r.Context(), httpReq, sctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return isMfaRequiredResponse{Required: required}, nil
}

// checkMFARequired checks if MFA is required for the target specified in the [isMFARequiredRequest].
func (h *Handler) checkMFARequired(ctx context.Context, req *IsMFARequiredRequest, sctx *SessionContext, site reversetunnelclient.RemoteSite) (bool, error) {
	protoReq, err := h.checkAndGetProtoRequest(ctx, sctx, req)
	if err != nil {
		return false, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return false, trace.Wrap(err)
	}

	res, err := clt.IsMFARequired(ctx, protoReq)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return res.GetRequired(), nil
}

// makeAuthenticateChallenge converts proto to JSON format.
func makeAuthenticateChallenge(protoChal *proto.MFAAuthenticateChallenge) *client.MFAAuthenticateChallenge {
	chal := &client.MFAAuthenticateChallenge{
		TOTPChallenge: protoChal.GetTOTP() != nil,
	}
	if protoChal.GetWebauthnChallenge() != nil {
		chal.WebauthnChallenge = wantypes.CredentialAssertionFromProto(protoChal.WebauthnChallenge)
	}
	return chal
}
