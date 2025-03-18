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

package client

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	websession "github.com/gravitational/teleport/lib/web/session"
)

const (
	// HTTPS is https prefix
	HTTPS = "https"
	// WSS is secure web sockets prefix
	WSS = "wss"
)

// SSOLoginConsoleReq is passed by tsh to authenticate an SSO user and receive
// short-lived certificates.
type SSOLoginConsoleReq struct {
	RedirectURL string `json:"redirect_url"`
	SSOUserPublicKeys
	CertTTL       time.Duration `json:"cert_ttl"`
	ConnectorID   string        `json:"connector_id"`
	Compatibility string        `json:"compatibility,omitempty"`
	// RouteToCluster is an optional cluster name to route the response
	// credentials to.
	RouteToCluster string
	// KubernetesCluster is an optional k8s cluster name to route the response
	// credentials to.
	KubernetesCluster string
}

// CheckAndSetDefaults makes sure that the request is valid
func (r *SSOLoginConsoleReq) CheckAndSetDefaults() error {
	switch {
	case r.RedirectURL == "":
		return trace.BadParameter("missing RedirectURL")
	case r.ConnectorID == "":
		return trace.BadParameter("missing ConnectorID")
	}
	if err := r.SSOUserPublicKeys.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SSOLoginConsoleResponse is a response to SSO console request
type SSOLoginConsoleResponse struct {
	RedirectURL string `json:"redirect_url"`
}

// MFAChallengeRequest is a request from the client for a MFA challenge from the
// server.
type MFAChallengeRequest struct {
	User string `json:"user"`
	Pass string `json:"pass"`
	// Passwordless explicitly requests a passwordless/usernameless challenge.
	Passwordless bool `json:"passwordless"`
}

// MFAChallengeResponse holds the response to a MFA challenge.
type MFAChallengeResponse struct {
	// TOTPCode is a code for a otp device.
	TOTPCode string `json:"totp_code,omitempty"`
	// WebauthnResponse is a response from a webauthn device.
	WebauthnResponse *wantypes.CredentialAssertionResponse `json:"webauthn_response,omitempty"`
	// SSOResponse is a response from an SSO MFA flow.
	SSOResponse *SSOResponse `json:"sso_response"`
	// TODO(Joerger): DELETE IN v19.0.0, WebauthnResponse used instead.
	WebauthnAssertionResponse *wantypes.CredentialAssertionResponse `json:"webauthnAssertionResponse"`
}

// SSOResponse is a json compatible [proto.SSOResponse].
type SSOResponse struct {
	RequestID string `json:"requestId,omitempty"`
	Token     string `json:"token,omitempty"`
}

// GetOptionalMFAResponseProtoReq converts response to a type proto.MFAAuthenticateResponse,
// if there were any responses set. Otherwise returns nil.
func (r *MFAChallengeResponse) GetOptionalMFAResponseProtoReq() (*proto.MFAAuthenticateResponse, error) {
	var availableResponses int
	if r.TOTPCode != "" {
		availableResponses++
	}
	if r.WebauthnResponse != nil {
		availableResponses++
	}
	if r.SSOResponse != nil {
		availableResponses++
	}

	if availableResponses > 1 {
		return nil, trace.BadParameter("only one MFA response field can be set")
	}

	switch {
	case r.WebauthnResponse != nil:
		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wantypes.CredentialAssertionResponseToProto(r.WebauthnResponse),
		}}, nil
	case r.SSOResponse != nil:
		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_SSO{
			SSO: &proto.SSOResponse{
				RequestId: r.SSOResponse.RequestID,
				Token:     r.SSOResponse.Token,
			},
		}}, nil
	case r.TOTPCode != "":
		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{Code: r.TOTPCode},
		}}, nil
	case r.WebauthnAssertionResponse != nil:
		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wantypes.CredentialAssertionResponseToProto(r.WebauthnAssertionResponse),
		}}, nil
	}

	return nil, nil
}

// ParseMFAChallengeResponse parses [MFAChallengeResponse] from JSON and returns it as a [proto.MFAAuthenticateResponse].
func ParseMFAChallengeResponse(mfaResponseJSON []byte) (*proto.MFAAuthenticateResponse, error) {
	var resp MFAChallengeResponse
	if err := json.Unmarshal(mfaResponseJSON, &resp); err != nil {
		return nil, trace.Wrap(err)
	}

	protoResp, err := resp.GetOptionalMFAResponseProtoReq()
	return protoResp, trace.Wrap(err)
}

