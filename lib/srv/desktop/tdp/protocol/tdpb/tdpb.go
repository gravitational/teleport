/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

// package tdpb implements Teleport Desktop Protocol via protobuf,
// a replacement for the original hand-written protocol.
package tdpb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

// ProtocolName is the identifier for the TDPB protocol.
const ProtocolName = "teleport-tdpb-1.0"

// ErrUnknownMessage is returned when an unknown message is decoded.
var ErrUnknownMessage = errors.New("decoded unknown TDPB message")

// ErrIsTDP is returned when a legacy TDP message is received
// during or after a connection upgrade to TDPB.
var ErrIsTDP = errors.New("message is TDP, not TDPB")

const (
	// We can differentiate between TDP and TDPB messages on the wire
	// by inspecting the first byte received. A non-empty first byte
	// is presumed to be a TDP message, otherwise, TDPB.
	// Since the first byte of a TDPB message is the high 8 bits of its
	// length, we must take care not to allow TDPB messages that
	// meet or exceed length 2^24 (16MiB).
	// Once TDP is fully deprecated we can relax this constraint, although
	// it's unlikely we would ever want messages anywhere near this size.
	maxMessageLength = (1 << 24) - 1
	tdpbHeaderLength = 4 // sizeof(uint32)
)

// ClientHello is the first message sent by the client, and advertises
// client capabilities and connection properties.
type ClientHello tdpbv1.ClientHello

// Encode encodes a ClientHello message.
func (c *ClientHello) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_ClientHello{
			ClientHello: (*tdpbv1.ClientHello)(c),
		},
	})
}

// ServerHello is the first message sent by the server *after* receiving
// the ClientHello. It selects and advertises server capabilities and
// connection properties.
type ServerHello tdpbv1.ServerHello

// Encode encodes a ServerHello message.
func (s *ServerHello) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_ServerHello{
			ServerHello: (*tdpbv1.ServerHello)(s),
		},
	})
}

// PNGFrame carries screen data in PNG format. It is required
// for interop with older session recordings that came before
// desktop access adopted the RemoteFX codec.
type PNGFrame tdpbv1.PNGFrame

// Encode encodes a PNGFrame message.
func (p *PNGFrame) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_PngFrame{
			PngFrame: (*tdpbv1.PNGFrame)(p),
		},
	})
}

// FastPathPDU is a raw RDP Fast-Path Protocol Data Unit (PDU).
type FastPathPDU tdpbv1.FastPathPDU

// Encode encodes a FastPathPDU message.
func (f *FastPathPDU) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_FastPathPdu{
			FastPathPdu: (*tdpbv1.FastPathPDU)(f),
		},
	})
}

// RDPResponsePDU is a raw RDP response PDU.
type RDPResponsePDU tdpbv1.RDPResponsePDU

// Encode encodes a RDPResponsePDU message.
func (f *RDPResponsePDU) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_RdpResponsePdu{
			RdpResponsePdu: (*tdpbv1.RDPResponsePDU)(f),
		},
	})
}

// SyncKeys message is sent from the client to the server to
// synchronize the state of keyboard's modifier keys.
type SyncKeys tdpbv1.SyncKeys

// Encode encodes a SyncKeys message.
func (s *SyncKeys) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_SyncKeys{
			SyncKeys: (*tdpbv1.SyncKeys)(s),
		},
	})
}

// MouseMove contains mouse coordinates.
type MouseMove tdpbv1.MouseMove

// Encode encodes a MouseMove message.
func (m *MouseMove) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_MouseMove{
			MouseMove: (*tdpbv1.MouseMove)(m),
		},
	})
}

// MouseButton contains mouse button state.
type MouseButton tdpbv1.MouseButton

// Encode encodes a MouseButton message.
func (m *MouseButton) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_MouseButton{
			MouseButton: (*tdpbv1.MouseButton)(m),
		},
	})
}

// KeyboardButton encodes a keyboard button update.
type KeyboardButton tdpbv1.KeyboardButton

// Encode encodes a KeyboardButton message.
func (k *KeyboardButton) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_KeyboardButton{
			KeyboardButton: (*tdpbv1.KeyboardButton)(k),
		},
	})
}

// ClientScreenSpec contains the dimensions of the client view.
// It is included in the ClientHello at the start of the session, and
// is also sent when the client resizes its window.
type ClientScreenSpec tdpbv1.ClientScreenSpec

// Encode encodes a ClientScreenSpec message.
func (c *ClientScreenSpec) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_ClientScreenSpec{
			ClientScreenSpec: (*tdpbv1.ClientScreenSpec)(c),
		},
	})
}

