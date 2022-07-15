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

package srv

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/user"
	"syscall"
	"unsafe"

	"github.com/gravitational/trace"
	"github.com/mozilla/libaudit-go"
	log "github.com/sirupsen/logrus"
)

const (
	success = "success"
	failed  = "failed"
)

// hostEndian is initialized to the byte order of the system
var hostEndian binary.ByteOrder

func init() {
	hostEndian = nativeEndian()
}

// nativeEndian determines the byte order for the system
func nativeEndian() binary.ByteOrder {
	var x uint32 = 0x01020304
	if *(*byte)(unsafe.Pointer(&x)) == 0x01 {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

// sshd
// type=USER_LOGIN msg=audit(1657493525.955:466): pid=16059 uid=0 auid=4294967295 ses=4294967295 subj==unconfined msg='op=login acct="jnyckowski" exe="/usr/sbin/sshd" hostname=? addr=127.0.0.1 terminal=sshd res=failed'UID="root" AUID="unset"
// type=USER_START msg=audit(1657493584.668:474): pid=16059 uid=0 auid=1000 ses=11 subj==unconfined msg='op=PAM:session_open grantors=pam_selinux,pam_loginuid,pam_keyinit,pam_permit,pam_umask,pam_unix,pam_systemd,pam_mail,pam_limits,pam_env,pam_env,pam_selinux,pam_tty_audit acct="jnyckowski" exe="/usr/sbin/sshd" hostname=127.0.0.1 addr=127.0.0.1 terminal=ssh res=success'UID="root" AUID="jnyckowski"
// type=USER_END msg=audit(1657744078.476:5916): pid=275303 uid=0 auid=1000 ses=118 subj==unconfined msg='op=PAM:session_close grantors=pam_selinux,pam_loginuid,pam_keyinit,pam_permit,pam_umask,pam_unix,pam_systemd,pam_mail,pam_limits,pam_env,pam_env,pam_selinux,pam_tty_audit acct="jnyckowski" exe="/usr/sbin/sshd" hostname=127.0.0.1 addr=127.0.0.1 terminal=ssh res=success'UID="root" AUID="jnyckowski"

type AuditDClient struct {
	*libaudit.NetlinkConnection
	seqNum uint32

	pid      int
	execName string
	hostname string
	user     string
	address  string
	ttyName  string
}

func NewAuditDClient(ttyName string) (*AuditDClient, error) {
	s, err := libaudit.NewNetlinkConnection()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	enabled, err := libaudit.AuditIsEnabled(s)
	if err != nil {
		return nil, trace.Errorf("failed to get audutd state: %v", err)
	}

	if !enabled {
		return nil, trace.Errorf("audutd is disabled")
	}
	log.Warnf("auditd is enabled")

	pid := os.Getpid()
	execName, err := os.Executable()
	if err != nil {
		log.WithError(err).Warn("failed to get executable name")
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.WithError(err).Warn("failed to get hostname")
	}
	currentUser, err := user.Current()
	if err != nil {
		log.WithError(err).Warn("failed to get the current user")
	}

	addr := "127.0.0.1"

	return &AuditDClient{
		NetlinkConnection: s,
		seqNum:            2, //todo explain
		pid:               pid,
		execName:          execName,
		hostname:          hostname,
		user:              currentUser.Username, //TODO: fix me
		address:           addr,
		ttyName:           ttyName,
	}, nil
}

func (c *AuditDClient) SendLogin() error {
	log.Warnf("sending login audit event")
	//libaudit.NewAuditEvent()
	//libaudit.GetAuditEvents()

	//libaudit.AUDIT_USER_LOGIN

	const msgDataTmpl = "op=%s acct=\"%s\" exe=%s hostname=%s addr=%s terminal=%s res=%s"
	const op = "login"

	MsgData := []byte(fmt.Sprintf(msgDataTmpl, op, c.user, c.execName, c.hostname, c.address, c.ttyName, success))

	return c.sendMsg(uint16(libaudit.AUDIT_USER_LOGIN), MsgData)
}

// type=USER_END msg=audit(1657744078.476:5916): pid=275303 uid=0 auid=1000 ses=118 subj==unconfined msg='op=PAM:session_close grantors=pam_selinux,pam_loginuid,pam_keyinit,pam_permit,pam_umask,pam_unix,pam_systemd,pam_mail,pam_limits,pam_env,pam_env,pam_selinux,pam_tty_audit acct="jnyckowski" exe="/usr/sbin/sshd" hostname=127.0.0.1 addr=127.0.0.1 terminal=ssh res=success'UID="root" AUID="jnyckowski"

func (c *AuditDClient) SendSessionEnd() error {
	log.Warnf("sending login audit event")

	//const msgDataTmpl = "op=PAM:session_close grantors=pam_selinux,pam_loginuid,pam_keyinit,pam_permit,pam_umask,pam_unix,pam_systemd,pam_mail,pam_limits,pam_env,pam_env,pam_selinux,pam_tty_audit acct=\"jnyckowski\" exe=\"/usr/sbin/sshd\" hostname=127.0.0.1 addr=127.0.0.1 terminal=ssh res=success'UID=\"root\" AUID=\"jnyckowski\""
	const msgDataTmpl = "op=%s acct=\"%s\" exe=%s hostname=%s addr=%s terminal=%s res=%s"
	const op = "session_close"

	MsgData := []byte(fmt.Sprintf(msgDataTmpl, op, c.user, c.execName, c.hostname, c.address, c.ttyName, success))

	return c.sendMsg(uint16(libaudit.AUDIT_USER_END), MsgData)
}

func (c *AuditDClient) sendMsg(eventType uint16, MsgData []byte) error {
	sizeofData := len(MsgData)

	msg := &libaudit.NetlinkMessage{
		Header: syscall.NlMsghdr{
			Len:   uint32(syscall.NLMSG_HDRLEN + sizeofData),
			Type:  eventType,
			Flags: syscall.NLM_F_REQUEST | syscall.NLM_F_ACK,
			Seq:   c.seqNum,
			Pid:   uint32(c.pid),
		},
		Data: MsgData,
	}
	c.seqNum++ // TODO: make thread safe

	if err := c.Send(msg); err != nil {
		return trace.Wrap(err)
	}

	msgs, err := auditGetReply(c.NetlinkConnection, msg.Header.Seq, true)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(msgs) != 0 {
		return fmt.Errorf("unexpected number of responses from kernel for status request: %d", len(msgs))
	}
	log.Infof("reply: %v", msgs)

	return nil
}

func auditGetReply(s libaudit.Netlink, seq uint32, chkAck bool) (ret []libaudit.NetlinkMessage, err error) {
done:
	for {
		dbrk := false
		msgs, err := s.Receive(false)
		if err != nil {
			return ret, err
		}
		log.Warnf("read messages: %+v", msgs)
		for _, m := range msgs {
			socketPID, err := s.GetPID()
			if err != nil {
				return ret, err
			}
			if m.Header.Seq != seq {
				log.Warnf("seq num is diffrent")
				// Wasn't the sequence number we are looking for, just discard it
				continue
			}
			if int(m.Header.Pid) != socketPID {
				log.Warnf("socked pid is different; m.Header.Pid: %v, socketPID: %v", m.Header.Pid, socketPID)
				// PID didn't match, just discard it
				continue
			}
			if m.Header.Type == syscall.NLMSG_DONE {
				break done
			}
			if m.Header.Type == syscall.NLMSG_ERROR {
				log.Warnf("msg type error")
				e := int32(hostEndian.Uint32(m.Data[0:4]))
				if e == 0 {
					log.Warnf("error code == 0")
					// ACK response from the kernel; if chkAck is true
					// we just return as there is nothing left to do
					if chkAck {
						break done
					}
					// Otherwise, keep going, so we can get the response
					// we want
					continue
				} else {
					return ret, trace.Errorf("error while receiving reply %v", e)
				}
			}
			ret = append(ret, m)
			if (m.Header.Flags & syscall.NLM_F_MULTI) == 0 {
				// If it's not a multipart message, once we get one valid
				// message just return
				dbrk = true
				break
			}
			log.Warnf("waiting for more parts")
		}
		if dbrk {
			break
		}
	}
	return ret, nil
}
