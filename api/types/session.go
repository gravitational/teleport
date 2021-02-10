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

	"github.com/gravitational/teleport/api/defaults"

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
	// GetName returns session name
	GetName() string
	// GetUser returns the user this session is associated with
	GetUser() string
	// SetName sets session name
	SetName(string)
	// SetUser sets user associated with this session
	SetUser(string)
	// GetPub is returns public certificate signed by auth server
	GetPub() []byte
	// GetPriv returns private OpenSSH key used to auth with SSH nodes
	GetPriv() []byte
	// SetPriv sets private key
	SetPriv([]byte)
	// GetTLSCert returns PEM encoded TLS certificate associated with session
	GetTLSCert() []byte
	// BearerToken is a special bearer token used for additional
	// bearer authentication
	GetBearerToken() string
	// SetExpiryTime sets session expiry time
	SetExpiryTime(time.Time)
	// GetBearerTokenExpiryTime - absolute time when token expires
	GetBearerTokenExpiryTime() time.Time
	// GetExpiryTime - absolute time when web session expires
	GetExpiryTime() time.Time
	// WithoutSecrets returns copy of the web session but without private keys
	WithoutSecrets() WebSession
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
	// String returns string representation of the session.
	String() string
	// Expiry is the expiration time for this resource.
	Expiry() time.Time
}

// NewWebSession returns new instance of the web session based on the V2 spec
func NewWebSession(name string, kind string, subkind string, spec WebSessionSpecV2) WebSession {
	return &WebSessionV2{
		Kind:    kind,
		SubKind: subkind,
		Version: V2,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
			Expires:   &spec.Expires,
		},
		Spec: spec,
	}
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

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (ws *WebSessionV2) SetTTL(clock Clock, ttl time.Duration) {
	ws.Metadata.SetTTL(clock, ttl)
}

// GetMetadata gets resource Metadata
func (ws *WebSessionV2) GetMetadata() Metadata {
	return ws.Metadata
}

// GetResourceID gets ResourceID
func (ws *WebSessionV2) GetResourceID() int64 {
	return ws.Metadata.GetID()
}

// SetResourceID sets ResourceID
func (ws *WebSessionV2) SetResourceID(id int64) {
	ws.Metadata.SetID(id)
}

// WithoutSecrets returns copy of the object but without secrets
func (ws *WebSessionV2) WithoutSecrets() WebSession {
	ws.Spec.Priv = nil
	return ws
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (ws *WebSessionV2) CheckAndSetDefaults() error {
	err := ws.Metadata.CheckAndSetDefaults()
	if err != nil {
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

// GetPriv returns private OpenSSH key used to auth with SSH nodes
func (ws *WebSessionV2) GetPriv() []byte {
	return ws.Spec.Priv
}

// SetPriv sets private key
func (ws *WebSessionV2) SetPriv(priv []byte) {
	ws.Spec.Priv = priv
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

// CreateAppSessionRequest contains the parameters needed to request
// creating an application web session.
type CreateAppSessionRequest struct {
	// Username is the identity of the user requesting the session.
	Username string `json:"username"`
	// PublicAddr is the public address of the application.
	PublicAddr string `json:"public_addr"`
	// ClusterName is the name of the cluster within which the application is running.
	ClusterName string `json:"cluster_name"`
}

// Check validates the request.
func (r CreateAppSessionRequest) Check() error {
	if r.Username == "" {
		return trace.BadParameter("username missing")
	}
	if r.PublicAddr == "" {
		return trace.BadParameter("public address missing")
	}
	if r.ClusterName == "" {
		return trace.BadParameter("cluster name missing")
	}

	return nil
}

// DeleteAppSessionRequest are the parameters used to request removal of
// an application web session.
type DeleteAppSessionRequest struct {
	SessionID string `json:"session_id"`
}

// NewWebToken returns a new web token with the given expiration and spec
func NewWebToken(expires time.Time, spec WebTokenSpecV3) WebToken {
	return &WebTokenV3{
		Kind:    KindWebToken,
		Version: V3,
		Metadata: Metadata{
			Name:      spec.Token,
			Namespace: defaults.Namespace,
			Expires:   &expires,
		},
		Spec: spec,
	}
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

	// Delete deletes the web token described by req.
	Delete(ctx context.Context, req DeleteWebTokenRequest) error

	// DeleteAll removes all web tokens.
	DeleteAll(context.Context) error
}

// WebToken is a time-limited unique token bound to a user's session
type WebToken interface {
	// Resource represents common properties for all resources.
	Resource

	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
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

// GetResourceID returns the token resource ID
func (r *WebTokenV3) GetResourceID() int64 {
	return r.Metadata.GetID()
}

// SetResourceID sets the token resource ID
func (r *WebTokenV3) SetResourceID(id int64) {
	r.Metadata.SetID(id)
}

// SetTTL sets the token resource TTL (time-to-live) value
func (r *WebTokenV3) SetTTL(clock Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
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

// CheckAndSetDefaults validates this token value and sets defaults
func (r *WebTokenV3) CheckAndSetDefaults() error {
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

// NewWebSessionRequest defines a request to create a new user
// web session
type NewWebSessionRequest struct {
	// User specifies the user this session is bound to
	User string
	// Roles optionally lists additional user roles
	Roles []string
	// Traits optionally lists role traits
	Traits map[string][]string
	// SessionTTL optionally specifies the session time-to-live.
	// If left unspecified, the default certificate duration is used.
	SessionTTL time.Duration
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

// Equals compares two filters.
func (f *WebSessionFilter) Equals(o WebSessionFilter) bool {
	return f.User == o.User
}
