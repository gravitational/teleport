//go:build bpf && !386
// +build bpf,!386

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

package restrictedsession

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"unsafe"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/bpf"
	api "github.com/gravitational/teleport/lib/events"
)

const (
	BlockedIP4 = iota
	BlockedIP6
)

//
// Audit Event types communicated between the kernel and userspace
//

// auditEventHeader matches audit_event_header in the C file
type auditEventHeader struct {
	CGroupID  uint64
	PID       uint32
	EventType int32
	Command   [bpf.CommMax]byte
}

// auditEventBlockedIPv4 matches audit_event_blocked_ipv4 in the C file
type auditEventBlockedIPv4 struct {
	SrcIP   [4]byte
	DstIP   [4]byte
	DstPort uint16
	Op      uint8
}

// auditEventBlockedIPv6 matches audit_event_blocked_ipv6 in the C file
type auditEventBlockedIPv6 struct {
	SrcIP   [16]byte
	DstIP   [16]byte
	DstPort uint16
	Op      uint8
}

// newNetworkAuditEvent creates events.SessionNetwork, filling in common fields
// from the SessionContext
func newNetworkAuditEvent(ctx *bpf.SessionContext, hdr *auditEventHeader) events.SessionNetwork {
	return events.SessionNetwork{
		Metadata: events.Metadata{
			Type: api.SessionNetworkEvent,
			Code: api.SessionNetworkCode,
		},
		ServerMetadata: events.ServerMetadata{
			ServerID:        ctx.ServerID,
			ServerHostname:  ctx.ServerHostname,
			ServerNamespace: ctx.Namespace,
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: ctx.SessionID,
		},
		UserMetadata: events.UserMetadata{
			User:  ctx.User,
			Login: ctx.Login,
		},
		BPFMetadata: events.BPFMetadata{
			CgroupID: hdr.CGroupID,
			Program:  bpf.ConvertString(unsafe.Pointer(&hdr.Command)),
			PID:      uint64(hdr.PID),
		},
	}
}

// parseAuditEventHeader parse the header portion of the event.
// buf is consumed so that only body bytes remain.
func parseAuditEventHeader(buf *bytes.Buffer) (auditEventHeader, error) {
	var hdr auditEventHeader
	err := binary.Read(buf, binary.LittleEndian, &hdr)
	if err != nil {
		return auditEventHeader{}, trace.BadParameter("corrupt event header: %v", err)
	}
	return hdr, nil
}

// ip6String is similar to IP.String but retains mapped addresses
// in IPv6 form.
func ip6String(ip net.IP) string {
	var prefix string

	if ip.To4() != nil {
		// IP4 mapped address
		prefix = "::ffff:"
	}

	return prefix + ip.String()
}

// parseAuditEvent parses the body of the audit event
func parseAuditEvent(buf *bytes.Buffer, hdr *auditEventHeader, ctx *bpf.SessionContext) (events.AuditEvent, error) {
	switch hdr.EventType {
	case BlockedIP4:
		var body auditEventBlockedIPv4
		if err := binary.Read(buf, binary.LittleEndian, &body); err != nil {
			return nil, trace.Wrap(err)
		}

		event := newNetworkAuditEvent(ctx, hdr)
		event.DstPort = int32(body.DstPort)
		event.DstAddr = net.IP(body.DstIP[:]).String()
		event.SrcAddr = net.IP(body.SrcIP[:]).String()
		event.TCPVersion = 4
		event.Operation = events.SessionNetwork_NetworkOperation(body.Op)
		event.Action = events.EventAction_DENIED

		return &event, nil

	case BlockedIP6:
		var body auditEventBlockedIPv6
		if err := binary.Read(buf, binary.LittleEndian, &body); err != nil {
			return nil, trace.Wrap(err)
		}

		event := newNetworkAuditEvent(ctx, hdr)
		event.DstPort = int32(body.DstPort)
		event.DstAddr = ip6String(net.IP(body.DstIP[:]))
		event.SrcAddr = ip6String(net.IP(body.SrcIP[:]))
		event.TCPVersion = 6
		event.Operation = events.SessionNetwork_NetworkOperation(body.Op)
		event.Action = events.EventAction_DENIED

		return &event, nil
	}

	return nil, fmt.Errorf("received unknown event type: %v", hdr.EventType)
}
