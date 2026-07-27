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
	return marshalWithHeader(tdpbv1.Envelope_builder{
		ClientHello: proto.ValueOrDefault((*tdpbv1.ClientHello)(c)),
	}.Build())
}

func (*ClientHello) validate() error { return nil }

// ServerHello is the first message sent by the server *after* receiving
// the ClientHello. It selects and advertises server capabilities and
// connection properties.
type ServerHello tdpbv1.ServerHello

// Encode encodes a ServerHello message.
func (s *ServerHello) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		ServerHello: proto.ValueOrDefault((*tdpbv1.ServerHello)(s)),
	}.Build())
}

func (*ServerHello) validate() error { return nil }

// PNGFrame carries screen data in PNG format. It is required
// for interop with older session recordings that came before
// desktop access adopted the RemoteFX codec.
type PNGFrame tdpbv1.PNGFrame

// Encode encodes a PNGFrame message.
func (p *PNGFrame) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		PngFrame: proto.ValueOrDefault((*tdpbv1.PNGFrame)(p)),
	}.Build())
}

func (*PNGFrame) validate() error { return nil }

// FastPathPDU is a raw RDP Fast-Path Protocol Data Unit (PDU).
type FastPathPDU tdpbv1.FastPathPDU

// Encode encodes a FastPathPDU message.
func (f *FastPathPDU) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		FastPathPdu: proto.ValueOrDefault((*tdpbv1.FastPathPDU)(f)),
	}.Build())
}

func (*FastPathPDU) validate() error { return nil }

// EgfxBitmap is a pre-decoded EGFX (RDPGFX) bitmap update — desktop-coordinate
// RGBA produced by the server-side IronRDP EGFX client and forwarded to the
// browser for direct blitting into the framebuffer image.
type EgfxBitmap tdpbv1.EgfxBitmap

// Encode encodes an EgfxBitmap message.
func (b *EgfxBitmap) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxBitmap{
			EgfxBitmap: (*tdpbv1.EgfxBitmap)(b),
		},
	})
}

func (*EgfxBitmap) validate() error { return nil }

// EgfxAvcFrame is an EGFX AVC444/v2 frame: the server unpacks the
// RFX_AVC444V2_BITMAP_STREAM wrapper and forwards the inner H.264 streams
// to the browser, which decodes them with the WebCodecs VideoDecoder API
// and composes YUV444 → RGBA client-side.
type EgfxAvcFrame tdpbv1.EgfxAvcFrame

// Encode encodes an EgfxAvcFrame message.
func (f *EgfxAvcFrame) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxAvcFrame{
			EgfxAvcFrame: (*tdpbv1.EgfxAvcFrame)(f),
		},
	})
}

func (*EgfxAvcFrame) validate() error { return nil }

// EgfxClearCodec is a raw ClearCodec ([MS-RDPEGFX] 2.2.4.2) PDU forwarded
// from the server to the wasm client. The server only parses the surrounding
// WireToSurface1Pdu wrapper and ships the inner ClearCodec bytes verbatim;
// the wasm decoder writes in-place into the framebuffer so PDUs that paint
// only a sub-region of the destination rectangle preserve existing pixels
// elsewhere in the rect (vs. EgfxBitmap which always overwrites a full
// rect-sized RGBA buffer).
type EgfxClearCodec tdpbv1.EgfxClearCodec

// Encode encodes an EgfxClearCodec message.
func (c *EgfxClearCodec) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxClearCodec{
			EgfxClearCodec: (*tdpbv1.EgfxClearCodec)(c),
		},
	})
}

func (*EgfxClearCodec) validate() error { return nil }

// EgfxUncompressed is a raw `Codec1Type::Uncompressed` ([MS-RDPEGFX] 2.2.4.2)
// PDU forwarded from server to wasm. Windows uses this codec heavily for
// small UI overlays (tooltips, popup chrome, hover shadows) where the
// per-frame setup cost of a compressed codec outweighs the savings, often
// with an alpha channel (PIXEL_FORMAT_ARGB_8888). The wasm side reorders
// channels and source-over composites against the existing framebuffer.
type EgfxUncompressed tdpbv1.EgfxUncompressed

func (m *EgfxUncompressed) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxUncompressed{
			EgfxUncompressed: (*tdpbv1.EgfxUncompressed)(m),
		},
	})
}

func (*EgfxUncompressed) validate() error { return nil }

