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

// sshd
// type=USER_LOGIN msg=audit(1657493525.955:466): pid=16059 uid=0 auid=4294967295 ses=4294967295 subj==unconfined msg='op=login acct="jnyckowski" exe="/usr/sbin/sshd" hostname=? addr=127.0.0.1 terminal=sshd res=failed'UID="root" AUID="unset"
// type=USER_START msg=audit(1657493584.668:474): pid=16059 uid=0 auid=1000 ses=11 subj==unconfined msg='op=PAM:session_open grantors=pam_selinux,pam_loginuid,pam_keyinit,pam_permit,pam_umask,pam_unix,pam_systemd,pam_mail,pam_limits,pam_env,pam_env,pam_selinux,pam_tty_audit acct="jnyckowski" exe="/usr/sbin/sshd" hostname=127.0.0.1 addr=127.0.0.1 terminal=ssh res=success'UID="root" AUID="jnyckowski"
// type=USER_END msg=audit(1657744078.476:5916): pid=275303 uid=0 auid=1000 ses=118 subj==unconfined msg='op=PAM:session_close grantors=pam_selinux,pam_loginuid,pam_keyinit,pam_permit,pam_umask,pam_unix,pam_systemd,pam_mail,pam_limits,pam_env,pam_env,pam_selinux,pam_tty_audit acct="jnyckowski" exe="/usr/sbin/sshd" hostname=127.0.0.1 addr=127.0.0.1 terminal=ssh res=success'UID="root" AUID="jnyckowski"

type AuditDClient struct {
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
	msg.SetDefaults()

	auditd, err := NewAuditDClient(msg)
	if err != nil {
		return trace.Wrap(err)
	}
	defer auditd.Close()

	if err := auditd.SendMsg(event, result); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *AuditDClient) connect() error {
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
		//if errors.Is(err, syscall.EPERM) {
		//	return nil, trace.ConvertSystemError()
		//}
		return trace.Errorf("failed to get audutd state: %v", trace.ConvertSystemError(err))
	}

	c.enabled = status.Enabled == 1

	if status.Enabled != 1 {
		return ErrAuditdDisabled
	}
	log.Warnf("auditd is enabled")

	return nil
}

func NewAuditDClient(msg Message) (*AuditDClient, error) { //TODO consider removing error
	msg.SetDefaults()

	execName, err := os.Executable()
	if err != nil {
		log.WithError(err).Warn("failed to get executable name")
		execName = "?"
	}

	// Match sshd
	const hostname = "?"

	return &AuditDClient{
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
	}, nil
}

func (c *AuditDClient) getAuditStatus() (*auditStatus, error) {
	resp, err := c.conn.Execute(netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(AUDIT_GET),
			Flags: netlink.Request | netlink.Acknowledge,
		},
		Data: nil,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Warnf("AuditGetResp: %v\n", resp)

	msgs, err := c.conn.Receive()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Warnf("msgs: %v\n", msgs)

	if len(msgs) != 1 {
		return nil, trace.Errorf("returned wrong messages number, expected 1, got: %d", len(msgs))
	}

	byteOrder := nlenc.NativeEndian()
	status := &auditStatus{}

	payload := bytes.NewReader(msgs[0].Data[:])
	if err := binary.Read(payload, byteOrder, status); err != nil {
		return nil, trace.Wrap(err)
	}

	log.Warnf("status: %+v\n", status)

	return status, nil
}

func (c *AuditDClient) SendMsg(event EventType, result ResultType) error {
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

func (c *AuditDClient) sendMsg(eventType netlink.HeaderType, MsgData []byte) error {
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
	log.Infof("reply: %v", resp)

	return nil
}

func (c *AuditDClient) Close() error {
	return c.conn.Close()
}