// CreateSSHCertReq is passed by tsh to authenticate a local user without MFA
// and receive short-lived certificates.
type CreateSSHCertReq struct {
	// User is a teleport username
	User string `json:"user"`
	// Password is user's pass
	Password string `json:"password"`
	// OTPToken is second factor token
	OTPToken string `json:"otp_token"`
	// HeadlessAuthenticationID is a headless authentication resource id.
	HeadlessAuthenticationID string `json:"headless_id"`
	// UserPublicKeys is embedded and holds user SSH and TLS public keys that
	// should be used as the subject of issued certificates, and optional
	// hardware key attestation statements for each key.
	UserPublicKeys
	TTL time.Duration `json:"ttl"`
	// Compatibility specifies OpenSSH compatibility flags.
	Compatibility string `json:"compatibility,omitempty"`
	// RouteToCluster is an optional cluster name to route the response
	// credentials to.
	RouteToCluster string
	// KubernetesCluster is an optional k8s cluster name to route the response
	// credentials to.
	KubernetesCluster string
}

// CheckAndSetDefaults checks and sets default values.
func (r *CreateSSHCertReq) CheckAndSetDefaults() error {
	return trace.Wrap(r.UserPublicKeys.CheckAndSetDefaults())
}

// HeadlessLoginReq is a headless login request for /webapi/headless/login.
type HeadlessLoginReq struct {
	// User is a teleport username
	User string `json:"user"`
	// HeadlessAuthenticationID is a headless authentication resource id.
	HeadlessAuthenticationID string `json:"headless_id"`
	// UserPublicKeys is embedded and holds user SSH and TLS public keys that
	// should be used as the subject of issued certificates, and optional
	// hardware key attestation statements for each key.
	UserPublicKeys
	TTL time.Duration `json:"ttl"`
	// Compatibility specifies OpenSSH compatibility flags.
	Compatibility string `json:"compatibility,omitempty"`
	// RouteToCluster is an optional cluster name to route the response
	// credentials to.
	RouteToCluster string
	// KubernetesCluster is an optional k8s cluster name to route the response
	// credentials to.
	KubernetesCluster string
}

// CheckAndSetDefaults checks and sets default values.
func (r *HeadlessLoginReq) CheckAndSetDefaults() error {
	if r.HeadlessAuthenticationID == "" {
		return trace.BadParameter("missing headless authentication id for headless login")
	}

	return trace.Wrap(r.UserPublicKeys.CheckAndSetDefaults())
}

// UserPublicKeys holds user-submitted public keys and attestation statements
// used in local login requests.
type UserPublicKeys struct {
	// PubKey is a public key the user wants as the subject of their SSH and TLS
	// certificates. It must be in SSH authorized_keys format.
	//
	// Deprecated: prefer SSHPubKey and/or TLSPubKey.
	// TODO(nklaassen): DELETE IN 18.0.0 when all clients should be using
	// separate keys.
	PubKey []byte `json:"pub_key,omitempty"`
	// SSHPubKey is an SSH public key the user wants as the subject of their SSH
	// certificate. It must be in SSH authorized_keys format.
	SSHPubKey []byte `json:"ssh_pub_key,omitempty"`
	// TLSPubKey is a TLS public key the user wants as the subject of their TLS
	// certificate. It must be in PEM-encoded PKCS#1 or PKIX format.
	TLSPubKey []byte `json:"tls_pub_key,omitempty"`

	// AttestationStatement is an attestation statement associated with the given public key.
	//
	// Deprecated: prefer SSHAttestationStatement and/or TLSAttestationStatement.
	// TODO(nklaassen): DELETE IN 18.0.0 when all clients should be using
	// separate keys.
	AttestationStatement *keys.AttestationStatement `json:"attestation_statement,omitempty"`
	// SSHAttestationStatement is an attestation statement associated with the
	// given SSH public key.
	SSHAttestationStatement *keys.AttestationStatement `json:"ssh_attestation_statement,omitempty"`
	// TLSAttestationStatement is an attestation statement associated with the
	// given TLS public key.
	TLSAttestationStatement *keys.AttestationStatement `json:"tls_attestation_statement,omitempty"`
}