// Alert encodes an error/warning/informational message and severity code.
// Sent by the server to the client for display.
type Alert tdpbv1.Alert

// Encode encodes a Alert message.
func (a *Alert) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_Alert{
			Alert: (*tdpbv1.Alert)(a),
		},
	})
}

// MouseWheel contains a mousewheel update.
type MouseWheel tdpbv1.MouseWheel

// Encode encodes a MouseWheel message.
func (m *MouseWheel) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_MouseWheel{
			MouseWheel: (*tdpbv1.MouseWheel)(m),
		},
	})
}

// ClipboardData carries clipboard data to support copy/paste
// operations between the client and target desktop.
type ClipboardData tdpbv1.ClipboardData

// Encode encodes a ClipboardData message.
func (c *ClipboardData) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_ClipboardData{
			ClipboardData: (*tdpbv1.ClipboardData)(c),
		},
	})
}

// MFA encodes the MFA challenge and response when per-session
// MFA is enabled.
type MFA tdpbv1.MFA

// Encode encodes a MFA message.
func (m *MFA) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_Mfa{
			Mfa: (*tdpbv1.MFA)(m),
		},
	})
}

// SharedDirectoryAnnounce is sent by the client to begin sharing a directory.
type SharedDirectoryAnnounce tdpbv1.SharedDirectoryAnnounce

// Encode encodes a SharedDirectoryAnnounce message.
func (s *SharedDirectoryAnnounce) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryAnnounce{
			SharedDirectoryAnnounce: (*tdpbv1.SharedDirectoryAnnounce)(s),
		},
	})
}

// SharedDirectoryAcknowledge is sent by the server to acknowledge a
// new shared directory.
type SharedDirectoryAcknowledge tdpbv1.SharedDirectoryAcknowledge

// Encode encodes a SharedDirectoryAcknowledge message.
func (s *SharedDirectoryAcknowledge) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryAcknowledge{
			SharedDirectoryAcknowledge: (*tdpbv1.SharedDirectoryAcknowledge)(s),
		},
	})
}

// SharedDirectoryRequest encodes various directory operation requests
// such as Info, Create, Delete, List, Read, Write, Move, or Truncate.
type SharedDirectoryRequest tdpbv1.SharedDirectoryRequest

// Encode encodes a SharedDirectoryRequest message.
func (s *SharedDirectoryRequest) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryRequest{
			SharedDirectoryRequest: (*tdpbv1.SharedDirectoryRequest)(s),
		},
	})
}

// SharedDirectoryResponse encodes a response to a previous SharedDirectoryRequest.
type SharedDirectoryResponse tdpbv1.SharedDirectoryResponse

// Encode encodes a SharedDirectoryResponse message.
func (s *SharedDirectoryResponse) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryResponse{
			SharedDirectoryResponse: (*tdpbv1.SharedDirectoryResponse)(s),
		},
	})
}

// LatencyStats are sent to the client to display connection
// latency between both the user and Teleport, as well as
// between Teleport and the target desktop.
type LatencyStats tdpbv1.LatencyStats

// Encode encodes a LatencyStats message.
func (l *LatencyStats) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_LatencyStats{
			LatencyStats: (*tdpbv1.LatencyStats)(l),
		},
	})
}

// Ping is used to measure latency between the Proxy and
// target desktop.
type Ping tdpbv1.Ping

// Encodes a ping message.
func (p *Ping) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_Ping{
			Ping: (*tdpbv1.Ping)(p),
		},
	})
}

func marshalWithHeader(msg proto.Message) ([]byte, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(data) > maxMessageLength {
		// Message too large, or did we somehow receive a legacy TDP message by mistake?
		return nil, trace.Errorf("TDPB message too large. %d bytes exceeds maximum: %d", len(data), maxMessageLength)
	}

	header := make([]byte, len(data)+tdpbHeaderLength)
	binary.BigEndian.PutUint32(header[:tdpbHeaderLength], uint32(len(data)))
	copy(header[tdpbHeaderLength:], data)

	return header, nil
}

