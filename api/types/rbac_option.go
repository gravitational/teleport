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

	"github.com/gravitational/trace"
)

// FromRawOption describes a value that can be deserialized from a raw string option value.
type FromRawOption interface {
	// Name is the static name of the option type.
	Name() string

	// DeserializeInto deserializes the raw value into the receiver.
	DeserializeInto(raw string) error
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

type combinableOption[T any] interface {
	combineOptions(instances ...T)
}

func combineOptions[T combinableOption[T]](instances ...T) T {
	var combined T
	combined.combineOptions(instances...)
	return combined
}

type AccessPolicySessionTTL time.Duration

// Name is the static name of the option type.
func (o *AccessPolicySessionTTL) Name() string {
	return "session_ttl"
}

// DeserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySessionTTL) DeserializeInto(raw string) error {
	d, err := deserializeOptionDuration(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = AccessPolicySessionTTL(d)
	return nil
}

func (o *AccessPolicySessionTTL) combineOptions(instances ...AccessPolicySessionTTL) {
	*o = 0
	for _, instance := range instances {
		if instance > *o {
			*o = instance
		}
	}
}

type AccessPolicyLockingMode int

const (
	AccessPolicyLockingModeBestEffort AccessPolicyLockingMode = iota
	AccessPolicyLockingModeStrict
)

// Name is the static name of the option type.
func (o *AccessPolicyLockingMode) Name() string {
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

func (o *AccessPolicyLockingMode) combineOptions(instances ...AccessPolicyLockingMode) {
	*o = AccessPolicyLockingModeBestEffort
	for _, instance := range instances {
		if instance == AccessPolicyLockingModeStrict {
			*o = instance
			break
		}
	}
}

type AccessPolicySessionMFA bool

// Name is the static name of the option type.
func (o *AccessPolicySessionMFA) Name() string {
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

func (o *AccessPolicySessionMFA) combineOptions(instances ...AccessPolicySessionMFA) {
	*o = false
	for _, instance := range instances {
		if instance {
			*o = instance
			break
		}
	}
}

type AccessPolicySSHSessionRecordingMode int

const (
	AccessPolicySSHSessionRecordingModeBestEffort AccessPolicySSHSessionRecordingMode = iota
	AccessPolicySSHSessionRecordingModeStrict
)

// Name is the static name of the option type.
func (o *AccessPolicySSHSessionRecordingMode) Name() string {
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

func (o *AccessPolicySSHSessionRecordingMode) combineOptions(instances ...AccessPolicySSHSessionRecordingMode) {
	*o = AccessPolicySSHSessionRecordingModeBestEffort
	for _, instance := range instances {
		if instance == AccessPolicySSHSessionRecordingModeStrict {
			*o = instance
			break
		}
	}
}

type AccessPolicySSHAllowAgentForwarding bool

// Name is the static name of the option type.
func (o *AccessPolicySSHAllowAgentForwarding) Name() string {
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

func (o *AccessPolicySSHAllowAgentForwarding) combineOptions(instances ...AccessPolicySSHAllowAgentForwarding) {
	*o = false
	for _, instance := range instances {
		if instance {
			*o = instance
			break
		}
	}
}

type AccessPolicySSHAllowPortForwarding bool

// Name is the static name of the option type.
func (o *AccessPolicySSHAllowPortForwarding) Name() string {
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

func (o *AccessPolicySSHAllowPortForwarding) combineOptions(instances ...AccessPolicySSHAllowPortForwarding) {
	*o = true
	for _, instance := range instances {
		if !instance {
			*o = instance
			break
		}
	}
}

type AccessPolicySSHAllowX11Forwarding bool

// Name is the static name of the option type.
func (o *AccessPolicySSHAllowX11Forwarding) Name() string {
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

func (o *AccessPolicySSHAllowX11Forwarding) combineOptions(instances ...AccessPolicySSHAllowX11Forwarding) {
	*o = false
	for _, instance := range instances {
		if instance {
			*o = instance
			break
		}
	}
}

type AccessPolicySSHAllowFileCopying bool

// Name is the static name of the option type.
func (o *AccessPolicySSHAllowFileCopying) Name() string {
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

func (o *AccessPolicySSHAllowFileCopying) combineOptions(instances ...AccessPolicySSHAllowFileCopying) {
	*o = true
	for _, instance := range instances {
		if !instance {
			*o = instance
			break
		}
	}
}

type AccessPolicySSHAllowExpiredCert bool

// Name is the static name of the option type.
func (o *AccessPolicySSHAllowExpiredCert) Name() string {
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

func (o *AccessPolicySSHAllowExpiredCert) combineOptions(instances ...AccessPolicySSHAllowExpiredCert) {
	*o = true
	for _, instance := range instances {
		if !instance {
			*o = instance
			break
		}
	}
}

type AccessPolicySSHPinSourceIP bool

// Name is the static name of the option type.
func (o *AccessPolicySSHPinSourceIP) Name() string {
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

func (o *AccessPolicySSHPinSourceIP) combineOptions(instances ...AccessPolicySSHPinSourceIP) {
	*o = false
	for _, instance := range instances {
		if instance {
			*o = instance
			break
		}
	}
}

type AccessPolicySSHMaxConnections int

// Name is the static name of the option type.
func (o *AccessPolicySSHMaxConnections) Name() string {
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

func (o *AccessPolicySSHMaxConnections) combineOptions(instances ...AccessPolicySSHMaxConnections) {
	*o = 0
	for _, instance := range instances {
		if instance > *o {
			*o = instance
		}
	}
}

type AccessPolicySSHMaxSessionsPerConnection int

// Name is the static name of the option type.
func (o *AccessPolicySSHMaxSessionsPerConnection) Name() string {
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

func (o *AccessPolicySSHMaxSessionsPerConnection) combineOptions(instances ...AccessPolicySSHMaxSessionsPerConnection) {
	*o = 0
	for _, instance := range instances {
		if instance > *o {
			*o = instance
		}
	}
}

type AccessPolicySSHClientIdleTimeout time.Duration

// Name is the static name of the option type.
func (o *AccessPolicySSHClientIdleTimeout) Name() string {
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

func (o *AccessPolicySSHClientIdleTimeout) combineOptions(instances ...AccessPolicySSHClientIdleTimeout) {
	*o = 0
	for _, instance := range instances {
		if instance > *o {
			*o = instance
		}
	}
}