// CheckAndSetDefaults checks and sets default values.
func (k *UserPublicKeys) CheckAndSetDefaults() error {
	switch {
	case len(k.PubKey) > 0 && len(k.SSHPubKey) > 0:
		return trace.BadParameter("'pub_key' and 'ssh_pub_key' cannot both be set")
	case len(k.PubKey) > 0 && len(k.TLSPubKey) > 0:
		return trace.BadParameter("'pub_key' and 'tls_pub_key' cannot both be set")
	case len(k.PubKey)+len(k.SSHPubKey)+len(k.TLSPubKey) == 0:
		return trace.BadParameter("'ssh_pub_key' or 'tls_pub_key' must be set")
	case k.AttestationStatement != nil && k.SSHAttestationStatement != nil:
		return trace.BadParameter("'attestation_statement' and 'ssh_attestation_statement' cannot both be set")
	case k.AttestationStatement != nil && k.TLSAttestationStatement != nil:
		return trace.BadParameter("'attestation_statement' and 'tls_attestation_statement' cannot both be set")
	}
	var err error
	k.SSHPubKey, k.TLSPubKey, err = authclient.UserPublicKeys(k.PubKey, k.SSHPubKey, k.TLSPubKey)
	if err != nil {
		return trace.Wrap(err)
	}
	k.SSHAttestationStatement, k.TLSAttestationStatement = authclient.UserAttestationStatements(k.AttestationStatement, k.SSHAttestationStatement, k.TLSAttestationStatement)
	k.PubKey = nil
	k.AttestationStatement = nil
	return nil
}

// SSOUserPublicKeys holds user-submitted public keys and attestation statements
// used in SSO login requests. This is identical to UserPublicKeys except for
// the JSON tag on PublicKey, which is deprecated.
//
// TODO(nklaassen): DELETE IN 18.0.0 and replace with UserPublicKeys.
type SSOUserPublicKeys struct {
	// PublicKey is a public key the user wants as the subject of their SSH and TLS
	// certificates. It must be in SSH authorized_keys format.
	//
	// Deprecated: prefer SSHPubKey and/or TLSPubKey.
	PublicKey []byte `json:"public_key,omitempty"`
	// SSHPubKey is an SSH public key the user wants as the subject of their SSH
	// certificate. It must be in SSH authorized_keys format.
	SSHPubKey []byte `json:"ssh_pub_key,omitempty"`
	// TLSPubKey is a TLS public key the user wants as the subject of their TLS
	// certificate. It must be in PEM-encoded PKCS#1 or PKIX format.
	TLSPubKey []byte `json:"tls_pub_key,omitempty"`

	// AttestationStatement is an attestation statement associated with the given public key.
	//
	// Deprecated: prefer SSHAttestationStatement and/or TLSAttestationStatement.
	AttestationStatement *keys.AttestationStatement `json:"attestation_statement,omitempty"`
	// SSHAttestationStatement is an attestation statement associated with the
	// given SSH public key.
	SSHAttestationStatement *keys.AttestationStatement `json:"ssh_attestation_statement,omitempty"`
	// TLSAttestationStatement is an attestation statement associated with the
	// given TLS public key.
	TLSAttestationStatement *keys.AttestationStatement `json:"tls_attestation_statement,omitempty"`
}

// CheckAndSetDefaults checks and sets default values.
func (k *SSOUserPublicKeys) CheckAndSetDefaults() error {
	userPublicKeys := UserPublicKeys{
		PubKey:                  k.PublicKey,
		SSHPubKey:               k.SSHPubKey,
		TLSPubKey:               k.TLSPubKey,
		AttestationStatement:    k.AttestationStatement,
		SSHAttestationStatement: k.SSHAttestationStatement,
		TLSAttestationStatement: k.TLSAttestationStatement,
	}
	if err := userPublicKeys.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	*k = SSOUserPublicKeys{
		PublicKey:               userPublicKeys.PubKey,
		SSHPubKey:               userPublicKeys.SSHPubKey,
		TLSPubKey:               userPublicKeys.TLSPubKey,
		AttestationStatement:    userPublicKeys.AttestationStatement,
		SSHAttestationStatement: userPublicKeys.SSHAttestationStatement,
		TLSAttestationStatement: userPublicKeys.TLSAttestationStatement,
	}
	return nil
}

// AuthenticateSSHUserRequest is passed by tsh to authenticate a local user with
// MFA and receive short-lived certificates.
type AuthenticateSSHUserRequest struct {
	// User is a teleport username
	User string `json:"user"`
	// Password for the user, to authenticate in case no MFA check was
	// performed.
	Password string `json:"password"`
	// WebauthnChallengeResponse is a signed WebAuthn credential assertion.
	WebauthnChallengeResponse *wantypes.CredentialAssertionResponse `json:"webauthn_challenge_response"`
	// TOTPCode is a code from the TOTP device.
	TOTPCode string `json:"totp_code"`
	// UserPublicKeys is embedded and holds user SSH and TLS public keys that
	// should be used as the subject of issued certificates, and optional
	// hardware key attestation statements for each key.
	UserPublicKeys
	// TTL is a desired TTL for the cert (max is still capped by server,
	// however user can shorten the time)
	TTL time.Duration `json:"ttl"`
	// Compatibility specifies OpenSSH compatibility flags.
	Compatibility string `json:"compatibility,omitempty"`
	// RouteToCluster is an optional cluster name to route the response
	// credentials to.
	RouteToCluster string
	// KubernetesCluster is an optional k8s cluster name to route the response
	// credentials to.
	KubernetesCluster string
}

