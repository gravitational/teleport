/*
Copyright 2020 Gravitational, Inc.

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

package types

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"
)

// WebSessionsGetter provides access to web sessions
type WebSessionsGetter interface {
	// WebSessions returns the web session manager
	WebSessions() WebSessionInterface
}

// WebSessionInterface defines interface to regular web sessions
type WebSessionInterface interface {
	// Get returns a web session state for the given request.
	Get(ctx context.Context, req GetWebSessionRequest) (WebSession, error)

	// List gets all regular web sessions.
	List(context.Context) ([]WebSession, error)

	// Upsert updates existing or inserts a new web session.
	Upsert(ctx context.Context, session WebSession) error

	// Delete deletes the web session described by req.
	Delete(ctx context.Context, req DeleteWebSessionRequest) error

	// DeleteAll removes all web sessions.
	DeleteAll(context.Context) error
}

// WebSession stores key and value used to authenticate with SSH
// notes on behalf of user
type WebSession interface {
	// Resource represents common properties for all resources.
	Resource
	// GetShortName returns visible short name used in logging
	GetShortName() string
	// GetUser returns the user this session is associated with
	GetUser() string
	// SetUser sets user associated with this session
	SetUser(string)
	// GetPub is returns public certificate signed by auth server
	GetPub() []byte
	// GetSSHPriv returns private SSH key used to auth with SSH nodes.
	GetSSHPriv() []byte
	// SetSSHPriv sets SSH private key.
	SetSSHPriv([]byte)
	// GetTLSPriv returns private TLS key.
	GetTLSPriv() []byte
	// SetTLSPriv sets TLS private key.
	SetTLSPriv([]byte)
	// GetTLSCert returns PEM encoded TLS certificate associated with session
	GetTLSCert() []byte
	// GetBearerToken is a special bearer token used for additional
	// bearer authentication
	GetBearerToken() string
	// SetExpiryTime sets session expiry time
	SetExpiryTime(time.Time)
	// GetBearerTokenExpiryTime - absolute time when token expires
	GetBearerTokenExpiryTime() time.Time
	// GetExpiryTime - absolute time when web session expires
	GetExpiryTime() time.Time
	// GetLoginTime returns the time this user recently logged in.
	GetLoginTime() time.Time
	// SetLoginTime sets when this user logged in.
	SetLoginTime(time.Time)
	// GetIdleTimeout returns the max time a user can be inactive for this session.
	GetIdleTimeout() time.Duration
	// WithoutSecrets returns copy of the web session but without private keys
	WithoutSecrets() WebSession
	// String returns string representation of the session.
	String() string
	// SetConsumedAccessRequestID sets the ID of the access request from which additional roles to assume were obtained.
	SetConsumedAccessRequestID(string)
	// GetConsumedAccessRequestID returns the ID of the access request from which additional roles to assume were obtained.
	GetConsumedAccessRequestID() string
	// SetSAMLSession sets the SAML session data. Is considered secret.
	SetSAMLSession(*SAMLSessionData)
	// GetSAMLSession gets the SAML session data. Is considered secret.
	GetSAMLSession() *SAMLSessionData
	// SetDeviceWebToken sets the session's DeviceWebToken.
	// The token is considered a secret.
	SetDeviceWebToken(*DeviceWebToken)
	// GetDeviceWebToken returns the session's DeviceWebToken, if any.
	// The token is considered a secret.
	GetDeviceWebToken() *DeviceWebToken
	// GetHasDeviceExtensions returns the HasDeviceExtensions value.
	// If true the session's TLS and SSH certificates are augmented with device
	// extensions.
	GetHasDeviceExtensions() bool
	// SetTrustedDeviceRequirement sets the session's trusted device requirement.
	// See [TrustedDeviceRequirement].
	SetTrustedDeviceRequirement(r TrustedDeviceRequirement)
	// GetTrustedDeviceRequirement returns the session's trusted device
	// requirement.
	// See [TrustedDeviceRequirement].
	GetTrustedDeviceRequirement() TrustedDeviceRequirement
}

// NewWebSession returns new instance of the web session based on the V2 spec
func NewWebSession(name string, subkind string, spec WebSessionSpecV2) (WebSession, error) {
	ws := &WebSessionV2{
		SubKind: subkind,
		Metadata: Metadata{
			Name:    name,
			Expires: &spec.Expires,
		},
		Spec: spec,
	}
	if err := ws.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return ws, nil
}

// GetKind gets resource Kind
func (ws *WebSessionV2) GetKind() string {
	return ws.Kind
}

// GetSubKind gets resource SubKind
func (ws *WebSessionV2) GetSubKind() string {
	return ws.SubKind
}

// SetSubKind sets resource SubKind
func (ws *WebSessionV2) SetSubKind(subKind string) {
	ws.SubKind = subKind
}

// GetVersion gets resource Version
func (ws *WebSessionV2) GetVersion() string {
	return ws.Version
}

// GetName gets resource Name
func (ws *WebSessionV2) GetName() string {
	return ws.Metadata.Name
}

// SetName sets resource Name
func (ws *WebSessionV2) SetName(name string) {
	ws.Metadata.Name = name
}

// Expiry returns resource Expiry
func (ws *WebSessionV2) Expiry() time.Time {
	return ws.Metadata.Expiry()
}

// SetExpiry Sets resource Expiry
func (ws *WebSessionV2) SetExpiry(expiry time.Time) {
	ws.Metadata.SetExpiry(expiry)
}

// GetMetadata gets resource Metadata
func (ws *WebSessionV2) GetMetadata() Metadata {
	return ws.Metadata
}

// GetRevision returns the revision
func (ws *WebSessionV2) GetRevision() string {
	return ws.Metadata.GetRevision()
}

// SetRevision sets the revision
func (ws *WebSessionV2) SetRevision(rev string) {
	ws.Metadata.SetRevision(rev)
}

// GetIdleTimeout returns the max idle timeout duration.
func (ws *WebSessionV2) GetIdleTimeout() time.Duration {
	return ws.Spec.IdleTimeout.Duration()
}

// WithoutSecrets returns a copy of the WebSession without secrets.
func (ws *WebSessionV2) WithoutSecrets() WebSession {
	cp := *ws
	cp.Spec.Priv = nil
	cp.Spec.TLSPriv = nil
	cp.Spec.SAMLSession = nil
	cp.Spec.DeviceWebToken = nil
	return &cp
}

// SetConsumedAccessRequestID sets the ID of the access request from which additional roles to assume were obtained.
func (ws *WebSessionV2) SetConsumedAccessRequestID(requestID string) {
	ws.Spec.ConsumedAccessRequestID = requestID
}

// GetConsumedAccessRequestID returns the ID of the access request from which additional roles to assume were obtained.
func (ws *WebSessionV2) GetConsumedAccessRequestID() string {
	return ws.Spec.ConsumedAccessRequestID
}

// SetSAMLSession sets the SAML session data. Is considered secret.
func (ws *WebSessionV2) SetSAMLSession(samlSession *SAMLSessionData) {
	ws.Spec.SAMLSession = samlSession
}

// GetSAMLSession gets the SAML session data. Is considered secret.
func (ws *WebSessionV2) GetSAMLSession() *SAMLSessionData {
	return ws.Spec.SAMLSession
}

// SetDeviceWebToken sets the session's DeviceWebToken.
// The token is considered a secret.
func (ws *WebSessionV2) SetDeviceWebToken(webToken *DeviceWebToken) {
	ws.Spec.DeviceWebToken = webToken
}

// GetDeviceWebToken returns the session's DeviceWebToken, if any.
// The token is considered a secret.
func (ws *WebSessionV2) GetDeviceWebToken() *DeviceWebToken {
	return ws.Spec.DeviceWebToken
}

// GetHasDeviceExtensions returns the HasDeviceExtensions value.
// If true the session's TLS and SSH certificates are augmented with device
// extensions.
func (ws *WebSessionV2) GetHasDeviceExtensions() bool {
	return ws.Spec.HasDeviceExtensions
}

// SetTrustedDeviceRequirement sets the session's trusted device requirement.
func (ws *WebSessionV2) SetTrustedDeviceRequirement(r TrustedDeviceRequirement) {
	ws.Spec.TrustedDeviceRequirement = r
}

// GetTrustedDeviceRequirement returns the session's trusted device
// requirement.
func (ws *WebSessionV2) GetTrustedDeviceRequirement() TrustedDeviceRequirement {
	return ws.Spec.TrustedDeviceRequirement
}

// setStaticFields sets static resource header and metadata fields.
func (ws *WebSessionV2) setStaticFields() {
	ws.Version = V2
	ws.Kind = KindWebSession
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (ws *WebSessionV2) CheckAndSetDefaults() error {
	ws.setStaticFields()
	if err := ws.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if ws.Spec.User == "" {
		return trace.BadParameter("missing User")
	}
	return nil
}

// String returns string representation of the session.
func (ws *WebSessionV2) String() string {
	return fmt.Sprintf("WebSession(kind=%v/%v,user=%v,id=%v,expires=%v)",
		ws.GetKind(), ws.GetSubKind(), ws.GetUser(), ws.GetName(), ws.GetExpiryTime())
}

// SetUser sets user associated with this session
func (ws *WebSessionV2) SetUser(u string) {
	ws.Spec.User = u
}

// GetUser returns the user this session is associated with
func (ws *WebSessionV2) GetUser() string {
	return ws.Spec.User
}

// GetShortName returns visible short name used in logging
func (ws *WebSessionV2) GetShortName() string {
	if len(ws.Metadata.Name) < 4 {
		return "<undefined>"
	}
	return ws.Metadata.Name[:4]
}

// GetTLSCert returns PEM encoded TLS certificate associated with session
func (ws *WebSessionV2) GetTLSCert() []byte {
	return ws.Spec.TLSCert
}

// GetPub is returns public certificate signed by auth server
func (ws *WebSessionV2) GetPub() []byte {
	return ws.Spec.Pub
}

// GetSSHPriv returns private SSH key.
func (ws *WebSessionV2) GetSSHPriv() []byte {
	return ws.Spec.Priv
}

// SetSSHPriv sets private SSH key.
func (ws *WebSessionV2) SetSSHPriv(priv []byte) {
	ws.Spec.Priv = priv
}

// GetTLSPriv returns private TLS key.
func (ws *WebSessionV2) GetTLSPriv() []byte {
	// TODO(nklaassen): DELETE IN 18.0.0 when all auth servers are writing web session TLS key.
	if ws.Spec.TLSPriv == nil {
		// An older auth instance may have written this web session before the
		// SSH and TLS keys were split.
		return ws.Spec.Priv
	}
	return ws.Spec.TLSPriv
}

// SetTLSPriv sets private TLS key.
func (ws *WebSessionV2) SetTLSPriv(priv []byte) {
	ws.Spec.TLSPriv = priv
}

// GetBearerToken gets a special bearer token used for additional
// bearer authentication
func (ws *WebSessionV2) GetBearerToken() string {
	return ws.Spec.BearerToken
}

// SetExpiryTime sets session expiry time
func (ws *WebSessionV2) SetExpiryTime(tm time.Time) {
	ws.Spec.Expires = tm
}

// GetBearerTokenExpiryTime - absolute time when token expires
func (ws *WebSessionV2) GetBearerTokenExpiryTime() time.Time {
	return ws.Spec.BearerTokenExpires
}

// GetExpiryTime - absolute time when web session expires
func (ws *WebSessionV2) GetExpiryTime() time.Time {
	return ws.Spec.Expires
}

// GetLoginTime returns the time this user recently logged in.
func (ws *WebSessionV2) GetLoginTime() time.Time {
	return ws.Spec.LoginTime
}

// SetLoginTime sets when this user logged in.
func (ws *WebSessionV2) SetLoginTime(loginTime time.Time) {
	ws.Spec.LoginTime = loginTime
}

// GetAppSessionRequest contains the parameters to request an application
// web session.
type GetAppSessionRequest struct {
	// SessionID is the session ID of the application session itself.
	SessionID string
}

// Check validates the request.
func (r *GetAppSessionRequest) Check() error {
	if r.SessionID == "" {
		return trace.BadParameter("session ID missing")
	}
	return nil
}

// GetSnowflakeSessionRequest contains the parameters to request a Snowflake
// web session.
type GetSnowflakeSessionRequest struct {
	// SessionID is the session ID of the Snowflake session itself.
	SessionID string
}

// Check validates the request.
func (r *GetSnowflakeSessionRequest) Check() error {
	if r.SessionID == "" {
		return trace.BadParameter("session ID missing")
	}
	return nil
}

// GetSAMLIdPSessionRequest contains the parameters to request a SAML IdP
// session.
type GetSAMLIdPSessionRequest struct {
	// SessionID is the session ID of the SAML IdP session.
	SessionID string
}

// Check validates the request.
func (r *GetSAMLIdPSessionRequest) Check() error {
	if r.SessionID == "" {
		return trace.BadParameter("session ID missing")
	}
	return nil
}

// CreateSnowflakeSessionRequest contains the parameters needed to request
// creating a Snowflake web session.
type CreateSnowflakeSessionRequest struct {
	// Username is the identity of the user requesting the session.
	Username string
	// SessionToken is the Snowflake server session token.
	SessionToken string
	// TokenTTL is the token validity period.
	TokenTTL time.Duration
}

// CreateSAMLIdPSessionRequest contains the parameters needed to request
// creating a SAML IdP session.
type CreateSAMLIdPSessionRequest struct {
	// SessionID is the identifier for the session.
	SessionID string
	// Username is the identity of the user requesting the session.
	Username string `json:"username"`
	// SAMLSession is the session data associated with the SAML IdP session.
	SAMLSession *SAMLSessionData `json:"saml_session"`
}

// Check validates the request.
func (r CreateSAMLIdPSessionRequest) Check() error {
	if r.Username == "" {
		return trace.BadParameter("username missing")
	}
	if r.SAMLSession == nil {
		return trace.BadParameter("saml session missing")
	}

	return nil
}

// DeleteAppSessionRequest are the parameters used to request removal of
// an application web session.
type DeleteAppSessionRequest struct {
	SessionID string `json:"session_id"`
}

// DeleteSnowflakeSessionRequest are the parameters used to request removal of
// a Snowflake web session.
type DeleteSnowflakeSessionRequest struct {
	SessionID string `json:"session_id"`
}

// DeleteSAMLIdPSessionRequest are the parameters used to request removal of
// a SAML IdP session.
type DeleteSAMLIdPSessionRequest struct {
	SessionID string `json:"session_id"`
}

// NewWebToken returns a new web token with the given expiration and spec
func NewWebToken(expires time.Time, spec WebTokenSpecV3) (WebToken, error) {
	r := &WebTokenV3{
		Metadata: Metadata{
			Name:    spec.Token,
			Expires: &expires,
		},
		Spec: spec,
	}
	if err := r.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return r, nil
}

// WebTokensGetter provides access to web tokens
type WebTokensGetter interface {
	// WebTokens returns the tokens manager
	WebTokens() WebTokenInterface
}

// WebTokenInterface defines interface for managing web tokens
type WebTokenInterface interface {
	// Get returns a token specified by the request.
	Get(ctx context.Context, req GetWebTokenRequest) (WebToken, error)

	// List gets all web tokens.
	List(context.Context) ([]WebToken, error)

	// Upsert updates existing or inserts a new web token.
	Upsert(ctx context.Context, token WebToken) error

	// Delete deletes the web token described by req.
	Delete(ctx context.Context, req DeleteWebTokenRequest) error

	// DeleteAll removes all web tokens.
	DeleteAll(context.Context) error
}

// WebToken is a time-limited unique token bound to a user's session
type WebToken interface {
	// Resource represents common properties for all resources.
	Resource

	// GetToken returns the token value
	GetToken() string
	// SetToken sets the token value
	SetToken(token string)
	// GetUser returns the user the token is bound to
	GetUser() string
	// SetUser sets the user the token is bound to
	SetUser(user string)
	// String returns the text representation of this token
	String() string
}

var _ WebToken = &WebTokenV3{}

// GetMetadata returns the token metadata
func (r *WebTokenV3) GetMetadata() Metadata {
	return r.Metadata
}

// GetKind returns the token resource kind
func (r *WebTokenV3) GetKind() string {
	return r.Kind
}

// GetSubKind returns the token resource subkind
func (r *WebTokenV3) GetSubKind() string {
	return r.SubKind
}

// SetSubKind sets the token resource subkind
func (r *WebTokenV3) SetSubKind(subKind string) {
	r.SubKind = subKind
}

// GetVersion returns the token resource version
func (r *WebTokenV3) GetVersion() string {
	return r.Version
}

// GetName returns the token value
func (r *WebTokenV3) GetName() string {
	return r.Metadata.Name
}

// SetName sets the token value
func (r *WebTokenV3) SetName(name string) {
	r.Metadata.Name = name
}

// GetRevision returns the revision
func (r *WebTokenV3) GetRevision() string {
	return r.Metadata.GetRevision()
}

// SetRevision sets the revision
func (r *WebTokenV3) SetRevision(rev string) {
	r.Metadata.SetRevision(rev)
}

// GetToken returns the token value
func (r *WebTokenV3) GetToken() string {
	return r.Spec.Token
}

// SetToken sets the token value
func (r *WebTokenV3) SetToken(token string) {
	r.Spec.Token = token
}

// GetUser returns the user this token is bound to
func (r *WebTokenV3) GetUser() string {
	return r.Spec.User
}

// SetUser sets the user this token is bound to
func (r *WebTokenV3) SetUser(user string) {
	r.Spec.User = user
}

// Expiry returns the token absolute expiration time
func (r *WebTokenV3) Expiry() time.Time {
	if r.Metadata.Expires == nil {
		return time.Time{}
	}
	return *r.Metadata.Expires
}

// SetExpiry sets the token absolute expiration time
func (r *WebTokenV3) SetExpiry(t time.Time) {
	r.Metadata.Expires = &t
}

// setStaticFields sets static resource header and metadata fields.
func (r *WebTokenV3) setStaticFields() {
	r.Kind = KindWebToken
	r.Version = V3
}

// CheckAndSetDefaults validates this token value and sets defaults
func (r *WebTokenV3) CheckAndSetDefaults() error {
	r.setStaticFields()
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if r.Spec.User == "" {
		return trace.BadParameter("User required")
	}
	if r.Spec.Token == "" {
		return trace.BadParameter("Token required")
	}
	return nil
}

// String returns string representation of the token.
func (r *WebTokenV3) String() string {
	return fmt.Sprintf("WebToken(kind=%v,user=%v,token=%v,expires=%v)",
		r.GetKind(), r.GetUser(), r.GetToken(), r.Expiry())
}

// Check validates the request.
func (r *GetWebSessionRequest) Check() error {
	if r.User == "" {
		return trace.BadParameter("user name missing")
	}
	if r.SessionID == "" {
		return trace.BadParameter("session ID missing")
	}
	return nil
}

// Check validates the request.
func (r *DeleteWebSessionRequest) Check() error {
	if r.SessionID == "" {
		return trace.BadParameter("session ID missing")
	}
	return nil
}

// Check validates the request.
func (r *GetWebTokenRequest) Check() error {
	if r.User == "" {
		return trace.BadParameter("user name missing")
	}
	if r.Token == "" {
		return trace.BadParameter("token missing")
	}
	return nil
}

// Check validates the request.
func (r *DeleteWebTokenRequest) Check() error {
	if r.Token == "" {
		return trace.BadParameter("token missing")
	}
	return nil
}

// IntoMap makes this filter into a map.
//
// This filter is used with the cache watcher to make sure only sessions
// for a particular user are returned.
func (f *WebSessionFilter) IntoMap() map[string]string {
	m := make(map[string]string)
	if f.User != "" {
		m[keyUser] = f.User
	}
	return m
}

// FromMap converts provided map into this filter.
//
// This filter is used with the cache watcher to make sure only sessions
// for a particular user are returned.
func (f *WebSessionFilter) FromMap(m map[string]string) error {
	for key, val := range m {
		switch key {
		case keyUser:
			f.User = val
		default:
			return trace.BadParameter("unknown filter key %s", key)
		}
	}
	return nil
}

// Match checks if a given web session matches this filter.
func (f *WebSessionFilter) Match(session WebSession) bool {
	if f.User != "" && session.GetUser() != f.User {
		return false
	}
	return true
}
