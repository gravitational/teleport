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

package auditd

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"math"
	"os"
	"strconv"
	"sync"
	"syscall"
	"text/template"

	"github.com/gravitational/trace"
	"github.com/mdlayher/netlink"
	"github.com/mdlayher/netlink/nlenc"
)

// featureStatus is a 3 state boolean yes/no/unknown type.
type featureStatus int

const (
	unset featureStatus = iota
	disabled
	enabled
)

const msgDataTmpl = `op={{ .Opcode }} acct="{{ .Msg.SystemUser }}" exe="{{ .Exe }}" ` +
	`hostname={{ .Hostname }} addr={{ .Msg.ConnAddress }} terminal={{ .Msg.TTYName }} ` +
	`{{if .Msg.TeleportUser}}teleportUser={{.Msg.TeleportUser}} {{end}}res={{ .Result }}`

var messageTmpl = template.Must(template.New("auditd-message").Parse(msgDataTmpl))

// Client is auditd client.
type Client struct {
	conn NetlinkConnector

	execName     string
	hostname     string
	systemUser   string
	teleportUser string
	address      string
	ttyName      string

	mtx     sync.Mutex
	dial    func(family int, config *netlink.Config) (NetlinkConnector, error)
	enabled featureStatus
}

// auditStatus represent auditd status.
// Struct comes https://github.com/linux-audit/audit-userspace/blob/222dbaf5de27ab85e7aafcc7ea2cb68af2eab9b9/docs/audit_request_status.3#L19
type auditStatus struct {
	Mask         uint32 /* Bit mask for valid entries */
	Enabled      uint32 /* 1 = enabled, 0 = disabled */
	Failure      uint32 /* Failure-to-log action */
	PID          uint32 /* pid of auditd process */
	RateLimit    uint32 /* messages rate limit (per second) */
	BacklogLimit uint32 /* waiting messages limit */
	Lost         uint32 /* messages lost */
	Backlog      uint32 /* messages waiting in queue */
	// Newer kernels have more fields, but adding them here will cause
	// compatibility issues with older kernels (3.10 for example).
	// ref: https://github.com/gravitational/teleport/issues/16267
}

// IsLoginUIDSet returns true if login UID is set, false otherwise.
func IsLoginUIDSet() bool {
	if !hasCapabilities() {
		// Current process doesn't have system permissions to talk to auditd.
		return false
	}

	client := NewClient(Message{})
	defer func() {
		if err := client.Close(); err != nil {
			slog.WarnContext(context.TODO(), "Failed to close auditd client", "error", err)
		}
	}()
	// We don't need to acquire the internal client mutex as the connection is
	// not shared.
	if err := client.connectUnderMutex(); err != nil {
		return false
	}

	enabled, err := client.isEnabledUnderMutex()
	if err != nil || !enabled {
		return false
	}

	loginuid, err := getSelfLoginUID()
	if err != nil {
		slog.DebugContext(context.TODO(), "Failed to read login UID", "error", err)
		return false
	}

	// if value is not set, logind PAM module will set it to the correct value
	// after fork. 4294967295 (math.MaxUint32) is -1 converted to uint32.
	return loginuid != math.MaxUint32
}

func getSelfLoginUID() (int64, error) {
	data, err := os.ReadFile("/proc/self/loginuid")
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}

	loginuid, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return loginuid, nil
}

// SendEvent sends a single auditd event. Each request create a new netlink connection.
// This function does not send the event and returns no error if it runs with no root permissions.
func SendEvent(event EventType, result ResultType, msg Message) error {
	if !hasCapabilities() {
		// Do nothing when not running as root.
		return nil
	}

	client := NewClient(msg)
	defer func() {
		err := client.Close()
		if err != nil {
			slog.ErrorContext(context.TODO(), "Failed to close auditd client", "error", err)
		}
	}()

	if err := client.SendMsg(event, result); err != nil {
		if errors.Is(err, ErrAuditdDisabled) || isPermissionError(err) {
			// Do not return the error to the caller if auditd is disabled,
			// or we don't have required permissions to use it.
			return nil
		}
		return trace.Wrap(err)
	}

	return nil
}

// isPermissionError returns true if we lack permission to talk to Linux Audit System.
func isPermissionError(err error) bool {
	return errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EPROTONOSUPPORT)
}