// DecodePermissive quietly tolerates unknown message types to allow interop
// with newer TDPB implementations
func DecodePermissive(rdr io.Reader) (tdp.Message, error) {
	for {
		msg, err := DecodeStrict(rdr)
		if err != nil {
			if errors.Is(err, ErrUnknownMessage) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		return msg, nil
	}
}

// DecodeWithTDPDiscard wraps 'DecodePermissive' and also detects and quietly ignores
// legacy TDP messages that may appear on the wire. Intended for use during TDP Upgrade as
// the TDP client *may* send a few legacy messages before receiving the Upgrade request.
// Assumes you have the full message available.
func DecodeWithTDPDiscard(data []byte) (tdp.Message, error) {
	switch {
	case len(data) < 1:
		return nil, trace.BadParameter("message is empty")
	case data[0] != 0:
		// "Legacy" TDP messages begin with non-zero first byte
		// discard any legacy TDP messages received
		return nil, ErrIsTDP
	default:
		msg, err := DecodePermissive(bytes.NewReader(data))
		return msg, trace.Wrap(err)
	}
}

// DecodeStrict reads a TDPB message from a reader.
// Returns ErrUnknownMessage if a valid TDPB Envelope was received, but no
// wrapped message was found (likely because it came from a newer implementation).
func DecodeStrict(rdr io.Reader) (tdp.Message, error) {
	// Read header
	header := make([]byte, tdpbHeaderLength)
	_, err := io.ReadFull(rdr, header)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "error reading next TDPB message header")
	}

	messageLength := binary.BigEndian.Uint32(header)

	if messageLength >= maxMessageLength {
		return nil, trace.Errorf("message of length '%d' exceeds maximum allowed length '%d'", messageLength, maxMessageLength)
	}

	message := make([]byte, messageLength)
	_, err = io.ReadFull(rdr, message)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "error reading TDPB message body")
	}

	env := &tdpbv1.Envelope{}
	if err = proto.Unmarshal(message, env); err != nil {
		return nil, trace.WrapWithMessage(err, "error unmarshalling TDPB message envelope")
	}

	if msg := messageFromEnvelope(env); msg != nil {
		return msg, nil
	}

	// Allow the caller to distinguish unmarshal errors (likely considered fatal)
	// from an "empty" message, which could simply mean that we've received
	// a new (unsupported) message from a newer implementation.
	return nil, trace.Wrap(ErrUnknownMessage)
}

func messageFromEnvelope(e *tdpbv1.Envelope) tdp.Message {
	switch m := e.Payload.(type) {
	case *tdpbv1.Envelope_ClientHello:
		return (*ClientHello)(m.ClientHello)
	case *tdpbv1.Envelope_ServerHello:
		return (*ServerHello)(m.ServerHello)
	case *tdpbv1.Envelope_PngFrame:
		return (*PNGFrame)(m.PngFrame)
	case *tdpbv1.Envelope_FastPathPdu:
		return (*FastPathPDU)(m.FastPathPdu)
	case *tdpbv1.Envelope_RdpResponsePdu:
		return (*RDPResponsePDU)(m.RdpResponsePdu)
	case *tdpbv1.Envelope_SyncKeys:
		return (*SyncKeys)(m.SyncKeys)
	case *tdpbv1.Envelope_MouseMove:
		return (*MouseMove)(m.MouseMove)
	case *tdpbv1.Envelope_MouseButton:
		return (*MouseButton)(m.MouseButton)
	case *tdpbv1.Envelope_KeyboardButton:
		return (*KeyboardButton)(m.KeyboardButton)
	case *tdpbv1.Envelope_ClientScreenSpec:
		return (*ClientScreenSpec)(m.ClientScreenSpec)
	case *tdpbv1.Envelope_Alert:
		return (*Alert)(m.Alert)
	case *tdpbv1.Envelope_MouseWheel:
		return (*MouseWheel)(m.MouseWheel)
	case *tdpbv1.Envelope_ClipboardData:
		return (*ClipboardData)(m.ClipboardData)
	case *tdpbv1.Envelope_Mfa:
		return (*MFA)(m.Mfa)
	case *tdpbv1.Envelope_SharedDirectoryAnnounce:
		return (*SharedDirectoryAnnounce)(m.SharedDirectoryAnnounce)
	case *tdpbv1.Envelope_SharedDirectoryAcknowledge:
		return (*SharedDirectoryAcknowledge)(m.SharedDirectoryAcknowledge)
	case *tdpbv1.Envelope_SharedDirectoryRequest:
		return (*SharedDirectoryRequest)(m.SharedDirectoryRequest)
	case *tdpbv1.Envelope_SharedDirectoryResponse:
		return (*SharedDirectoryResponse)(m.SharedDirectoryResponse)
	case *tdpbv1.Envelope_LatencyStats:
		return (*LatencyStats)(m.LatencyStats)
	case *tdpbv1.Envelope_Ping:
		return (*Ping)(m.Ping)
	default:
		return nil
	}
}