func (r *AuthenticateSSHUserRequest) CheckAndSetDefaults() error {
	return trace.Wrap(r.UserPublicKeys.CheckAndSetDefaults())
}

type AuthenticateWebUserRequest struct {
	// User is a teleport username.
	User string `json:"user"`
	// WebauthnAssertionResponse is a signed WebAuthn credential assertion.
	WebauthnAssertionResponse *wantypes.CredentialAssertionResponse `json:"webauthnAssertionResponse,omitempty"`
}

type HeadlessRequest struct {
	// Actions can be either accept or deny.
	Action string `json:"action"`
	// MFAResponse is an MFA response used to authenticate the headless request.
	MFAResponse *MFAChallengeResponse `json:"mfaResponse"`
	// WebauthnAssertionResponse is a signed WebAuthn credential assertion.
	// TODO(Joerger): DELETE IN v19.0.0, new clients send mfaResponse
	WebauthnAssertionResponse *wantypes.CredentialAssertionResponse `json:"webauthnAssertionResponse,omitempty"`
}

// SSHLogin contains common SSH login parameters.
type SSHLogin struct {
	// ProxyAddr is the target proxy address
	ProxyAddr string
	// SSHPubKey is SSH public key to sign
	SSHPubKey []byte
	// TLSPubKey is TLS public key to sign
	TLSPubKey []byte
	// TTL is requested TTL of the client certificates
	TTL time.Duration
	// Insecure turns off verification for x509 target proxy
	Insecure bool
	// Pool is x509 cert pool to use for server certificate verification
	Pool *x509.CertPool
	// Compatibility sets compatibility mode for SSH certificates
	Compatibility string
	// RouteToCluster is an optional cluster name to route the response
	// credentials to.
	RouteToCluster string
	// KubernetesCluster is an optional k8s cluster name to route the response
	// credentials to.
	KubernetesCluster string
	// SSHAttestationStatement is an attestation statement for SSHPubKey.
	SSHAttestationStatement *keys.AttestationStatement
	// TLSAttestationStatement is an attestation statement for TLSPubKey.
	TLSAttestationStatement *keys.AttestationStatement
	// ExtraHeaders is a map of extra HTTP headers to be included in requests.
	ExtraHeaders map[string]string
}

// SSHLoginSSO contains SSH login parameters for SSO login.
type SSHLoginSSO struct {
	SSHLogin
	// ConnectorID is the SSO Auth connector ID to use.
	ConnectorID string
	// ConnectorName is the display name of the SSO Auth connector.
	ConnectorName string
	// ConnectorType is the type of SSO Auth connector.
	ConnectorType string
}

// SSHLoginDirect contains SSH login parameters for direct (user/pass/OTP)
// login.
type SSHLoginDirect struct {
	SSHLogin
	// User is the login username.
	User string
	// User is the login password.
	Password string
	// User is the optional OTP token for the login.
	OTPToken string
}

// SSHLoginMFA contains SSH login parameters for MFA login.
type SSHLoginMFA struct {
	SSHLogin
	// MFAPromptConstructor is a custom MFA prompt constructor to use when prompting for MFA.
	MFAPromptConstructor mfa.PromptConstructor
	// User is the login username.
	User string
	// Password is the login password.
	Password string
}

// SSHLoginPasswordless contains SSH login parameters for passwordless login.
type SSHLoginPasswordless struct {
	SSHLogin

	// WebauthnLogin is a customizable webauthn login function.
	// Defaults to [wancli.Login]
	WebauthnLogin WebauthnLoginFunc

	// StderrOverride will override the default os.Stderr if provided.
	StderrOverride io.Writer

	// User is the login username.
	User string

	// AuthenticatorAttachment is the authenticator attachment for passwordless prompts.
	AuthenticatorAttachment wancli.AuthenticatorAttachment

	// CustomPrompt defines a custom webauthn login prompt.
	// It's an optional field that when nil, it will use the wancli.DefaultPrompt.
	CustomPrompt wancli.LoginPrompt
}

type SSHLoginHeadless struct {
	SSHLogin

	// User is the login username.
	User string

	// HeadlessAuthenticationID is a headless authentication request ID.
	HeadlessAuthenticationID string
}

// MFAAuthenticateChallenge is an MFA authentication challenge sent on user
// login / authentication ceremonies.
type MFAAuthenticateChallenge struct {
	// WebauthnChallenge contains a WebAuthn credential assertion used for
	// login/authentication ceremonies.
	WebauthnChallenge *wantypes.CredentialAssertion `json:"webauthn_challenge"`
	// TOTPChallenge specifies whether TOTP is supported for this user.
	TOTPChallenge bool `json:"totp_challenge"`
	// SSOChallenge is an SSO MFA challenge.
	SSOChallenge *SSOChallenge `json:"sso_challenge"`
}

