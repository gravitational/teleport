/*
 *
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package auditd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"sync"
	"syscall"

	"github.com/gravitational/trace"
	"github.com/mdlayher/netlink"
	"github.com/mdlayher/netlink/nlenc"
	log "github.com/sirupsen/logrus"
)

// Client is auditd client.
type Client struct {
	conn NetlinkConnecter

	execName     string
	hostname     string
	systemUser   string
	teleportUser string
	address      string
	ttyName      string

	mtx     *sync.Mutex
	dial    func(family int, config *netlink.Config) (NetlinkConnecter, error)
	enabled bool
}

// auditStatus represent auditd status.
// Struct comes https://github.com/linux-audit/audit-userspace/blob/222dbaf5de27ab85e7aafcc7ea2cb68af2eab9b9/docs/audit_request_status.3#L19
// and has been updated to include fields added the kernel more recently.
type auditStatus struct {
	Mask                  uint32 /* Bit mask for valid entries */
	Enabled               uint32 /* 1 = enabled, 0 = disabled */
	Failure               uint32 /* Failure-to-log action */
	Pid                   uint32 /* pid of auditd process */
	RateLimit             uint32 /* messages rate limit (per second) */
	BacklogLimit          uint32 /* waiting messages limit */
	Lost                  uint32 /* messages lost */
	Backlog               uint32 /* messages waiting in queue */
	Version               uint32 /* audit api version number or feature bitmap */
	BacklogWaitTime       uint32 /* message queue wait timeout */
	BacklogWaitTimeActual uint32 /* message queue wait timeout */
}

// IsLoginUIDSet returns true if login UID is set, false otherwise.
func IsLoginUIDSet() bool {
	if !hasCapabilities() {
		// Current process doesn't have system permissions to talk to auditd.
		return false
	}

	client := NewClient(Message{})
	if client.connect() != nil {
		// connect returns an error when auditd is disabled,
		// or when we were not able to talk to it.
		return false
	}

	loginuid, err := getSelfLoginUID()
	if err != nil {
		log.WithError(err).Debug("failed to read login UID")
		return false
	}

	// if value is not set, logind PAM module will set it to the correct value
	// after fork.
	// 4294967295 is -1 converted to uint32
	return loginuid != 4294967295
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

func SendEvent(event EventType, result ResultType, msg Message) error {
	if !hasCapabilities() {
		// Disable auditd when not running as a root.
		return nil
	}

	msg.SetDefaults()

	auditd := NewClient(msg)
	defer func() {
		err := auditd.Close()
		if err != nil {
			log.WithError(err).Error("failed to close auditd client")
		}
	}()

	if err := auditd.SendMsg(event, result); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *Client) connect() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if !c.enabled && c.conn != nil {
		return ErrAuditdDisabled
	}

	conn, err := c.dial(syscall.NETLINK_AUDIT, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	c.conn = conn

	status, err := getAuditStatus(c.conn)
	if err != nil {
		return trace.Errorf("failed to get auditd status: %v", trace.ConvertSystemError(err))
	}

	c.enabled = status.Enabled == 1

	if status.Enabled != 1 {
		return ErrAuditdDisabled
	}

	return nil
}

func NewClient(msg Message) *Client {
	msg.SetDefaults()

	execName, err := os.Executable()
	if err != nil {
		log.WithError(err).Warn("failed to get executable name")
		execName = "?"
	}

	// Teleport never tries to get the hostname name.
	// Let's mimic the sshd behavior.
	const hostname = "?"

	return &Client{
		execName:     execName,
		hostname:     hostname,
		systemUser:   msg.SystemUser,
		teleportUser: msg.TeleportUser,
		address:      msg.ConnAddress,
		ttyName:      msg.TTYName,

		dial: func(family int, config *netlink.Config) (NetlinkConnecter, error) {
			return netlink.Dial(family, config)
		},
		mtx: &sync.Mutex{},
	}
}

func getAuditStatus(conn NetlinkConnecter) (*auditStatus, error) {
	_, err := conn.Execute(netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(AUDIT_GET),
			Flags: netlink.Request | netlink.Acknowledge,
		},
		Data: nil,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	msgs, err := conn.Receive()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(msgs) != 1 {
		return nil, trace.Errorf("returned wrong messages number, expected 1, got: %d", len(msgs))
	}

	byteOrder := nlenc.NativeEndian()
	status := &auditStatus{}

	payload := bytes.NewReader(msgs[0].Data[:])
	if err := binary.Read(payload, byteOrder, status); err != nil {
		return nil, trace.Wrap(err)
	}

	return status, nil
}

func (c *Client) SendMsg(event EventType, result ResultType) error {
	extraData := ""

	if c.teleportUser != "" {
		extraData += fmt.Sprintf("teleportUser=%s ", c.teleportUser)
	}

	const msgDataTmpl = "op=%s acct=\"%s\" exe=%s hostname=%s addr=%s terminal=%s %sres=%s"
	op := eventToOp(event)

	msgData := []byte(fmt.Sprintf(msgDataTmpl, op, c.systemUser, c.execName, c.hostname,
		c.address, c.ttyName, extraData, result))

	return trace.Wrap(c.sendMsg(netlink.HeaderType(event), msgData))
}

func (c *Client) sendMsg(eventType netlink.HeaderType, MsgData []byte) error {
	if err := c.connect(); err != nil {
		return trace.Wrap(err)
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
		return fmt.Errorf("unexpected number of responses from kernel for status request: %d, %v", len(resp), resp)
	}

	return nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func hasCapabilities() bool {
	return os.Getuid() == 0
}
