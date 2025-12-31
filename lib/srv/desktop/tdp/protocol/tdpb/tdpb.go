package tdpb

import (
	"encoding/binary"
	"io"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
)

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

// ClientHello message.
type ClientHello tdpbv1.ClientHello

// Encode encodes a ClientHello message.
func (c *ClientHello) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_ClientHello{
			ClientHello: (*tdpbv1.ClientHello)(c),
		},
	})
}

// ServerHello message.
type ServerHello tdpbv1.ServerHello

// Encode encodes a ServerHello message.
func (S *ServerHello) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_ServerHello{
			ServerHello: (*tdpbv1.ServerHello)(S),
		},
	})
}

// PNGFrame message.
type PNGFrame tdpbv1.PNGFrame

// Encode encodes a PNGFrame message.
func (p *PNGFrame) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_PngFrame{
			PngFrame: (*tdpbv1.PNGFrame)(p),
		},
	})
}

// FastPathPDU message.
type FastPathPDU tdpbv1.FastPathPDU

// Encode encodes a FastPathPDU message.
func (f *FastPathPDU) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_FastPathPdu{
			FastPathPdu: (*tdpbv1.FastPathPDU)(f),
		},
	})
}

// RDPResponsePDU message.
type RDPResponsePDU tdpbv1.RDPResponsePDU

// Encode encodes a RDPResponsePDU message.
func (f *RDPResponsePDU) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_RdpResponsePdu{
			RdpResponsePdu: (*tdpbv1.RDPResponsePDU)(f),
		},
	})
}

// SyncKeys message.
type SyncKeys tdpbv1.SyncKeys

// Encode encodes a SyncKeys message.
func (s *SyncKeys) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_SyncKeys{
			SyncKeys: (*tdpbv1.SyncKeys)(s),
		},
	})
}

// MouseMove message.
type MouseMove tdpbv1.MouseMove

// Encode encodes a MouseMove message.
func (m *MouseMove) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_MouseMove{
			MouseMove: (*tdpbv1.MouseMove)(m),
		},
	})
}

// MouseButton message.
type MouseButton tdpbv1.MouseButton

// Encode encodes a MouseButton message.
func (m *MouseButton) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_MouseButton{
			MouseButton: (*tdpbv1.MouseButton)(m),
		},
	})
}

// KeyboardButton message.
type KeyboardButton tdpbv1.KeyboardButton

// Encode encodes a KeyboardButton message.
func (k *KeyboardButton) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_KeyboardButton{
			KeyboardButton: (*tdpbv1.KeyboardButton)(k),
		},
	})
}

// ClientScreenSpec message.
type ClientScreenSpec tdpbv1.ClientScreenSpec

// Encode encodes a ClientScreenSpec message.
func (c *ClientScreenSpec) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_ClientScreenSpec{
			ClientScreenSpec: (*tdpbv1.ClientScreenSpec)(c),
		},
	})
}

// Alert message.
type Alert tdpbv1.Alert

// Encode encodes a Alert message.
func (a *Alert) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_Alert{
			Alert: (*tdpbv1.Alert)(a),
		},
	})
}

// MouseWheel message.
type MouseWheel tdpbv1.MouseWheel

// Encode encodes a MouseWheel message.
func (m *MouseWheel) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_MouseWheel{
			MouseWheel: (*tdpbv1.MouseWheel)(m),
		},
	})
}

// ClipboardData message.
type ClipboardData tdpbv1.ClipboardData

// Encode encodes a ClipboardData message.
func (c *ClipboardData) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_ClipboardData{
			ClipboardData: (*tdpbv1.ClipboardData)(c),
		},
	})
}

// MFA message.
type MFA tdpbv1.MFA

// Encode encodes a MFA message.
func (m *MFA) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_Mfa{
			Mfa: (*tdpbv1.MFA)(m),
		},
	})
}

// SharedDirectoryAnnounce message.
type SharedDirectoryAnnounce tdpbv1.SharedDirectoryAnnounce

// Encode encodes a SharedDirectoryAnnounce message.
func (s *SharedDirectoryAnnounce) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryAnnounce{
			SharedDirectoryAnnounce: (*tdpbv1.SharedDirectoryAnnounce)(s),
		},
	})
}

// SharedDirectoryAcknowledge message.
type SharedDirectoryAcknowledge tdpbv1.SharedDirectoryAcknowledge

// Encode encodes a SharedDirectoryAcknowledge message.
func (s *SharedDirectoryAcknowledge) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryAcknowledge{
			SharedDirectoryAcknowledge: (*tdpbv1.SharedDirectoryAcknowledge)(s),
		},
	})
}

// SharedDirectoryRequest message.
type SharedDirectoryRequest tdpbv1.SharedDirectoryRequest

// Encode encodes a SharedDirectoryRequest message.
func (s *SharedDirectoryRequest) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryRequest{
			SharedDirectoryRequest: (*tdpbv1.SharedDirectoryRequest)(s),
		},
	})
}

// SharedDirectoryResponse message.
type SharedDirectoryResponse tdpbv1.SharedDirectoryResponse

// Encode encodes a SharedDirectoryResponse message.
func (s *SharedDirectoryResponse) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryResponse{
			SharedDirectoryResponse: (*tdpbv1.SharedDirectoryResponse)(s),
		},
	})
}

// LatencyStats message.
type LatencyStats tdpbv1.LatencyStats

// Encode encodes a LatencyStats message.
func (l *LatencyStats) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_LatencyStats{
			LatencyStats: (*tdpbv1.LatencyStats)(l),
		},
	})
}

// Ping message.
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
		return nil, trace.Errorf("TDPB message too large. %d bytes exceeds maximum: %d", len(data), maxMessageLength)
	}

	header := make([]byte, len(data)+tdpbHeaderLength)
	binary.BigEndian.PutUint32(header[:tdpbHeaderLength], uint32(len(data)))
	copy(header[tdpbHeaderLength:], data)

	return header, nil
}

// Decode reads a TDPB message from a reader.
// Returns ErrEmptyMessage if a valid TDPB Envelope was received, but no
// wrapped message was found.
func Decode(rdr io.Reader) (protocol.Message, error) {
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
	return nil, trace.Wrap(protocol.ErrEmptyMessage)
}

func messageFromEnvelope(e *tdpbv1.Envelope) protocol.Message {
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

// EncodeTo calls 'Encode' on the given message and writes it to 'w'.
func EncodeTo(w io.Writer, msg protocol.Message) error {
	data, err := msg.Encode()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}