// SSOChallenge is a json compatible [proto.SSOChallenge].
type SSOChallenge struct {
	RequestID   string        `json:"requestId,omitempty"`
	RedirectURL string        `json:"redirectUrl,omitempty"`
	Device      *SSOMFADevice `json:"device"`
	// ChannelID is used by the front end to differentiate multiple ongoing SSO
	// MFA requests so they don't interfere with each other.
	ChannelID string `json:"channelId"`
}

// SSOMFADevice is a json compatible [proto.SSOMFADevice].
type SSOMFADevice struct {
	ConnectorID   string `json:"connectorId,omitempty"`
	ConnectorType string `json:"connectorType,omitempty"`
	DisplayName   string `json:"displayName,omitempty"`
}

func SSOChallengeFromProto(ssoChal *proto.SSOChallenge) *SSOChallenge {
	return &SSOChallenge{
		RequestID:   ssoChal.RequestId,
		RedirectURL: ssoChal.RedirectUrl,
		Device: &SSOMFADevice{
			ConnectorID:   ssoChal.Device.ConnectorId,
			ConnectorType: ssoChal.Device.ConnectorType,
			DisplayName:   ssoChal.Device.DisplayName,
		},
	}
}

// MFARegisterChallenge is an MFA register challenge sent on new MFA register.
type MFARegisterChallenge struct {
	// Webauthn contains webauthn challenge.
	Webauthn *wantypes.CredentialCreation `json:"webauthn"`
	// TOTP contains TOTP challenge.
	TOTP *TOTPRegisterChallenge `json:"totp"`
}

// TOTPRegisterChallenge contains a TOTP challenge.
type TOTPRegisterChallenge struct {
	QRCode []byte `json:"qrCode"`
}

// initClient creates a new client to the HTTPS web proxy.
func initClient(proxyAddr string, insecure bool, pool *x509.CertPool, extraHeaders map[string]string, opts ...roundtrip.ClientParam) (*WebClient, *url.URL, error) {
	log := slog.With(teleport.ComponentKey, teleport.ComponentClient)
	log.DebugContext(context.Background(), "Initializing proxy HTTPS client",
		"proxy_addr", proxyAddr,
		"insecure", insecure,
		"extra_headers", extraHeaders,
	)

	// validate proxy address
	host, port, err := net.SplitHostPort(proxyAddr)
	if err != nil || host == "" || port == "" {
		if err != nil {
			log.ErrorContext(context.Background(), "invalid proxy address", "error", err)
		}
		return nil, nil, trace.BadParameter("'%v' is not a valid proxy address", proxyAddr)
	}
	proxyAddr = "https://" + net.JoinHostPort(host, port)
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, nil, trace.BadParameter("'%v' is not a valid proxy address", proxyAddr)
	}

	if insecure {
		// Skipping https cert verification, print a warning that this is insecure.
		fmt.Fprintf(os.Stderr, "WARNING: You are using insecure connection to Teleport proxy %v\n", proxyAddr)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	opts = append(opts,
		roundtrip.HTTPClient(newClient(insecure, pool, extraHeaders)),
		roundtrip.CookieJar(jar),
	)
	clt, err := NewWebClient(proxyAddr, opts...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return clt, u, nil
}

// SSHAgentLogin is used by tsh to fetch local user credentials.
func SSHAgentLogin(ctx context.Context, login SSHLoginDirect) (*authclient.SSHLoginResponse, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	re, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "ssh", "certs"), CreateSSHCertReq{
		User:     login.User,
		Password: login.Password,
		OTPToken: login.OTPToken,
		UserPublicKeys: UserPublicKeys{
			SSHPubKey:               login.SSHPubKey,
			TLSPubKey:               login.TLSPubKey,
			SSHAttestationStatement: login.SSHAttestationStatement,
			TLSAttestationStatement: login.TLSAttestationStatement,
		},
		TTL:               login.TTL,
		Compatibility:     login.Compatibility,
		RouteToCluster:    login.RouteToCluster,
		KubernetesCluster: login.KubernetesCluster,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out authclient.SSHLoginResponse
	err = json.Unmarshal(re.Bytes(), &out)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &out, nil
}

// SSHAgentHeadlessLogin begins the headless login ceremony, returning new user certificates if successful.
func SSHAgentHeadlessLogin(ctx context.Context, login SSHLoginHeadless) (*authclient.SSHLoginResponse, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// This request will block until the headless login is approved.
	clt.Client.HTTPClient().Timeout = defaults.HeadlessLoginTimeout

	req := HeadlessLoginReq{
		User:                     login.User,
		HeadlessAuthenticationID: login.HeadlessAuthenticationID,
		UserPublicKeys: UserPublicKeys{
			SSHPubKey:               login.SSHPubKey,
			TLSPubKey:               login.TLSPubKey,
			SSHAttestationStatement: login.SSHAttestationStatement,
			TLSAttestationStatement: login.TLSAttestationStatement,
		},
		TTL:               login.TTL,
		Compatibility:     login.Compatibility,
		RouteToCluster:    login.RouteToCluster,
		KubernetesCluster: login.KubernetesCluster,
	}

	re, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "headless", "login"), req)
	if trace.IsNotFound(err) {
		// fallback to deprecated headless login endpoint
		// TODO(Joerger): DELETE IN v18.0.0
		re, err = clt.PostJSON(ctx, clt.Endpoint("webapi", "ssh", "certs"), req)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out authclient.SSHLoginResponse
	err = json.Unmarshal(re.Bytes(), &out)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &out, nil
}