// EgfxPlanar is a raw `Codec1Type::Planar` ([MS-RDPEGDI] 2.2.9.1.0.2 RDP 6.0
// bitmap stream) PDU forwarded from server to wasm. Decoded via
// ironrdp_graphics::rdp6::bitmap_stream on the client.
type EgfxPlanar tdpbv1.EgfxPlanar

func (m *EgfxPlanar) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxPlanar{
			EgfxPlanar: (*tdpbv1.EgfxPlanar)(m),
		},
	})
}

func (*EgfxPlanar) validate() error { return nil }

// EgfxAvc420 is a raw `Codec1Type::Avc420` ([MS-RDPEGFX] 2.2.4.3) PDU
// forwarded from server to wasm. The `Avc420EncapsulatedBitmapStream`
// envelope + H.264 NAL units are decoded entirely on the wasm side.
type EgfxAvc420 tdpbv1.EgfxAvc420

func (m *EgfxAvc420) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxAvc420{
			EgfxAvc420: (*tdpbv1.EgfxAvc420)(m),
		},
	})
}

func (*EgfxAvc420) validate() error { return nil }

// EgfxSolidFill is a solid-color fill of one or more rectangles on a
// surface ([MS-RDPEGFX] 2.2.2.4). Forwarded from IronRDP's EGFX handler to
// the wasm client, which applies the fill directly to its framebuffer.
type EgfxSolidFill tdpbv1.EgfxSolidFill

func (m *EgfxSolidFill) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxSolidFill{
			EgfxSolidFill: (*tdpbv1.EgfxSolidFill)(m),
		},
	})
}

func (*EgfxSolidFill) validate() error { return nil }

// EgfxSurfaceToCache snapshots a region of the surface into the bitmap
// cache at the given slot ([MS-RDPEGFX] 2.2.2.6).
type EgfxSurfaceToCache tdpbv1.EgfxSurfaceToCache

func (m *EgfxSurfaceToCache) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxSurfaceToCache{
			EgfxSurfaceToCache: (*tdpbv1.EgfxSurfaceToCache)(m),
		},
	})
}

func (*EgfxSurfaceToCache) validate() error { return nil }

// EgfxCacheToSurface blits a cached region onto the surface at each
// destination point ([MS-RDPEGFX] 2.2.2.7).
type EgfxCacheToSurface tdpbv1.EgfxCacheToSurface

func (m *EgfxCacheToSurface) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxCacheToSurface{
			EgfxCacheToSurface: (*tdpbv1.EgfxCacheToSurface)(m),
		},
	})
}

func (*EgfxCacheToSurface) validate() error { return nil }

// EgfxEvictCacheEntry drops a bitmap cache slot ([MS-RDPEGFX] 2.2.2.8).
type EgfxEvictCacheEntry tdpbv1.EgfxEvictCacheEntry

func (m *EgfxEvictCacheEntry) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxEvictCacheEntry{
			EgfxEvictCacheEntry: (*tdpbv1.EgfxEvictCacheEntry)(m),
		},
	})
}

func (*EgfxEvictCacheEntry) validate() error { return nil }

// EgfxEndFrame marks the end of a logical EGFX frame ([MS-RDPEGFX] 2.2.2.15).
// The client presents on this boundary so only fully-composited frames reach
// the screen (presenting mid-frame causes black-rectangle flicker).
type EgfxEndFrame tdpbv1.EgfxEndFrame

func (m *EgfxEndFrame) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxEndFrame{
			EgfxEndFrame: (*tdpbv1.EgfxEndFrame)(m),
		},
	})
}

func (*EgfxEndFrame) validate() error { return nil }

// EgfxSurfaceToSurface copies a region between (or within) surfaces
// ([MS-RDPEGFX] 2.2.2.5). Used by Windows for scrolling, taskbar item
// moves, drag previews.
type EgfxSurfaceToSurface tdpbv1.EgfxSurfaceToSurface

func (m *EgfxSurfaceToSurface) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxSurfaceToSurface{
			EgfxSurfaceToSurface: (*tdpbv1.EgfxSurfaceToSurface)(m),
		},
	})
}

func (*EgfxSurfaceToSurface) validate() error { return nil }

// EgfxWireToSurface2 is a raw RFX Progressive payload ([MS-RDPEGFX] 2.2.2.2)
// forwarded from the server for wasm-side decode. Stateful per-(surface,
// codec_context_id) decoder maintains per-tile sub-band coefficients
// across PDUs until evicted by EgfxDeleteEncodingContext.
type EgfxWireToSurface2 tdpbv1.EgfxWireToSurface2

