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
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

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
	// SetBearerTokenExpiryTime sets bearer token expiry time
	SetBearerTokenExpiryTime(time.Time)
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

// SetTTL Sets resource TTL
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

	return nil
}

// String returns string representation of the session.
func (ws *WebSessionV2) String() string {
	return fmt.Sprintf("WebSession(kind=%v,name=%v,id=%v)", ws.GetKind(), ws.GetUser(), ws.GetName())
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

// SetBearerTokenExpiryTime sets bearer token expiry time
func (ws *WebSessionV2) SetBearerTokenExpiryTime(tm time.Time) {
	ws.Spec.BearerTokenExpires = tm
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
	// ParentSession is the session ID of the parent session.
	ParentSession string `json:"parent_session"`
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
	if r.ParentSession == "" {
		return trace.BadParameter("parent session missing")
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

// WebSessionSpecV2Schema is JSON schema for cert authority V2
const WebSessionSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["pub", "bearer_token", "bearer_token_expires", "expires", "user"],
  "properties": {
    "user": {"type": "string"},
    "pub": {"type": "string"},
    "priv": {"type": "string"},
    "tls_cert": {"type": "string"},
    "bearer_token": {"type": "string"},
    "bearer_token_expires": {"type": "string"},
    "expires": {"type": "string"}%v
  }
}`

// GetWebSessionSchema returns JSON Schema for web session
func GetWebSessionSchema() string {
	return GetWebSessionSchemaWithExtensions("")
}

// GetWebSessionSchemaWithExtensions returns JSON Schema for web session with user-supplied extensions
func GetWebSessionSchemaWithExtensions(extension string) string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, fmt.Sprintf(WebSessionSpecV2Schema, extension), DefaultDefinitions)
}

// WebSessionMarshaler implements marshal/unmarshal of User implementations
// mostly adds support for extended versions
type WebSessionMarshaler interface {
	// UnmarshalWebSession unmarhsals cert authority from binary representation
	UnmarshalWebSession(bytes []byte, opts ...MarshalOption) (WebSession, error)
	// MarshalWebSession to binary representation
	MarshalWebSession(c WebSession, opts ...MarshalOption) ([]byte, error)
	// GenerateWebSession generates new web session and is used to
	// inject additional data in extenstions
	GenerateWebSession(WebSession) (WebSession, error)
	// ExtendWebSession extends web session and is used to
	// inject additional data in extenstions when session is getting renewed
	ExtendWebSession(WebSession) (WebSession, error)
}

type teleportWebSessionMarshaler struct{}

// GenerateWebSession generates new web session and is used to
// inject additional data in extenstions
func (*teleportWebSessionMarshaler) GenerateWebSession(ws WebSession) (WebSession, error) {
	return ws, nil
}

// ExtendWebSession renews web session and is used to
// inject additional data in extenstions when session is getting renewed
func (*teleportWebSessionMarshaler) ExtendWebSession(ws WebSession) (WebSession, error) {
	return ws, nil
}

// UnmarshalWebSession unmarshals web session from on-disk byte format
func (*teleportWebSessionMarshaler) UnmarshalWebSession(bytes []byte, opts ...MarshalOption) (WebSession, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var h ResourceHeader
	err = json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V2:
		var ws WebSessionV2
		if err := utils.UnmarshalWithSchema(GetWebSessionSchema(), &ws, bytes); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		utils.UTC(&ws.Spec.BearerTokenExpires)
		utils.UTC(&ws.Spec.Expires)

		if err := ws.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			ws.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			ws.SetExpiry(cfg.Expires)
		}

		return &ws, nil
	}

	return nil, trace.BadParameter("web session resource version %v is not supported", h.Version)
}

// MarshalWebSession marshals web session into on-disk representation
func (*teleportWebSessionMarshaler) MarshalWebSession(ws WebSession, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch webSession := ws.(type) {
	case *WebSessionV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *webSession
			copy.SetResourceID(0)
			webSession = &copy
		}
		return utils.FastMarshal(webSession)
	default:
		return nil, trace.BadParameter("unrecognized web session version %T", ws)
	}
}

var webSessionMarshaler WebSessionMarshaler = &teleportWebSessionMarshaler{}

// SetWebSessionMarshaler sets global WebSessionMarshaler
func SetWebSessionMarshaler(u WebSessionMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	webSessionMarshaler = u
}

// GetWebSessionMarshaler returns currently set WebSessionMarshaler
func GetWebSessionMarshaler() WebSessionMarshaler {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	return webSessionMarshaler
}
