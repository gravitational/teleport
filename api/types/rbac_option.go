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
type FromRawOption[T any] interface {
	// Name is the static name of the option type.
	Name() string

	deserializeInto(raw string) error
	combineOptions(instances ...T)
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

func CombineOptions[T FromRawOption[T]](instances ...T) T {
	var combined T
	combined.combineOptions(instances...)
	return combined
}

type SessionTTL time.Duration

// Name is the static name of the option type.
func (o *SessionTTL) Name() string {
	return "session_ttl"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SessionTTL) deserializeInto(raw string) error {
	d, err := deserializeOptionDuration(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = SessionTTL(d)
	return nil
}

func (o *SessionTTL) combineOptions(instances ...*SessionTTL) {
	*o = 0
	for _, instance := range instances {
		if *instance > *o {
			*o = *instance
		}
	}
}

func (o *SessionTTL) fromRoleOptions(options RoleOptions) bool {
	if options.MaxSessionTTL == 0 {
		return false
	}

	*o = SessionTTL(options.MaxSessionTTL)
	return true
}

type LockingMode int

const (
	LockingModeBestEffort LockingMode = iota
	LockingModeStrict
)

// Name is the static name of the option type.
func (o *LockingMode) Name() string {
	return "locking_mode"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *LockingMode) deserializeInto(raw string) error {
	switch raw {
	case "best_effort":
		*o = LockingModeBestEffort
		return nil
	case "strict":
		*o = LockingModeStrict
		return nil
	default:
		return trace.BadParameter("invalid locking mode %q", raw)
	}
}

func (o *LockingMode) combineOptions(instances ...*LockingMode) {
	*o = LockingModeBestEffort
	for _, instance := range instances {
		if *instance == LockingModeStrict {
			*o = *instance
			break
		}
	}
}

func (o *LockingMode) fromRoleOptions(options RoleOptions) bool {
	switch options.Lock {
	case constants.LockingModeBestEffort:
		*o = LockingModeBestEffort
	case constants.LockingModeStrict:
		*o = LockingModeStrict
	default:
		return false
	}

	return true
}

type SessionMFA bool

// Name is the static name of the option type.
func (o *SessionMFA) Name() string {
	return "session_mfa"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SessionMFA) deserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = SessionMFA(b)
	return nil
}

func (o *SessionMFA) combineOptions(instances ...*SessionMFA) {
	*o = false
	for _, instance := range instances {
		if *instance {
			*o = *instance
			break
		}
	}
}

func (o *SessionMFA) fromRoleOptions(options RoleOptions) bool {
	*o = SessionMFA(options.RequireSessionMFA)
	return true
}

type SSHSessionRecordingMode int

const (
	SSHSessionRecordingModeBestEffort SSHSessionRecordingMode = iota
	SSHSessionRecordingModeStrict
)

// Name is the static name of the option type.
func (o *SSHSessionRecordingMode) Name() string {
	return "ssh.session_recording_mode"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SSHSessionRecordingMode) deserializeInto(raw string) error {
	switch raw {
	case "best_effort":
		*o = SSHSessionRecordingModeBestEffort
		return nil
	case "strict":
		*o = SSHSessionRecordingModeStrict
		return nil
	default:
		return trace.BadParameter("invalid session recording mode %q", raw)
	}
}

func (o *SSHSessionRecordingMode) combineOptions(instances ...*SSHSessionRecordingMode) {
	*o = SSHSessionRecordingModeBestEffort
	for _, instance := range instances {
		if *instance == SSHSessionRecordingModeStrict {
			*o = *instance
			break
		}
	}
}

func (o *SSHSessionRecordingMode) fromRoleOptions(options RoleOptions) bool {
	switch options.Lock {
	case constants.LockingModeBestEffort:
		*o = SSHSessionRecordingModeBestEffort
	case constants.LockingModeStrict:
		*o = SSHSessionRecordingModeStrict
	default:
		return false
	}

	return true
}

type SSHAllowAgentForwarding bool

// Name is the static name of the option type.
func (o *SSHAllowAgentForwarding) Name() string {
	return "ssh.allow_agent_forwarding"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SSHAllowAgentForwarding) deserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = SSHAllowAgentForwarding(b)
	return nil
}

func (o *SSHAllowAgentForwarding) combineOptions(instances ...*SSHAllowAgentForwarding) {
	*o = false
	for _, instance := range instances {
		if *instance {
			*o = *instance
			break
		}
	}
}

func (o *SSHAllowAgentForwarding) fromRoleOptions(options RoleOptions) bool {
	*o = SSHAllowAgentForwarding(options.ForwardAgent)
	return true
}

type SSHAllowPortForwarding bool

// Name is the static name of the option type.
func (o *SSHAllowPortForwarding) Name() string {
	return "ssh.allow_port_forwarding"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SSHAllowPortForwarding) deserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = SSHAllowPortForwarding(b)
	return nil
}

func (o *SSHAllowPortForwarding) combineOptions(instances ...*SSHAllowPortForwarding) {
	*o = true
	for _, instance := range instances {
		if !*instance {
			*o = *instance
			break
		}
	}
}

func (o *SSHAllowPortForwarding) fromRoleOptions(options RoleOptions) bool {
	if options.PortForwarding == nil {
		return false
	}

	*o = SSHAllowPortForwarding(options.PortForwarding.Value)
	return true
}