func (m *EgfxWireToSurface2) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxWireToSurface2{
			EgfxWireToSurface2: (*tdpbv1.EgfxWireToSurface2)(m),
		},
	})
}

func (*EgfxWireToSurface2) validate() error { return nil }

// EgfxDeleteEncodingContext drops the per-(surface, codec_context_id)
// progressive decoder state ([MS-RDPEGFX] 2.2.2.3).
type EgfxDeleteEncodingContext tdpbv1.EgfxDeleteEncodingContext

func (m *EgfxDeleteEncodingContext) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_EgfxDeleteEncodingContext{
			EgfxDeleteEncodingContext: (*tdpbv1.EgfxDeleteEncodingContext)(m),
		},
	})
}

func (*EgfxDeleteEncodingContext) validate() error { return nil }

// RDPResponsePDU is a raw RDP response PDU.
type RDPResponsePDU tdpbv1.RDPResponsePDU

// Encode encodes a RDPResponsePDU message.
func (f *RDPResponsePDU) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		RdpResponsePdu: proto.ValueOrDefault((*tdpbv1.RDPResponsePDU)(f)),
	}.Build())
}

func (*RDPResponsePDU) validate() error { return nil }

// RefreshRect asks the RDP server to repaint a region. The proxy
// translates it into an RDP Refresh Rect PDU on the live session.
type RefreshRect tdpbv1.RefreshRect

// Encode encodes a RefreshRect message.
func (r *RefreshRect) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_RefreshRect{
			RefreshRect: (*tdpbv1.RefreshRect)(r),
		},
	})
}

func (*RefreshRect) validate() error { return nil }

// SyncKeys message is sent from the client to the server to
// synchronize the state of keyboard's modifier keys.
type SyncKeys tdpbv1.SyncKeys

// Encode encodes a SyncKeys message.
func (s *SyncKeys) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		SyncKeys: proto.ValueOrDefault((*tdpbv1.SyncKeys)(s)),
	}.Build())
}

func (*SyncKeys) validate() error { return nil }

// SessionSelection is sent by client to select one of available sessions for Linux desktop
type SessionSelection tdpbv1.SessionSelection

func (s *SessionSelection) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		SessionSelection: proto.ValueOrDefault((*tdpbv1.SessionSelection)(s)),
	}.Build())
}

func (*SessionSelection) validate() error { return nil }

// MouseMove contains mouse coordinates.
type MouseMove tdpbv1.MouseMove

// Encode encodes a MouseMove message.
func (m *MouseMove) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		MouseMove: proto.ValueOrDefault((*tdpbv1.MouseMove)(m)),
	}.Build())
}

func (*MouseMove) validate() error { return nil }

// MouseButton contains mouse button state.
type MouseButton tdpbv1.MouseButton

// Encode encodes a MouseButton message.
func (m *MouseButton) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		MouseButton: proto.ValueOrDefault((*tdpbv1.MouseButton)(m)),
	}.Build())
}

func (*MouseButton) validate() error { return nil }

// KeyboardButton encodes a keyboard button update.
type KeyboardButton tdpbv1.KeyboardButton

// Encode encodes a KeyboardButton message.
func (k *KeyboardButton) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		KeyboardButton: proto.ValueOrDefault((*tdpbv1.KeyboardButton)(k)),
	}.Build())
}

func (*KeyboardButton) validate() error { return nil }

// ClientScreenSpec contains the dimensions of the client view.
// It is included in the ClientHello at the start of the session, and
// is also sent when the client resizes its window.
type ClientScreenSpec tdpbv1.ClientScreenSpec

// Encode encodes a ClientScreenSpec message.
func (c *ClientScreenSpec) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		ClientScreenSpec: proto.ValueOrDefault((*tdpbv1.ClientScreenSpec)(c)),
	}.Build())
}

func (*ClientScreenSpec) validate() error { return nil }

// Alert encodes an error/warning/informational message and severity code.
// Sent by the server to the client for display.
type Alert tdpbv1.Alert

// Encode encodes a Alert message.
func (a *Alert) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		Alert: proto.ValueOrDefault((*tdpbv1.Alert)(a)),
	}.Build())
}

func (*Alert) validate() error { return nil }

// MouseWheel contains a mousewheel update.
type MouseWheel tdpbv1.MouseWheel