func (c *Client) connectUnderMutex() error {
	if c.conn != nil {
		// Already connected, return
		return nil
	}

	conn, err := c.dial(syscall.NETLINK_AUDIT, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	c.conn = conn

	return nil
}

func (c *Client) isEnabledUnderMutex() (bool, error) {
	if c.enabled != unset {
		// We've already gotten the status.
		return c.enabled == enabled, nil
	}

	status, err := getAuditStatus(c.conn)
	if err != nil {
		return false, trace.Errorf("failed to get auditd status: %w", trace.ConvertSystemError(err))
	}

	// enabled can be either 1 or 2 if enabled, 0 otherwise
	if status.Enabled > 0 {
		c.enabled = enabled
	} else {
		c.enabled = disabled
	}

	return c.enabled == enabled, nil
}

// NewClient creates a new auditd client. Client is not connected when it is returned.
func NewClient(msg Message) *Client {
	msg.SetDefaults()

	execName, err := os.Executable()
	if err != nil {
		slog.WarnContext(context.TODO(), "Failed to get executable name", "error", err)
		execName = UnknownValue
	}

	// Teleport never tries to get the hostname name.
	// Let's mimic the sshd behavior.
	const hostname = UnknownValue

	return &Client{
		execName:     execName,
		hostname:     hostname,
		systemUser:   msg.SystemUser,
		teleportUser: msg.TeleportUser,
		address:      msg.ConnAddress,
		ttyName:      msg.TTYName,

		dial: func(family int, config *netlink.Config) (NetlinkConnector, error) {
			return netlink.Dial(family, config)
		},
	}
}

func getAuditStatus(conn NetlinkConnector) (*auditStatus, error) {
	_, err := conn.Execute(netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(AuditGet),
			Flags: netlink.Request | netlink.Acknowledge,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	msgs, err := conn.Receive()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(msgs) != 1 {
		return nil, trace.BadParameter("returned wrong messages number, expected 1, got: %d", len(msgs))
	}

	// auditd marshaling depends on the system architecture.
	byteOrder := nlenc.NativeEndian()
	status := &auditStatus{}

	payload := bytes.NewReader(msgs[0].Data[:])
	if err := binary.Read(payload, byteOrder, status); err != nil {
		return nil, trace.Wrap(err)
	}

	return status, nil
}

// SendMsg sends a message. Client will create a new connection if not connected already.
func (c *Client) SendMsg(event EventType, result ResultType) error {
	op := eventToOp(event)
	buf := &bytes.Buffer{}

	if err := messageTmpl.Execute(buf,
		struct {
			Result   ResultType
			Opcode   string
			Exe      string
			Hostname string
			Msg      Message
		}{
			Opcode:   op,
			Result:   result,
			Exe:      c.execName,
			Hostname: c.hostname,
			Msg: Message{
				SystemUser:   c.systemUser,
				TeleportUser: c.teleportUser,
				ConnAddress:  c.address,
				TTYName:      c.ttyName,
			},
		}); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(c.sendMsg(netlink.HeaderType(event), buf.Bytes()))
}

func (c *Client) sendMsg(eventType netlink.HeaderType, MsgData []byte) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if err := c.connectUnderMutex(); err != nil {
		return trace.Wrap(err)
	}

	enabled, err := c.isEnabledUnderMutex()
	if err != nil {
		return trace.Wrap(err)
	}

	if !enabled {
		return ErrAuditdDisabled
	}

	msg := netlink.Message{
		Header: netlink.Header{
			Type:  eventType,
			Flags: syscall.NLM_F_REQUEST | syscall.NLM_F_ACK,
		},
		Data: MsgData,
	}

	resp, err := c.conn.Execute(msg)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(resp) != 1 {
		return trace.Errorf("unexpected number of responses from kernel for status request: %d, %v", len(resp), resp)
	}

	return nil
}

// Close closes the underlying netlink connection and resets the struct state.
func (c *Client) Close() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	var err error

	if c.conn != nil {
		err = c.conn.Close()
		// reset to avoid a potential use of closed connection.
		c.conn = nil
	}

	c.enabled = unset

	return err
}

func eventToOp(event EventType) string {
	switch event {
	case AuditUserEnd:
		return "session_close"
	case AuditUserLogin:
		return "login"
	case AuditUserErr:
		return "invalid_user"
	default:
		return UnknownValue
	}
}

// hasCapabilities returns true if the OS process has permission to
// write to auditd events log.
// Currently, we require the process to run as a root.
func hasCapabilities() bool {
	return os.Getuid() == 0
}
