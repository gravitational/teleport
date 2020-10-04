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

package services

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
)

// AppSession represents a specific application session. The existence of a
// session indicates that the Auth Server at one point checked the callers
// identity and granted access. Unlike services.WebSession which is shared
// with the user (in the form of a browser cookie), services.AppSession is not
// shared with the user and only used internally to verify an application
// exists and to attach any additional metadata about the identity of the user
// before forwarding the request to the target application.
type AppSession interface {
	// Resource represents common properties for resources.
	Resource

	// GetPublicAddr gets the public address this session is linked to.
	GetPublicAddr() string
	// SetPublicAddr sets the public address this session is linked to.
	SetPublicAddr(string)

	// GetUsername gets the Teleport username of the user to whom this session belongs.
	GetUsername() string
	// SetUsername sets the Teleport username of the user to whom this session belongs.
	SetUsername(string)

	// GetRoles gets the Teleport roles of the user to whom this session belongs.
	GetRoles() []string
	// SetRoles sets the Teleport roles of the user to whom this session belongs.
	SetRoles([]string)

	// GetJWT gets the JWT token that will be attached to every request.
	GetJWT() string
	// SetJWT sets the JWT token that will be attached to every request.
	SetJWT(string)

	// CheckAndSetDefaults validates the application session.
	CheckAndSetDefaults() error
}

// NewAppSession creates a new services.AppSession.
func NewAppSession(expires time.Time, spec AppSessionSpecV3) (AppSession, error) {
	session := &AppSessionV3{
		Kind:    KindAppSession,
		Version: V3,
		Metadata: Metadata{
			Name:      uuid.New(),
			Namespace: defaults.Namespace,
			Expires:   &expires,
		},
		Spec: spec,
	}
	if err := session.CheckAndSetDefaults(); err != nil {
		return nil, err
	}
	return session, nil
}

func (r *AppSessionV3) GetKind() string {
	return r.Kind
}

func (r *AppSessionV3) GetSubKind() string {
	return r.SubKind
}

func (r *AppSessionV3) SetSubKind(subKind string) {
	r.SubKind = subKind
}

func (r *AppSessionV3) GetVersion() string {
	return r.Version
}

func (r *AppSessionV3) GetName() string {
	return r.Metadata.Name
}

func (r *AppSessionV3) SetName(name string) {
	r.Metadata.Name = name
}

func (r *AppSessionV3) Expiry() time.Time {
	return r.Metadata.Expiry()
}

func (r *AppSessionV3) SetExpiry(expiry time.Time) {
	r.Metadata.SetExpiry(expiry)
}

func (r *AppSessionV3) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
}

func (r *AppSessionV3) GetMetadata() Metadata {
	return r.Metadata
}

func (r *AppSessionV3) GetResourceID() int64 {
	return r.Metadata.GetID()
}

func (r *AppSessionV3) SetResourceID(id int64) {
	r.Metadata.SetID(id)
}

// GetPublicAddr gets the public address this session is linked to.
func (s *AppSessionV3) GetPublicAddr() string {
	return s.Spec.PublicAddr
}

// SetPublicAddr sets the public address this session is linked to.
func (s *AppSessionV3) SetPublicAddr(publicAddr string) {
	s.Spec.PublicAddr = publicAddr
}

// GetUsername gets the Teleport username of the user to whom this session belongs.
func (s *AppSessionV3) GetUsername() string {
	return s.Spec.Username
}

// SetUsername sets the Teleport username of the user to whom this session belongs.
func (s *AppSessionV3) SetUsername(username string) {
	s.Spec.Username = username
}

// GetRoles gets the Teleport roles of the user to whom this session belongs.
func (s *AppSessionV3) GetRoles() []string {
	return s.Spec.Roles
}

// SetRoles sets the Teleport roles of the user to whom this session belongs.
func (s *AppSessionV3) SetRoles(roles []string) {
	s.Spec.Roles = roles
}

// GetJWT gets the JWT token that will be attached to every request.
func (s *AppSessionV3) GetJWT() string {
	return s.Spec.JWT
}

// SetJWT sets the JWT token that will be attached to every request.
func (s *AppSessionV3) SetJWT(jwt string) {
	s.Spec.JWT = jwt
}

// String returns the human readable representation of an application session.
func (s *AppSessionV3) String() string {
	return fmt.Sprintf("AppSession(%v)", s.Spec.PublicAddr)
}

// CheckAndSetDefaults validates the application session.
func (s *AppSessionV3) CheckAndSetDefaults() error {
	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if s.Spec.PublicAddr == "" {
		return trace.BadParameter("public address missing")
	}
	if s.Spec.Username == "" {
		return trace.BadParameter("username missing")
	}
	if len(s.Spec.Roles) == 0 {
		return trace.BadParameter("roles missing")
	}

	return nil
}

// AppSessionMarshaler defines an interface to marshal and unmarshal application sessions.
type AppSessionMarshaler interface {
	// MarshalAppSession will marshal and application session.
	MarshalAppSession(req AppSession, opts ...MarshalOption) ([]byte, error)
	// UnmarshalAppSession will unmarshal an application session.
	UnmarshalAppSession(bytes []byte, opts ...MarshalOption) (AppSession, error)
}

type appSessionMarshaler struct{}

// MarshalAppSession will marshal and application session.
func (m *appSessionMarshaler) MarshalAppSession(data AppSession, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch r := data.(type) {
	case *AppSessionV3:
		if !cfg.PreserveResourceID {
			// Avoid modifying the original object to prevent unexpected data races.
			cp := *r
			cp.SetResourceID(0)
			r = &cp
		}
		return utils.FastMarshal(r)
	default:
		return nil, trace.BadParameter("unrecognized plugin data type: %T", data)
	}
}

// UnmarshalAppSession will unmarshal an application session.
func (m *appSessionMarshaler) UnmarshalAppSession(raw []byte, opts ...MarshalOption) (AppSession, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var data AppSessionV3
	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(raw, &data); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := utils.UnmarshalWithSchema(GetAppSessionSchema(), &data, raw); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err := data.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		data.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		data.SetExpiry(cfg.Expires)
	}
	return &data, nil
}

var appSessionMarshalerInstance AppSessionMarshaler = &appSessionMarshaler{}

// GetAppSessionMarshaler returns an application session marshaler.
func GetAppSessionMarshaler() AppSessionMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return appSessionMarshalerInstance
}

// AppSessionSpecSchema is the schema for a services.AppSessionSpecV3.
const AppSessionSpecSchema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
		"public_addr": { "type":"string" },
		"username": { "type":"string" },
		"roles": { "type":"array" },
		"jwt": { "type":"string" }
	}
}`

// GetAppSessionSchema returns the JSON schema for the application session.
func GetAppSessionSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, AppSessionSpecSchema, DefaultDefinitions)
}

// CreateAppSessionRequest contains the parameters needed to request an
// application session.
type CreateAppSessionRequest struct {
	// PublicAddr is the public address of the requested application.
	PublicAddr string `json:"app"`
}

// Check validates the request.
func (r CreateAppSessionRequest) Check() error {
	if r.PublicAddr == "" {
		return trace.BadParameter("public address missing")
	}

	return nil
}