// Encode encodes a MouseWheel message.
func (m *MouseWheel) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		MouseWheel: proto.ValueOrDefault((*tdpbv1.MouseWheel)(m)),
	}.Build())
}

func (*MouseWheel) validate() error { return nil }

// ClipboardData carries clipboard data to support copy/paste
// operations between the client and target desktop.
type ClipboardData tdpbv1.ClipboardData

// Encode encodes a ClipboardData message.
func (c *ClipboardData) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		ClipboardData: proto.ValueOrDefault((*tdpbv1.ClipboardData)(c)),
	}.Build())
}

func (c *ClipboardData) validate() error {
	if len(c.Data) > tdp.MaxClipboardDataLength {
		return tdp.ClipDataMaxLenErr
	}
	return nil
}

// MFA encodes the MFA challenge and response when per-session
// MFA is enabled.
type MFA tdpbv1.MFA

// Encode encodes a MFA message.
func (m *MFA) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		Mfa: proto.ValueOrDefault((*tdpbv1.MFA)(m)),
	}.Build())
}

func (*MFA) validate() error { return nil }

// SharedDirectoryAnnounce is sent by the client to begin sharing a directory.
type SharedDirectoryAnnounce tdpbv1.SharedDirectoryAnnounce

// Encode encodes a SharedDirectoryAnnounce message.
func (s *SharedDirectoryAnnounce) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		SharedDirectoryAnnounce: proto.ValueOrDefault((*tdpbv1.SharedDirectoryAnnounce)(s)),
	}.Build())
}

func (*SharedDirectoryAnnounce) validate() error { return nil }

// SharedDirectoryRemove is sent by the client to stop sharing a directory.
type SharedDirectoryRemove tdpbv1.SharedDirectoryRemove

// Encode encodes a SharedDirectoryAnnounce message.
func (s *SharedDirectoryRemove) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		SharedDirectoryRemove: proto.ValueOrDefault((*tdpbv1.SharedDirectoryRemove)(s)),
	}.Build())
}

func (*SharedDirectoryRemove) validate() error { return nil }

// SharedDirectoryAcknowledge is sent by the server to acknowledge a
// new shared directory.
type SharedDirectoryAcknowledge tdpbv1.SharedDirectoryAcknowledge

// Encode encodes a SharedDirectoryAcknowledge message.
func (s *SharedDirectoryAcknowledge) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		SharedDirectoryAcknowledge: proto.ValueOrDefault((*tdpbv1.SharedDirectoryAcknowledge)(s)),
	}.Build())
}

func (*SharedDirectoryAcknowledge) validate() error { return nil }

// SharedDirectoryRequest encodes various directory operation requests
// such as Info, Create, Delete, List, Read, Write, Move, or Truncate.
type SharedDirectoryRequest tdpbv1.SharedDirectoryRequest

// Encode encodes a SharedDirectoryRequest message.
func (s *SharedDirectoryRequest) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		SharedDirectoryRequest: proto.ValueOrDefault((*tdpbv1.SharedDirectoryRequest)(s)),
	}.Build())
}

func (s *SharedDirectoryRequest) validate() error {
	switch op := s.Operation.(type) {
	case *tdpbv1.SharedDirectoryRequest_Create_:
		if len(op.Create.GetPath()) > tdp.MaxPathLength {
			return tdp.StringMaxLenErr
		}
	case *tdpbv1.SharedDirectoryRequest_Delete_:
		if len(op.Delete.GetPath()) > tdp.MaxPathLength {
			return tdp.StringMaxLenErr
		}
	case *tdpbv1.SharedDirectoryRequest_Truncate_:
		if len(op.Truncate.GetPath()) > tdp.MaxPathLength {
			return tdp.StringMaxLenErr
		}
	case *tdpbv1.SharedDirectoryRequest_Read_:
		if len(op.Read.GetPath()) > tdp.MaxPathLength {
			return tdp.StringMaxLenErr
		}
		if op.Read.GetLength() > tdp.MaxFileReadWriteLength {
			return tdp.FileReadWriteMaxLenErr
		}
	case *tdpbv1.SharedDirectoryRequest_Write_:
		if len(op.Write.GetPath()) > tdp.MaxPathLength {
			return tdp.StringMaxLenErr
		}
		if len(op.Write.GetData()) > tdp.MaxFileReadWriteLength {
			return tdp.FileReadWriteMaxLenErr
		}
	case *tdpbv1.SharedDirectoryRequest_Info_:
		if len(op.Info.GetPath()) > tdp.MaxPathLength {
			return tdp.StringMaxLenErr
		}
	case *tdpbv1.SharedDirectoryRequest_List_:
		if len(op.List.GetPath()) > tdp.MaxPathLength {
			return tdp.StringMaxLenErr
		}
	case *tdpbv1.SharedDirectoryRequest_Move_:
		if len(op.Move.GetNewPath()) > tdp.MaxPathLength ||
			len(op.Move.GetOriginalPath()) > tdp.MaxPathLength {
			return tdp.StringMaxLenErr
		}
	}
	return nil
}

