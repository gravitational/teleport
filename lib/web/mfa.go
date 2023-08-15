/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package web

import (
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth/webauthn"
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
	WebauthnRegisterResponse *webauthn.CredentialCreationResponse `json:"webauthnRegisterResponse"`
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
			Webauthn: webauthn.CredentialCreationResponseToProto(req.WebauthnRegisterResponse),
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

// createAuthenticateChallengeHandle creates and returns MFA authentication challenges for the user in context (logged in user).
// Used when users need to re-authenticate their second factors.
func (h *Handler) createAuthenticateChallengeHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	chal, err := clt.CreateAuthenticateChallenge(r.Context(), &proto.CreateAuthenticateChallengeRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.MakeAuthenticateChallenge(chal), nil
}

// createAuthenticateChallengeWithTokenHandle creates and returns MFA authenticate challenges for the user defined in token.
func (h *Handler) createAuthenticateChallengeWithTokenHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	chal, err := h.cfg.ProxyClient.CreateAuthenticateChallenge(r.Context(), &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_RecoveryStartTokenID{RecoveryStartTokenID: p.ByName("token")},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.MakeAuthenticateChallenge(chal), nil
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

	return client.MakeRegisterChallenge(chal), nil
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

type isMFARequiredRequest struct {
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
}

func (r *isMFARequiredRequest) checkAndGetProtoRequest() (*proto.IsMFARequiredRequest, error) {
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
				}},
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
				}},
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
				}},
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

func (h *Handler) isMFARequired(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	var httpReq *isMFARequiredRequest
	if err := httplib.ReadJSON(r, &httpReq); err != nil {
		return nil, trace.Wrap(err)
	}

	protoReq, err := httpReq.checkAndGetProtoRequest()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := clt.IsMFARequired(r.Context(), protoReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return isMfaRequiredResponse{Required: res.GetRequired()}, nil
}
