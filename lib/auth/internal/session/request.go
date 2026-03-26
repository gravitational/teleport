/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package session

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/tlsca"
)

// NewWebSessionRequest defines a request to create a new user web session.
type NewWebSessionRequest struct {
	// User specifies the user this session is bound to.
	User string
	// LoginIP is an observed IP of the client, it will be embedded into certificates.
	LoginIP string
	// LoginUserAgent is the user agent of the client's browser, as captured by the Proxy.
	LoginUserAgent string
	// LoginMaxTouchPoints indicates whether the client device supports touch controls.
	LoginMaxTouchPoints int
	// ProxyGroupID is the proxy group id where request is generated.
	ProxyGroupID string
	// Roles optionally lists additional user roles.
	Roles []string
	// Traits optionally lists role traits.
	Traits map[string][]string
	// SessionTTL optionally specifies the session time-to-live.
	SessionTTL time.Duration
	// LoginTime is the time that this user recently logged in.
	LoginTime time.Time
	// AccessRequests contains the UUIDs of the access requests currently in use.
	AccessRequests []string
	// RequestedResourceAccessIDs optionally lists requested resources.
	RequestedResourceAccessIDs []types.ResourceAccessID
	// AttestWebSession optionally attests the web session to meet private key policy requirements.
	AttestWebSession bool
	// SSHPrivateKey is a specific private key to use when generating the web sessions' SSH certificates.
	SSHPrivateKey *keys.PrivateKey
	// TLSPrivateKey is a specific private key to use when generating the web sessions' SSH certificates.
	TLSPrivateKey *keys.PrivateKey
	// CreateDeviceWebToken informs Auth to issue a DeviceWebToken when creating this session.
	CreateDeviceWebToken bool
	// Scope, if non-empty, makes the authentication scoped.
	Scope string
	// DelegationSessionID is the ID of the Delegation Session this session is
	// being created for.
	DelegationSessionID string
}

// CheckAndSetDefaults validates the request and sets defaults.
func (r *NewWebSessionRequest) CheckAndSetDefaults() error {
	if r.User == "" {
		return trace.BadParameter("user name required")
	}
	if len(r.Roles) == 0 {
		return trace.BadParameter("roles required")
	}
	if len(r.Traits) == 0 {
		return trace.BadParameter("traits required")
	}
	if r.SessionTTL == 0 {
		r.SessionTTL = defaults.CertDuration
	}
	return nil
}

// NewAppSessionRequest defines a request to create a new user app session.
type NewAppSessionRequest struct {
	NewWebSessionRequest

	// PublicAddr is the public address the application.
	PublicAddr string
	// ClusterName is cluster within which the application is running.
	ClusterName string
	// AWSRoleARN is AWS role the user wants to assume.
	AWSRoleARN string
	// AzureIdentity is Azure identity the user wants to assume.
	AzureIdentity string
	// GCPServiceAccount is the GCP service account the user wants to assume.
	GCPServiceAccount string
	// MFAVerified is the UUID of an MFA device used to verify this request.
	MFAVerified string
	// DeviceExtensions holds device-aware user certificate extensions.
	DeviceExtensions tlsca.DeviceExtensions
	// AppName is the name of the app.
	AppName string
	// AppURI is the URI of the app. This is the internal endpoint where the application is running and isn't user-facing.
	AppURI string
	// AppTargetPort signifies that the session is made to a specific port of a multi-port TCP app.
	AppTargetPort int
	// Identity is the identity of the user.
	Identity tlsca.Identity
	// ClientAddr is a client (user's) address.
	ClientAddr string
	// SuggestedSessionID is a session ID suggested by the requester.
	SuggestedSessionID string
	// BotName is the name of the bot that is creating this session.
	BotName string
	// BotInstanceID is the ID of the bot instance that is creating this session.
	BotInstanceID string
}