// SharedDirectoryResponse encodes a response to a previous SharedDirectoryRequest.
type SharedDirectoryResponse tdpbv1.SharedDirectoryResponse

// Encode encodes a SharedDirectoryResponse message.
func (s *SharedDirectoryResponse) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		SharedDirectoryResponse: proto.ValueOrDefault((*tdpbv1.SharedDirectoryResponse)(s)),
	}.Build())
}

func (s *SharedDirectoryResponse) validate() error {
	switch op := s.Operation.(type) {
	case *tdpbv1.SharedDirectoryResponse_Create_:
		if len(op.Create.GetFso().GetPath()) > tdp.MaxPathLength {
			return tdp.StringMaxLenErr
		}
	case *tdpbv1.SharedDirectoryResponse_Read_:
		if len(op.Read.GetData()) > tdp.MaxFileReadWriteLength {
			return tdp.FileReadWriteMaxLenErr
		}
	case *tdpbv1.SharedDirectoryResponse_Write_:
		if op.Write.GetBytesWritten() > tdp.MaxFileReadWriteLength {
			return tdp.FileReadWriteMaxLenErr
		}
	case *tdpbv1.SharedDirectoryResponse_Info_:
		if len(op.Info.GetFso().GetPath()) > tdp.MaxPathLength {
			return tdp.StringMaxLenErr
		}
	case *tdpbv1.SharedDirectoryResponse_List_:
		for _, fso := range op.List.GetFsoList() {
			if len(fso.GetPath()) > tdp.MaxPathLength {
				return tdp.StringMaxLenErr
			}
		}
	}
	return nil
}

// LatencyStats are sent to the client to display connection
// latency between both the user and Teleport, as well as
// between Teleport and the target desktop.
type LatencyStats tdpbv1.LatencyStats

// Encode encodes a LatencyStats message.
func (l *LatencyStats) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		LatencyStats: proto.ValueOrDefault((*tdpbv1.LatencyStats)(l)),
	}.Build())
}

func (*LatencyStats) validate() error { return nil }

// Ping is used to measure latency between the Proxy and
// target desktop.
type Ping tdpbv1.Ping

// Encodes a ping message.
func (p *Ping) Encode() ([]byte, error) {
	return marshalWithHeader(tdpbv1.Envelope_builder{
		Ping: proto.ValueOrDefault((*tdpbv1.Ping)(p)),
	}.Build())
}

func (*Ping) validate() error { return nil }

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

func WarningConstructor(msg string) tdp.Message {
	return &Alert{
		Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_WARNING,
		Message:  msg,
	}
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
		return msg, msg.validate()
	}

	// Allow the caller to distinguish unmarshal errors (likely considered fatal)
	// from an "empty" message, which could simply mean that we've received
	// a new (unsupported) message from a newer implementation.
	return nil, trace.Wrap(ErrUnknownMessage)
}

type validatableMessage interface {
	tdp.Message
	validate() error
}

