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
	"sync"
	"syscall"

	"github.com/mdlayher/netlink"
	"github.com/mdlayher/netlink/nlenc"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

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

type auditStatus struct {
	Mask                  uint32 /* Bit mask for valid entries */
	Enabled               uint32 /* 1 = enabled, 0 = disabled */
	Failure               uint32 /* Failure-to-log action */
	Pid                   uint32 /* pid of auditd process */
	RateLimit             uint32 /* messages rate limit (per second) */
	BacklogLimit          uint32 /* waiting messages limit */
	Lost                  uint32 /* messages lost */
	Backlog               uint32 /* messages waiting in queue */
	Version               uint32 /* audit api version number */ // feature bitmap
	BacklogWaitTime       uint32 /* message queue wait timeout */
	BacklogWaitTimeActual uint32 /* message queue wait timeout */
}

func SendEvent(event EventType, result ResultType, msg Message) error {
	if os.Getuid() != 0 {
		// Disable Auditd when not running as a root.
		return nil
	}

	msg.SetDefaults()

	auditd := NewAuditDClient(msg)
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

	status, err := c.getAuditStatus()
	if err != nil {
		return trace.Errorf("failed to get audutd state: %v", trace.ConvertSystemError(err))
	}

	c.enabled = status.Enabled == 1

	if status.Enabled != 1 {
		return ErrAuditdDisabled
	}

	return nil
}

func NewAuditDClient(msg Message) *Client {
	msg.SetDefaults()

	execName, err := os.Executable()
	if err != nil {
		log.WithError(err).Warn("failed to get executable name")
		execName = "?"
	}

	// Match sshd
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

func (c *Client) getAuditStatus() (*auditStatus, error) {
	_, err := c.conn.Execute(netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(AUDIT_GET),
			Flags: netlink.Request | netlink.Acknowledge,
		},
		Data: nil,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	msgs, err := c.conn.Receive()
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