// SSHAgentPasswordlessLogin requests a passwordless MFA challenge via the proxy.
// weblogin.CustomPrompt (or a default prompt) is used for interaction with the
// end user.
//
// Returns the SSH certificate if authn is successful or an error.
func SSHAgentPasswordlessLogin(ctx context.Context, login SSHLoginPasswordless) (*authclient.SSHLoginResponse, error) {
	webClient, webURL, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challengeJSON, err := webClient.PostJSON(
		ctx, webClient.Endpoint("webapi", "mfa", "login", "begin"),
		&MFAChallengeRequest{
			Passwordless: true,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	challenge := &MFAAuthenticateChallenge{}
	if err := json.Unmarshal(challengeJSON.Bytes(), challenge); err != nil {
		return nil, trace.Wrap(err)
	}
	// Sanity check WebAuthn challenge.
	switch {
	case challenge.WebauthnChallenge == nil:
		return nil, trace.BadParameter("passwordless: webauthn challenge missing")
	case challenge.WebauthnChallenge.Response.UserVerification == protocol.VerificationDiscouraged:
		return nil, trace.BadParameter("passwordless: user verification requirement too lax (%v)", challenge.WebauthnChallenge.Response.UserVerification)
	}

	stderr := login.StderrOverride
	if stderr == nil {
		stderr = os.Stderr
	}

	prompt := login.CustomPrompt
	if prompt == nil {
		prompt = wancli.NewDefaultPrompt(ctx, stderr)
	}

	promptWebauthn := login.WebauthnLogin
	if promptWebauthn == nil {
		promptWebauthn = wancli.Login
	}

	mfaResp, _, err := promptWebauthn(ctx, webURL.String(), challenge.WebauthnChallenge, prompt, &wancli.LoginOpts{
		User:                    login.User,
		AuthenticatorAttachment: login.AuthenticatorAttachment,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	loginRespJSON, err := webClient.PostJSON(
		ctx, webClient.Endpoint("webapi", "mfa", "login", "finish"),
		&AuthenticateSSHUserRequest{
			User:                      "", // User carried on WebAuthn assertion.
			WebauthnChallengeResponse: wantypes.CredentialAssertionResponseFromProto(mfaResp.GetWebauthn()),
			UserPublicKeys: UserPublicKeys{
				SSHPubKey:               login.SSHPubKey,
				TLSPubKey:               login.TLSPubKey,
				SSHAttestationStatement: login.SSHAttestationStatement,
				TLSAttestationStatement: login.TLSAttestationStatement,
			},
			TTL:               login.TTL,
			Compatibility:     login.Compatibility,
			RouteToCluster:    login.RouteToCluster,
			KubernetesCluster: login.KubernetesCluster,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	loginResp := &authclient.SSHLoginResponse{}
	if err := json.Unmarshal(loginRespJSON.Bytes(), loginResp); err != nil {
		return nil, trace.Wrap(err)
	}
	return loginResp, nil
}

// SSHAgentMFALogin requests a MFA challenge via the proxy.
// If the credentials are valid, the proxy will return a challenge. We then
// prompt the user to provide 2nd factor and pass the response to the proxy.
// If the authentication succeeds, we will get a temporary certificate back.
func SSHAgentMFALogin(ctx context.Context, login SSHLoginMFA) (*authclient.SSHLoginResponse, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mfaResp, err := newMFALoginCeremony(clt, login).Run(ctx, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challengeResp := AuthenticateSSHUserRequest{
		User:     login.User,
		Password: login.Password,
		UserPublicKeys: UserPublicKeys{
			SSHPubKey:               login.SSHPubKey,
			TLSPubKey:               login.TLSPubKey,
			SSHAttestationStatement: login.SSHAttestationStatement,
			TLSAttestationStatement: login.TLSAttestationStatement,
		},
		TTL:               login.TTL,
		Compatibility:     login.Compatibility,
		RouteToCluster:    login.RouteToCluster,
		KubernetesCluster: login.KubernetesCluster,
	}

	// Convert back from auth gRPC proto response.
	switch r := mfaResp.Response.(type) {
	case *proto.MFAAuthenticateResponse_TOTP:
		challengeResp.TOTPCode = r.TOTP.Code
	case *proto.MFAAuthenticateResponse_Webauthn:
		challengeResp.WebauthnChallengeResponse = wantypes.CredentialAssertionResponseFromProto(r.Webauthn)
	default:
		// No challenge was sent, so we send back just username/password.
	}

	loginRespJSON, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "finish"), challengeResp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	loginResp := &authclient.SSHLoginResponse{}
	return loginResp, trace.Wrap(json.Unmarshal(loginRespJSON.Bytes(), loginResp))
}

func newMFALoginCeremony(clt *WebClient, login SSHLoginMFA) *mfa.Ceremony {
	return &mfa.Ceremony{
		CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
			beginReq := MFAChallengeRequest{
				User: login.User,
				Pass: login.Password,
			}
			challengeJSON, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "begin"), beginReq)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			challenge := &MFAAuthenticateChallenge{}
			if err := json.Unmarshal(challengeJSON.Bytes(), challenge); err != nil {
				return nil, trace.Wrap(err)
			}

			// Convert to auth gRPC proto challenge.
			chal := &proto.MFAAuthenticateChallenge{}
			if challenge.TOTPChallenge {
				chal.TOTP = &proto.TOTPChallenge{}
			}
			if challenge.WebauthnChallenge != nil {
				chal.WebauthnChallenge = wantypes.CredentialAssertionToProto(challenge.WebauthnChallenge)
			}
			return chal, nil
		},
		PromptConstructor: login.MFAPromptConstructor,
	}
}

// HostCredentials is used to fetch host credentials for a node.
func HostCredentials(ctx context.Context, proxyAddr string, insecure bool, req types.RegisterUsingTokenRequest) (*proto.Certs, error) {
	clt, _, err := initClient(proxyAddr, insecure, nil, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.PostJSONWithFallback(ctx, clt.Endpoint("webapi", "host", "credentials"), req, insecure)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var certs proto.Certs
	if err := json.Unmarshal(resp.Bytes(), &certs); err != nil {
		return nil, trace.Wrap(err)
	}

	return &certs, nil
}

// GetWebConfig is used by teleterm to fetch webconfig.js from proxies
func GetWebConfig(ctx context.Context, proxyAddr string, insecure bool) (*webclient.WebConfig, error) {
	clt, _, err := initClient(proxyAddr, insecure, nil, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.Get(ctx, clt.Endpoint("web", "config.js"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	body, err := io.ReadAll(response.Reader())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// WebConfig is served as JS file where GRV_CONFIG is a global object name
	text := bytes.TrimSuffix(bytes.Replace(body, []byte("var GRV_CONFIG = "), []byte(""), 1), []byte(";"))

	cfg := webclient.WebConfig{}
	if err := json.Unmarshal(text, &cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	return &cfg, nil
}

// CreateWebSessionReq is a request for the web api to
// initiate a new web session.
type CreateWebSessionReq struct {
	// User is the Teleport username.
	User string `json:"user"`
	// Pass is the password.
	Pass string `json:"pass"`
	// SecondFactorToken is the OTP.
	SecondFactorToken string `json:"second_factor_token"`
}

// CreateWebSessionResponse is a response from the web api
// to a [CreateWebSessionReq] request.
type CreateWebSessionResponse struct {
	// TokenType is token type (bearer)
	TokenType string `json:"type"`
	// Token value
	Token string `json:"token"`
	// TokenExpiresIn sets seconds before this token is not valid
	TokenExpiresIn int `json:"expires_in"`
	// SessionExpires is when this session expires.
	SessionExpires time.Time `json:"sessionExpires,omitempty"`
	// SessionInactiveTimeoutMS specifies how long in milliseconds
	// a user WebUI session can be left idle before being logged out
	// by the server. A zero value means there is no idle timeout set.
	SessionInactiveTimeoutMS int `json:"sessionInactiveTimeout"`
}

// SSHAgentLoginWeb is used by tsh to fetch local user credentials via the web api.
func SSHAgentLoginWeb(ctx context.Context, login SSHLoginDirect) (*WebClient, types.WebSession, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	resp, err := httplib.ConvertResponse(clt.RoundTrip(func() (*http.Response, error) {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(&CreateWebSessionReq{
			User:              login.User,
			Pass:              login.Password,
			SecondFactorToken: login.OTPToken,
		}); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", clt.Endpoint("webapi", "sessions", "web"), &buf)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		return clt.HTTPClient().Do(req)
	}))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	session, err := GetSessionFromResponse(resp)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return clt, session, nil
}

// SSHAgentMFAWebSessionLogin requests a MFA challenge via the proxy web api.
// If the credentials are valid, the proxy will return a challenge. We then
// prompt the user to provide 2nd factor and pass the response to the proxy.
func SSHAgentMFAWebSessionLogin(ctx context.Context, login SSHLoginMFA) (*WebClient, types.WebSession, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	mfaResp, err := newMFALoginCeremony(clt, login).Run(ctx, nil)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	challengeResp := AuthenticateWebUserRequest{
		User: login.User,
	}
	// Convert back from auth gRPC proto response.
	switch r := mfaResp.Response.(type) {
	case *proto.MFAAuthenticateResponse_Webauthn:
		challengeResp.WebauthnAssertionResponse = wantypes.CredentialAssertionResponseFromProto(r.Webauthn)
	default:
		// No challenge was sent, so we send back just username/password.
	}

	loginRespJSON, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "finishsession"), challengeResp)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	session, err := GetSessionFromResponse(loginRespJSON)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return clt, session, nil
}

// SSHAgentPasswordlessLoginWeb requests a passwordless MFA challenge via the proxy
// web api.
func SSHAgentPasswordlessLoginWeb(ctx context.Context, login SSHLoginPasswordless) (*WebClient, types.WebSession, error) {
	webClient, webURL, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	challengeJSON, err := webClient.PostJSON(
		ctx, webClient.Endpoint("webapi", "mfa", "login", "begin"),
		&MFAChallengeRequest{
			Passwordless: true,
		})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	challenge := &MFAAuthenticateChallenge{}
	if err := json.Unmarshal(challengeJSON.Bytes(), challenge); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// Sanity check WebAuthn challenge.
	switch {
	case challenge.WebauthnChallenge == nil:
		return nil, nil, trace.BadParameter("passwordless: webauthn challenge missing")
	case challenge.WebauthnChallenge.Response.UserVerification == protocol.VerificationDiscouraged:
		return nil, nil, trace.BadParameter("passwordless: user verification requirement too lax (%v)", challenge.WebauthnChallenge.Response.UserVerification)
	}

	stderr := login.StderrOverride
	if stderr == nil {
		stderr = os.Stderr
	}

	prompt := login.CustomPrompt
	if prompt == nil {
		prompt = wancli.NewDefaultPrompt(ctx, stderr)
	}

	promptWebauthn := login.WebauthnLogin
	if promptWebauthn == nil {
		promptWebauthn = wancli.Login
	}

	mfaResp, _, err := promptWebauthn(ctx, webURL.String(), challenge.WebauthnChallenge, prompt, &wancli.LoginOpts{
		User:                    login.User,
		AuthenticatorAttachment: login.AuthenticatorAttachment,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	loginRespJSON, err := webClient.PostJSON(
		ctx, webClient.Endpoint("webapi", "mfa", "login", "finishsession"),
		&AuthenticateWebUserRequest{
			User:                      login.User,
			WebauthnAssertionResponse: wantypes.CredentialAssertionResponseFromProto(mfaResp.GetWebauthn()),
		})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	webSession, err := GetSessionFromResponse(loginRespJSON)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return webClient, webSession, nil
}

// GetSessionFromResponse creates a [types.WebSession] if a cookie
// named [websession.CookieName] is present in the provided [roundtrip.Response].
func GetSessionFromResponse(resp *roundtrip.Response) (types.WebSession, error) {
	var sess CreateWebSessionResponse
	if err := json.Unmarshal(resp.Bytes(), &sess); err != nil {
		return nil, trace.Wrap(err)
	}

	cookies := resp.Cookies()

	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == websession.CookieName {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		return nil, trace.BadParameter("no session cookie present")
	}

	cookie, err := websession.DecodeCookie(sessionCookie.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := types.NewWebSession(cookie.SID, types.KindWebSession, types.WebSessionSpecV2{
		User:               cookie.User,
		BearerToken:        sess.Token,
		BearerTokenExpires: time.Now().Add(time.Duration(sess.TokenExpiresIn) * time.Second),
		Expires:            sess.SessionExpires,
		LoginTime:          time.Now(),
		IdleTimeout:        types.Duration(time.Duration(sess.SessionInactiveTimeoutMS) * time.Millisecond),
	})
	return session, trace.Wrap(err)
}
