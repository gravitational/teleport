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

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"
)

// FromRawOption describes a value that can be deserialized from an intermediate option format.
type FromRawOption interface {
	// Name is the static name of the option type.
	Name() string

	deserializeInto(raw string) error
	fromRoleOptions(options RoleOptions) bool
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

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySessionTTL) deserializeInto(raw string) error {
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

func (o *AccessPolicySessionTTL) fromRoleOptions(options RoleOptions) bool {
	if options.MaxSessionTTL == 0 {
		return false
	}

	*o = AccessPolicySessionTTL(options.MaxSessionTTL)
	return true
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

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicyLockingMode) deserializeInto(raw string) error {
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

func (o *AccessPolicyLockingMode) fromRoleOptions(options RoleOptions) bool {
	switch options.Lock {
	case constants.LockingModeBestEffort:
		*o = AccessPolicyLockingModeBestEffort
	case constants.LockingModeStrict:
		*o = AccessPolicyLockingModeStrict
	default:
		return false
	}

	return true
}

type AccessPolicySessionMFA bool

// Name is the static name of the option type.
func (o *AccessPolicySessionMFA) Name() string {
	return "session_mfa"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySessionMFA) deserializeInto(raw string) error {
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

func (o *AccessPolicySessionMFA) fromRoleOptions(options RoleOptions) bool {
	*o = AccessPolicySessionMFA(options.RequireSessionMFA)
	return true
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

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHSessionRecordingMode) deserializeInto(raw string) error {
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

func (o *AccessPolicySSHSessionRecordingMode) fromRoleOptions(options RoleOptions) bool {
	switch options.Lock {
	case constants.LockingModeBestEffort:
		*o = AccessPolicySSHSessionRecordingModeBestEffort
	case constants.LockingModeStrict:
		*o = AccessPolicySSHSessionRecordingModeStrict
	default:
		return false
	}

	return true
}

type AccessPolicySSHAllowAgentForwarding bool

// Name is the static name of the option type.
func (o *AccessPolicySSHAllowAgentForwarding) Name() string {
	return "ssh.allow_agent_forwarding"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHAllowAgentForwarding) deserializeInto(raw string) error {
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

func (o *AccessPolicySSHAllowAgentForwarding) fromRoleOptions(options RoleOptions) bool {
	*o = AccessPolicySSHAllowAgentForwarding(options.ForwardAgent)
	return true
}

type AccessPolicySSHAllowPortForwarding bool

// Name is the static name of the option type.
func (o *AccessPolicySSHAllowPortForwarding) Name() string {
	return "ssh.allow_port_forwarding"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHAllowPortForwarding) deserializeInto(raw string) error {
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

func (o *AccessPolicySSHAllowPortForwarding) fromRoleOptions(options RoleOptions) bool {
	if options.PortForwarding == nil {
		return false
	}

	*o = AccessPolicySSHAllowPortForwarding(options.PortForwarding.Value)
	return true
}

type AccessPolicySSHAllowX11Forwarding bool

// Name is the static name of the option type.
func (o *AccessPolicySSHAllowX11Forwarding) Name() string {
	return "ssh.allow_x11_forwarding"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHAllowX11Forwarding) deserializeInto(raw string) error {
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

func (o *AccessPolicySSHAllowX11Forwarding) fromRoleOptions(options RoleOptions) bool {
	*o = AccessPolicySSHAllowX11Forwarding(options.PermitX11Forwarding)
	return true
}

type AccessPolicySSHAllowFileCopying bool

// Name is the static name of the option type.
func (o *AccessPolicySSHAllowFileCopying) Name() string {
	return "ssh.allow_file_copying"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHAllowFileCopying) deserializeInto(raw string) error {
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

func (o *AccessPolicySSHAllowFileCopying) fromRoleOptions(options RoleOptions) bool {
	if options.SSHFileCopy == nil {
		return false
	}

	*o = AccessPolicySSHAllowFileCopying(options.SSHFileCopy.Value)
	return true
}

type AccessPolicySSHAllowExpiredCert bool

// Name is the static name of the option type.
func (o *AccessPolicySSHAllowExpiredCert) Name() string {
	return "ssh.allow_expired_cert"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHAllowExpiredCert) deserializeInto(raw string) error {
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

func (o *AccessPolicySSHAllowExpiredCert) fromRoleOptions(options RoleOptions) bool {
	*o = AccessPolicySSHAllowExpiredCert(options.DisconnectExpiredCert)
	return true
}

type AccessPolicySSHPinSourceIP bool

// Name is the static name of the option type.
func (o *AccessPolicySSHPinSourceIP) Name() string {
	return "ssh.pin_source_ip"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHPinSourceIP) deserializeInto(raw string) error {
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

func (o *AccessPolicySSHPinSourceIP) fromRoleOptions(options RoleOptions) bool {
	*o = AccessPolicySSHPinSourceIP(options.PinSourceIP)
	return true
}

type AccessPolicySSHMaxConnections int

// Name is the static name of the option type.
func (o *AccessPolicySSHMaxConnections) Name() string {
	return "ssh.max_connections"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHMaxConnections) deserializeInto(raw string) error {
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

func (o *AccessPolicySSHMaxConnections) fromRoleOptions(options RoleOptions) bool {
	*o = AccessPolicySSHMaxConnections(options.MaxConnections)
	return true
}

type AccessPolicySSHMaxSessionsPerConnection int

// Name is the static name of the option type.
func (o *AccessPolicySSHMaxSessionsPerConnection) Name() string {
	return "ssh.max_sessions_per_connection"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHMaxSessionsPerConnection) deserializeInto(raw string) error {
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

func (o *AccessPolicySSHMaxSessionsPerConnection) fromRoleOptions(options RoleOptions) bool {
	*o = AccessPolicySSHMaxSessionsPerConnection(options.MaxSessions)
	return true
}

type AccessPolicySSHClientIdleTimeout time.Duration

// Name is the static name of the option type.
func (o *AccessPolicySSHClientIdleTimeout) Name() string {
	return "ssh.client_idle_timeout"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *AccessPolicySSHClientIdleTimeout) deserializeInto(raw string) error {
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

func (o *AccessPolicySSHClientIdleTimeout) fromRoleOptions(options RoleOptions) bool {
	*o = AccessPolicySSHClientIdleTimeout(options.ClientIdleTimeout)
	return true
}