// All top-level messages inside the envelope must implement
// a 'validate' method.
func messageFromEnvelope(e *tdpbv1.Envelope) validatableMessage {
	switch e.WhichPayload() {
	case tdpbv1.Envelope_ClientHello_case:
		return (*ClientHello)(e.GetClientHello())
	case tdpbv1.Envelope_ServerHello_case:
		return (*ServerHello)(e.GetServerHello())
	case tdpbv1.Envelope_PngFrame_case:
		return (*PNGFrame)(e.GetPngFrame())
	case tdpbv1.Envelope_FastPathPdu_case:
		return (*FastPathPDU)(e.GetFastPathPdu())
	case tdpbv1.Envelope_RdpResponsePdu_case:
		return (*RDPResponsePDU)(e.GetRdpResponsePdu())
	case tdpbv1.Envelope_SyncKeys_case:
		return (*SyncKeys)(e.GetSyncKeys())
	case tdpbv1.Envelope_MouseMove_case:
		return (*MouseMove)(e.GetMouseMove())
	case tdpbv1.Envelope_MouseButton_case:
		return (*MouseButton)(e.GetMouseButton())
	case tdpbv1.Envelope_KeyboardButton_case:
		return (*KeyboardButton)(e.GetKeyboardButton())
	case tdpbv1.Envelope_ClientScreenSpec_case:
		return (*ClientScreenSpec)(e.GetClientScreenSpec())
	case tdpbv1.Envelope_Alert_case:
		return (*Alert)(e.GetAlert())
	case tdpbv1.Envelope_MouseWheel_case:
		return (*MouseWheel)(e.GetMouseWheel())
	case tdpbv1.Envelope_ClipboardData_case:
		return (*ClipboardData)(e.GetClipboardData())
	case tdpbv1.Envelope_Mfa_case:
		return (*MFA)(e.GetMfa())
	case tdpbv1.Envelope_SharedDirectoryAnnounce_case:
		return (*SharedDirectoryAnnounce)(e.GetSharedDirectoryAnnounce())
	case tdpbv1.Envelope_SharedDirectoryAcknowledge_case:
		return (*SharedDirectoryAcknowledge)(e.GetSharedDirectoryAcknowledge())
	case tdpbv1.Envelope_SharedDirectoryRequest_case:
		return (*SharedDirectoryRequest)(e.GetSharedDirectoryRequest())
	case tdpbv1.Envelope_SharedDirectoryResponse_case:
		return (*SharedDirectoryResponse)(e.GetSharedDirectoryResponse())
	case tdpbv1.Envelope_LatencyStats_case:
		return (*LatencyStats)(e.GetLatencyStats())
	case tdpbv1.Envelope_Ping_case:
		return (*Ping)(e.GetPing())
	case tdpbv1.Envelope_SharedDirectoryRemove_case:
		return (*SharedDirectoryRemove)(e.GetSharedDirectoryRemove())
	case tdpbv1.Envelope_SessionSelection_case:
		return (*SessionSelection)(e.GetSessionSelection())
	case tdpbv1.Envelope_RefreshRect_case:
		return (*RefreshRect)(e.GetRefreshRect())
	case tdpbv1.Envelope_EgfxBitmap_case:
		return (*EgfxBitmap)(e.GetEgfxBitmap())
	case tdpbv1.Envelope_EgfxAvcFrame_case:
		return (*EgfxAvcFrame)(e.GetEgfxAvcFrame())
	case tdpbv1.Envelope_EgfxClearCodec_case:
		return (*EgfxClearCodec)(e.GetEgfxClearCodec())
	case tdpbv1.Envelope_EgfxSolidFill_case:
		return (*EgfxSolidFill)(e.GetEgfxSolidFill())
	case tdpbv1.Envelope_EgfxSurfaceToCache_case:
		return (*EgfxSurfaceToCache)(e.GetEgfxSurfaceToCache())
	case tdpbv1.Envelope_EgfxCacheToSurface_case:
		return (*EgfxCacheToSurface)(e.GetEgfxCacheToSurface())
	case tdpbv1.Envelope_EgfxEvictCacheEntry_case:
		return (*EgfxEvictCacheEntry)(e.GetEgfxEvictCacheEntry())
	case tdpbv1.Envelope_EgfxSurfaceToSurface_case:
		return (*EgfxSurfaceToSurface)(e.GetEgfxSurfaceToSurface())
	case tdpbv1.Envelope_EgfxWireToSurface2_case:
		return (*EgfxWireToSurface2)(e.GetEgfxWireToSurface2())
	case tdpbv1.Envelope_EgfxDeleteEncodingContext_case:
		return (*EgfxDeleteEncodingContext)(e.GetEgfxDeleteEncodingContext())
	case tdpbv1.Envelope_EgfxUncompressed_case:
		return (*EgfxUncompressed)(e.GetEgfxUncompressed())
	case tdpbv1.Envelope_EgfxPlanar_case:
		return (*EgfxPlanar)(e.GetEgfxPlanar())
	case tdpbv1.Envelope_EgfxAvc420_case:
		return (*EgfxAvc420)(e.GetEgfxAvc420())
	case tdpbv1.Envelope_EgfxEndFrame_case:
		return (*EgfxEndFrame)(e.GetEgfxEndFrame())
	default:
		return nil
	}
}