type SSHAllowX11Forwarding bool

// Name is the static name of the option type.
func (o *SSHAllowX11Forwarding) Name() string {
	return "ssh.allow_x11_forwarding"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SSHAllowX11Forwarding) deserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = SSHAllowX11Forwarding(b)
	return nil
}

func (o *SSHAllowX11Forwarding) combineOptions(instances ...*SSHAllowX11Forwarding) {
	*o = false
	for _, instance := range instances {
		if *instance {
			*o = *instance
			break
		}
	}
}

func (o *SSHAllowX11Forwarding) fromRoleOptions(options RoleOptions) bool {
	*o = SSHAllowX11Forwarding(options.PermitX11Forwarding)
	return true
}

type SSHAllowFileCopying bool

// Name is the static name of the option type.
func (o *SSHAllowFileCopying) Name() string {
	return "ssh.allow_file_copying"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SSHAllowFileCopying) deserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = SSHAllowFileCopying(b)
	return nil
}

func (o *SSHAllowFileCopying) combineOptions(instances ...*SSHAllowFileCopying) {
	*o = true
	for _, instance := range instances {
		if !*instance {
			*o = *instance
			break
		}
	}
}

func (o *SSHAllowFileCopying) fromRoleOptions(options RoleOptions) bool {
	if options.SSHFileCopy == nil {
		return false
	}

	*o = SSHAllowFileCopying(options.SSHFileCopy.Value)
	return true
}

type SSHAllowExpiredCert bool

// Name is the static name of the option type.
func (o *SSHAllowExpiredCert) Name() string {
	return "ssh.allow_expired_cert"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SSHAllowExpiredCert) deserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = SSHAllowExpiredCert(b)
	return nil
}

func (o *SSHAllowExpiredCert) combineOptions(instances ...*SSHAllowExpiredCert) {
	*o = true
	for _, instance := range instances {
		if !*instance {
			*o = *instance
			break
		}
	}
}

func (o *SSHAllowExpiredCert) fromRoleOptions(options RoleOptions) bool {
	*o = SSHAllowExpiredCert(options.DisconnectExpiredCert)
	return true
}

type SSHPinSourceIP bool

// Name is the static name of the option type.
func (o *SSHPinSourceIP) Name() string {
	return "ssh.pin_source_ip"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SSHPinSourceIP) deserializeInto(raw string) error {
	b, err := deserializeOptionBool(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = SSHPinSourceIP(b)
	return nil
}

func (o *SSHPinSourceIP) combineOptions(instances ...*SSHPinSourceIP) {
	*o = false
	for _, instance := range instances {
		if *instance {
			*o = *instance
			break
		}
	}
}

func (o *SSHPinSourceIP) fromRoleOptions(options RoleOptions) bool {
	*o = SSHPinSourceIP(options.PinSourceIP)
	return true
}

type SSHMaxConnections int

// Name is the static name of the option type.
func (o *SSHMaxConnections) Name() string {
	return "ssh.max_connections"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SSHMaxConnections) deserializeInto(raw string) error {
	i, err := deserializeOptionInt(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = SSHMaxConnections(i)
	return nil
}

func (o *SSHMaxConnections) combineOptions(instances ...*SSHMaxConnections) {
	*o = 0
	for _, instance := range instances {
		if *instance > *o {
			*o = *instance
		}
	}
}

func (o *SSHMaxConnections) fromRoleOptions(options RoleOptions) bool {
	*o = SSHMaxConnections(options.MaxConnections)
	return true
}

type SSHMaxSessionsPerConnection int

// Name is the static name of the option type.
func (o *SSHMaxSessionsPerConnection) Name() string {
	return "ssh.max_sessions_per_connection"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SSHMaxSessionsPerConnection) deserializeInto(raw string) error {
	i, err := deserializeOptionInt(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = SSHMaxSessionsPerConnection(i)
	return nil
}

func (o *SSHMaxSessionsPerConnection) combineOptions(instances ...*SSHMaxSessionsPerConnection) {
	*o = 0
	for _, instance := range instances {
		if *instance > *o {
			*o = *instance
		}
	}
}

func (o *SSHMaxSessionsPerConnection) fromRoleOptions(options RoleOptions) bool {
	*o = SSHMaxSessionsPerConnection(options.MaxSessions)
	return true
}

type SSHClientIdleTimeout time.Duration

// Name is the static name of the option type.
func (o *SSHClientIdleTimeout) Name() string {
	return "ssh.client_idle_timeout"
}

// deserializeInto deserializes the raw value into the receiver.
func (o *SSHClientIdleTimeout) deserializeInto(raw string) error {
	d, err := deserializeOptionDuration(raw)
	if err != nil {
		return trace.Wrap(err)
	}

	*o = SSHClientIdleTimeout(d)
	return nil
}

func (o *SSHClientIdleTimeout) combineOptions(instances ...*SSHClientIdleTimeout) {
	*o = 0
	for _, instance := range instances {
		if *instance > *o {
			*o = *instance
		}
	}
}

func (o *SSHClientIdleTimeout) fromRoleOptions(options RoleOptions) bool {
	*o = SSHClientIdleTimeout(options.ClientIdleTimeout)
	return true
}
