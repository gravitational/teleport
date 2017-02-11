package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// WebSession stores key and value used to authenticate with SSH
// notes on behalf of user
type WebSession interface {
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
	// BearerToken is a special bearer token used for additional
	// bearer authentication
	GetBearerToken() string
	// SetExpiryTime sets session expiry time
	SetExpiryTime(time.Time)
	// Expires - absolute time when token expires
	GetExpiryTime() time.Time
	// V1 returns V1 version of the resource
	V1() *WebSessionV1
	// V2 returns V2 version of the resource
	V2() *WebSessionV2
	// WithoutSecrets returns copy of the web session but without private keys
	WithoutSecrets() WebSession
}

// NewWebSession returns new instance of the web session based on the V2 spec
func NewWebSession(name string, spec WebSessionSpecV2) WebSession {
	return &WebSessionV2{
		Kind:    KindWebSession,
		Version: V2,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// WebSessionV2 is version 2 spec for session
type WebSessionV2 struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is version
	Version string `json:"version"`
	// Metadata is connector metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains cert authority specification
	Spec WebSessionSpecV2 `json:"spec"`
}

// WebSessionSpecV2 is a spec for V2 session
type WebSessionSpecV2 struct {
	// User is a user this web session belongs to
	User string `json:"user"`
	// Pub is a public certificate signed by auth server
	Pub []byte `json:"pub"`
	// Priv is a private OpenSSH key used to auth with SSH nodes
	Priv []byte `json:"priv"`
	// BearerToken is a special bearer token used for additional
	// bearer authentication
	BearerToken string `json:"bearer_token"`
	// Expires - absolute time when token expires
	Expires time.Time `json:"expires"`
}

// WithoutSecrets returns copy of the object but without secrets
func (ws *WebSessionV2) WithoutSecrets() WebSession {
	v2 := ws.V2()
	v2.Spec.Priv = nil
	return nil
}

// SetName sets session name
func (ws *WebSessionV2) SetName(name string) {
	ws.Metadata.Name = name
}

// SetUser sets user associated with this session
func (ws *WebSessionV2) SetUser(u string) {
	ws.Spec.User = u
}

// GetUser returns the user this session is associated with
func (ws *WebSessionV2) GetUser() string {
	return ws.Spec.User
}

// GetName returns session name
func (ws *WebSessionV2) GetName() string {
	return ws.Metadata.Name
}

// GetPub is returns public certificate signed by auth server
func (ws *WebSessionV2) GetPub() []byte {
	return ws.Spec.Pub
}

// GetPriv returns private OpenSSH key used to auth with SSH nodes
func (ws *WebSessionV2) GetPriv() []byte {
	return ws.Spec.Priv
}

// BearerToken is a special bearer token used for additional
// bearer authentication
func (ws *WebSessionV2) GetBearerToken() string {
	return ws.Spec.BearerToken
}

// Expires - absolute time when token expires
func (ws *WebSessionV2) GetExpiryTime() time.Time {
	return ws.Spec.Expires
}

// SetExpiryTime sets session expiry time
func (ws *WebSessionV2) SetExpiryTime(tm time.Time) {
	ws.Spec.Expires = tm
}

// V2 returns V2 version of the resource
func (ws *WebSessionV2) V2() *WebSessionV2 {
	return ws
}

// V1 returns V1 version of the object
func (ws *WebSessionV2) V1() *WebSessionV1 {
	return &WebSessionV1{
		ID:          ws.Metadata.Name,
		Priv:        ws.Spec.Priv,
		Pub:         ws.Spec.Pub,
		BearerToken: ws.Spec.BearerToken,
		Expires:     ws.Spec.Expires,
	}
}

// WebSessionSpecV2Schema is JSON schema for cert authority V2
const WebSessionSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["pub", "priv", "bearer_token", "expires", "user"],
  "properties": {
    "user": {"type": "string"},
    "pub": {"type": "string"},
    "priv": {"type": "string"},
    "bearer_token": {"type": "string"},
    "expires": {"type": "string"}%v
  }
}`

// WebSession stores key and value used to authenticate with SSH
// nodes on behalf of user
type WebSessionV1 struct {
	// ID is session ID
	ID string `json:"id"`
	// User is a user this web session is associated with
	User string `json:"user"`
	// Pub is a public certificate signed by auth server
	Pub []byte `json:"pub"`
	// Priv is a private OpenSSH key used to auth with SSH nodes
	Priv []byte `json:"priv"`
	// BearerToken is a special bearer token used for additional
	// bearer authentication
	BearerToken string `json:"bearer_token"`
	// Expires - absolute time when token expires
	Expires time.Time `json:"expires"`
}

// V1 returns V1 version of the resource
func (s *WebSessionV1) V1() *WebSessionV1 {
	return s
}

// V2 returns V2 version of the resource
func (s *WebSessionV1) V2() *WebSessionV2 {
	return &WebSessionV2{
		Kind:    KindWebSession,
		Version: V2,
		Metadata: Metadata{
			Name:      s.ID,
			Namespace: defaults.Namespace,
		},
		Spec: WebSessionSpecV2{
			Pub:         s.Pub,
			Priv:        s.Priv,
			BearerToken: s.BearerToken,
			Expires:     s.Expires,
		},
	}
}

// WithoutSecrets returns copy of the web session but without private keys
func (ws *WebSessionV1) WithoutSecrets() WebSession {
	v2 := ws.V2()
	v2.Spec.Priv = nil
	return nil
}

// SetName sets session name
func (ws *WebSessionV1) SetName(name string) {
	ws.ID = name
}

// SetUser sets user associated with this session
func (ws *WebSessionV1) SetUser(u string) {
	ws.User = u
}

// GetUser returns the user this session is associated with
func (ws *WebSessionV1) GetUser() string {
	return ws.User
}

// GetName returns session name
func (ws *WebSessionV1) GetName() string {
	return ws.ID
}

// GetPub is returns public certificate signed by auth server
func (ws *WebSessionV1) GetPub() []byte {
	return ws.Pub
}

// GetPriv returns private OpenSSH key used to auth with SSH nodes
func (ws *WebSessionV1) GetPriv() []byte {
	return ws.Priv
}

// BearerToken is a special bearer token used for additional
// bearer authentication
func (ws *WebSessionV1) GetBearerToken() string {
	return ws.BearerToken
}

// Expires - absolute time when token expires
func (ws *WebSessionV1) GetExpiryTime() time.Time {
	return ws.Expires
}

// SetExpiryTime sets session expiry time
func (ws *WebSessionV1) SetExpiryTime(tm time.Time) {
	ws.Expires = tm
}

var webSessionMarshaler WebSessionMarshaler = &TeleportWebSessionMarshaler{}

// SetWebSessionMarshaler sets global user marshaler
func SetWebSessionMarshaler(u WebSessionMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	webSessionMarshaler = u
}

// GetWebSessionMarshaler returns currently set user marshaler
func GetWebSessionMarshaler() WebSessionMarshaler {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	return webSessionMarshaler
}

// WebSessionMarshaler implements marshal/unmarshal of User implementations
// mostly adds support for extended versions
type WebSessionMarshaler interface {
	// UnmarshalWebSession unmarhsals cert authority from binary representation
	UnmarshalWebSession(bytes []byte) (WebSession, error)
	// MarshalWebSession to binary representation
	MarshalWebSession(c WebSession, opts ...MarshalOption) ([]byte, error)
	// GenerateWebSession generates new web session and is used to
	// inject additional data in extenstions
	GenerateWebSession(WebSession) (WebSession, error)
	// ExtendWebSession extends web session and is used to
	// inject additional data in extenstions when session is getting renewed
	ExtendWebSession(WebSession) (WebSession, error)
}

// GetWebSessionSchema returns JSON Schema for web session
func GetWebSessionSchema() string {
	return GetWebSessionSchemaWithExtensions("")
}

// GetWebSessionSchemaWithExtensions returns JSON Schema for web session with user-supplied extensions
func GetWebSessionSchemaWithExtensions(extension string) string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, fmt.Sprintf(WebSessionSpecV2Schema, extension))
}

type TeleportWebSessionMarshaler struct{}

// GenerateWebSession generates new web session and is used to
// inject additional data in extenstions
func (*TeleportWebSessionMarshaler) GenerateWebSession(ws WebSession) (WebSession, error) {
	return ws, nil
}

// ExtendWebSession renews web session and is used to
// inject additional data in extenstions when session is getting renewed
func (*TeleportWebSessionMarshaler) ExtendWebSession(ws WebSession) (WebSession, error) {
	return ws, nil
}

// UnmarshalWebSession unmarshals web session from on-disk byte format
func (*TeleportWebSessionMarshaler) UnmarshalWebSession(bytes []byte) (WebSession, error) {
	var h ResourceHeader
	err := json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case "":
		var ws WebSessionV1
		err := json.Unmarshal(bytes, &ws)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ws.V2(), nil
	case V2:
		var ws WebSessionV2
		if err := utils.UnmarshalWithSchema(GetWebSessionSchema(), &ws, bytes); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		return &ws, nil
	}

	return nil, trace.BadParameter("web session resource version %v is not supported", h.Version)
}

// MarshalWebSession marshals web session into on-disk representation
func (*TeleportWebSessionMarshaler) MarshalWebSession(ca WebSession, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	type ws1 interface {
		V1() *WebSessionV1
	}
	type ws2 interface {
		V2() *WebSessionV2
	}
	version := cfg.GetVersion()
	switch version {
	case V1:
		v, ok := ca.(ws1)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V1)
		}
		return json.Marshal(v.V1())
	case V2:
		v, ok := ca.(ws2)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V2)
		}
		return json.Marshal(v.V2())
	default:
		return nil, trace.BadParameter("version %v is not supported", version)
	}
}
