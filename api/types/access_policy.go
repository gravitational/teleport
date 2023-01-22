/*
Copyright 2022 Gravitational, Inc.

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
	"strconv"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/trace"
)

// AccessPolicyV1 is a predicate policy used for RBAC, similar to rule but uses predicate language.
type AccessPolicy interface {
	// Resource provides common resource properties
	Resource
	// GetAllow returns a list of allow expressions grouped by scope.
	GetAllow() map[string]string
	// GetDeny returns a list of deny expressions grouped by scope.
	GetDeny() map[string]string
}

// AsOption describes a value that can be deserialized from a predicate option raw value.
type AsOption interface {
	// Name is the static name of the option type.
	Name() string

	// DeserializeInto deserializes the raw value into the receiver.
	DeserializeInto(raw string) error
}

// AccessPolicyOption retrieves and attempts to deserialize it into the provided type.
func AccessPolicyOption[T AsOption](policy *AccessPolicyV1) (T, error) {
	var opt T
	raw, ok := policy.Spec.Options[opt.Name()]
	if !ok {
		// TODO: default here=
		return opt, trace.NotFound("option %q not found", opt.Name())
	}

	if err := opt.DeserializeInto(raw); err != nil {
		return opt, trace.Wrap(err)
	}

	return opt, nil
}

func deserializeOptionDuration(raw string) (time.Duration, error) {
	i, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, trace.BadParameter("invalid duration %q", raw)
	}

	return time.Duration(i), nil
}

func deserializeOptionBool(raw string) (bool, error) {
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, trace.BadParameter("invalid duration %q", raw)
	}

	return b, nil
}

func deserializeOptionInt(raw string) (int, error) {
	b, err := strconv.Atoi(raw)
	if err != nil {
		return 0, trace.BadParameter("invalid integer %q", raw)
	}

	return b, nil
}

type AccessPolicySessionTTL time.Duration

func (o *AccessPolicySessionTTL) Name() string {
	// Name is the static name of the option type.
	return "session_ttl"
}

func (o *AccessPolicySessionTTL) DeserializeInto(raw string) error {
	d, err := deserializeOptionDuration(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySessionTTL(d)
	return nil
}

type AccessPolicyLockingMode int

const (
	AccessPolicyLockingModeBestEffort AccessPolicyLockingMode = iota
	AccessPolicyLockingModeStrict
)

func (o *AccessPolicyLockingMode) Name() string {
	// Name is the static name of the option type.
	return "locking_mode"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicyLockingMode) DeserializeInto(raw string) error {
	switch raw {
	case "best_effort":
		*o = AccessPolicyLockingModeBestEffort
		return nil
	case "strict":
		*o = AccessPolicyLockingModeStrict
		return nil
	default:
		return trace.BadParameter("invalid locking mode %q", raw)
	}
}

type AccessPolicySessionMFA bool

func (o *AccessPolicySessionMFA) Name() string {
	// Name is the static name of the option type.
	return "session_mfa"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySessionMFA) DeserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySessionMFA(b)
	return nil
}

type AccessPolicySSHSessionRecordingMode int

const (
	AccessPolicySSHSessionRecordingModeBestEffort AccessPolicySSHSessionRecordingMode = iota
	AccessPolicySSHSessionRecordingModeStrict
)

func (o *AccessPolicySSHSessionRecordingMode) Name() string {
	// Name is the static name of the option type.
	return "ssh.session_recording_mode"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHSessionRecordingMode) DeserializeInto(raw string) error {
	switch raw {
	case "best_effort":
		*o = AccessPolicySSHSessionRecordingModeBestEffort
		return nil
	case "strict":
		*o = AccessPolicySSHSessionRecordingModeStrict
		return nil
	default:
		return trace.BadParameter("invalid session recording mode %q", raw)
	}
}

type AccessPolicySSHAllowAgentForwarding bool

func (o *AccessPolicySSHAllowAgentForwarding) Name() string {
	// Name is the static name of the option type.
	return "ssh.allow_agent_forwarding"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHAllowAgentForwarding) DeserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySSHAllowAgentForwarding(b)
	return nil
}

type AccessPolicySSHAllowPortForwarding bool

func (o *AccessPolicySSHAllowPortForwarding) Name() string {
	// Name is the static name of the option type.
	return "ssh.allow_port_forwarding"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHAllowPortForwarding) DeserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySSHAllowPortForwarding(b)
	return nil
}

type AccessPolicySSHAllowX11Forwarding bool

func (o *AccessPolicySSHAllowX11Forwarding) Name() string {
	// Name is the static name of the option type.
	return "ssh.allow_x11_forwarding"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHAllowX11Forwarding) DeserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySSHAllowX11Forwarding(b)
	return nil
}

type AccessPolicySSHAllowFileCopying bool

func (o *AccessPolicySSHAllowFileCopying) Name() string {
	// Name is the static name of the option type.
	return "ssh.allow_file_copying"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHAllowFileCopying) DeserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySSHAllowFileCopying(b)
	return nil
}

type AccessPolicySSHAllowExpiredCert bool

func (o *AccessPolicySSHAllowExpiredCert) Name() string {
	// Name is the static name of the option type.
	return "ssh.allow_expired_cert"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHAllowExpiredCert) DeserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySSHAllowExpiredCert(b)
	return nil
}

type AccessPolicySSHPinSourceIP bool

func (o *AccessPolicySSHPinSourceIP) Name() string {
	// Name is the static name of the option type.
	return "ssh.pin_source_ip"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHPinSourceIP) DeserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySSHPinSourceIP(b)
	return nil
}

type AccessPolicySSHMaxConnections int

func (o *AccessPolicySSHMaxConnections) Name() string {
	// Name is the static name of the option type.
	return "ssh.max_connections"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHMaxConnections) DeserializeInto(raw string) error {
	i, err := deserializeOptionInt(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySSHMaxConnections(i)
	return nil
}

type AccessPolicySSHMaxSessionsPerConnection int

func (o *AccessPolicySSHMaxSessionsPerConnection) Name() string {
	// Name is the static name of the option type.
	return "ssh.max_sessions_per_connection"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHMaxSessionsPerConnection) DeserializeInto(raw string) error {
	i, err := deserializeOptionInt(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySSHMaxSessionsPerConnection(i)
	return nil
}

type AccessPolicySSHClientIdleTimeout time.Duration

func (o *AccessPolicySSHClientIdleTimeout) Name() string {
	// Name is the static name of the option type.
	return "ssh.client_idle_timeout"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHClientIdleTimeout) DeserializeInto(raw string) error {
	d, err := deserializeOptionDuration(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySSHClientIdleTimeout(d)
	return nil
}

// NewAccessPolicy creates a new access policy from a specification.
func NewAccessPolicy(name string, spec AccessPolicySpecV1) *AccessPolicyV1 {
	return &AccessPolicyV1{
		Kind:    KindAccessPolicy,
		Version: V1,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// GetVersion returns resource version
func (c *AccessPolicyV1) GetVersion() string {
	return c.Version
}

// GetKind returns resource kind
func (c *AccessPolicyV1) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource sub kind
func (c *AccessPolicyV1) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *AccessPolicyV1) SetSubKind(s string) {
	c.SubKind = s
}

// GetResourceID returns resource ID
func (c *AccessPolicyV1) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *AccessPolicyV1) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// setStaticFields sets static resource header and metadata fields.
func (c *AccessPolicyV1) setStaticFields() {
	c.Kind = KindAccessPolicy
	c.Version = V1
}

// CheckAndSetDefaults checks and sets default values
func (c *AccessPolicyV1) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetMetadata returns object metadata
func (c *AccessPolicyV1) GetMetadata() Metadata {
	return c.Metadata
}

// SetMetadata sets remote cluster metatada
func (c *AccessPolicyV1) SetMetadata(meta Metadata) {
	c.Metadata = meta
}

// SetExpiry sets expiry time for the object
func (c *AccessPolicyV1) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (c *AccessPolicyV1) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetName returns the name of the RemoteCluster.
func (c *AccessPolicyV1) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the RemoteCluster.
func (c *AccessPolicyV1) SetName(e string) {
	c.Metadata.Name = e
}

// GetAllow returns a list of allow expressions grouped by scope.
func (c *AccessPolicyV1) GetAllow() map[string]string {
	return c.Spec.Allow
}

// GetDeny returns a list of deny expressions grouped by scope.
func (c *AccessPolicyV1) GetDeny() map[string]string {
	return c.Spec.Deny
}
