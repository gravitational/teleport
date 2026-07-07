//go:build desktop_access_rdp

/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package rdpclient

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"sync/atomic"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type desktopServiceServer struct {
	tdpbv1.UnimplementedDesktopServiceServer
	client *Client
}

func (s *desktopServiceServer) Session(stream tdpbv1.DesktopService_SessionServer) error {
    // Send the initial gRPC headers to indicate that the session has started.
    // The client waits for these headers before sending any messages, so omitting
    // this would cause it to block indefinitely.
	if err := stream.SendHeader(nil); err != nil {
		return trace.Wrap(err)
	}

	rustToTDPErrCh := make(chan error, 1)
	go func() {
		rustToTDPErrCh <- s.forwardRustToTDP(stream)
	}()

	tdpToRustErrCh := make(chan error, 1)
	go func() {
		tdpToRustErrCh <- s.forwardTDPToRust(stream)
	}()

	select {
	case err := <-rustToTDPErrCh:
		return trace.Wrap(err)
	case err := <-tdpToRustErrCh:
		return trace.Wrap(err)
	case <-stream.Context().Done():
		return trace.Wrap(stream.Context().Err())
	}
}

func (s *desktopServiceServer) forwardRustToTDP(stream tdpbv1.DesktopService_SessionServer) error {
	for {
		envelope, err := stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}

		msg := tdpb.MessageFromEnvelope(envelope)
		if msg == nil {
			s.client.cfg.Logger.WarnContext(context.Background(), "Received unknown TDPB message from the Rust client", logutils.TypeAttr(envelope))
		}

		switch m := msg.(type) {
		case *tdpb.FastPathPDU:
			// Notify the `forwardTDPToRust` goroutine that we're ready for input.
			// Input can only be sent after connection was established, which we infer
			// from the fact that a fast path pdu was sent.
			atomic.StoreUint32(&s.client.readyForInput, 1)

			if err := s.client.conn.WriteMessage(m); err != nil {
				s.client.cfg.Logger.ErrorContext(context.Background(), "failed handling RDPFastPathPDU", "error", err)
				return trace.Wrap(err)
			}
		case *tdpb.ServerHello:
			s.client.cfg.Logger.DebugContext(context.Background(), "Received RDP channel IDs", "io_channel_id", m.ActivationSpec.IoChannelId, "user_channel_id", m.ActivationSpec.UserChannelId)

			// Note: RDP doesn't always use the resolution we asked for.
			// This is especially true when we request dimensions that are not a multiple of 4.
			s.client.cfg.Logger.DebugContext(context.Background(), "RDP server provided resolution", "width", m.ActivationSpec.ScreenWidth, "height", m.ActivationSpec.ScreenHeight)

			if err := s.client.conn.WriteMessage(m); err != nil {
				s.client.cfg.Logger.ErrorContext(context.Background(), "failed handling connection initialization", "error", err)
				return trace.Wrap(err)
			}
		case *tdpb.ClipboardData:
			// Ignore empty clipboard data to mitigate a part of a clipboard
			// issue where sometimes the clipboard data is wiped.
			if len(m.Data) == 0 {
				s.client.cfg.Logger.DebugContext(context.Background(), "Received empty clipboard data from the Rust client, ignoring message")
				continue
			}

			s.client.cfg.Logger.DebugContext(context.Background(), "Received clipboard data from the Rust client", "len", len(m.Data))

			if err := s.client.conn.WriteMessage(m); err != nil {
				s.client.cfg.Logger.ErrorContext(context.Background(), "failed handling remote copy", "error", err)
				return trace.Wrap(err)
			}
		case *tdpb.SharedDirectoryAcknowledge:
			if !s.client.cfg.AllowDirectorySharing {
				return trace.Wrap(errors.New("received shared directory acknowledge message but directory sharing is not allowed"))
			}

			if err := s.client.conn.WriteMessage(m); err != nil {
				s.client.cfg.Logger.ErrorContext(context.Background(), "failed to send shared directory acknowledge", "error", err)
				return trace.Wrap(err)
			}
		case *tdpb.SharedDirectoryRequest:
			if !s.client.cfg.AllowDirectorySharing {
				return trace.Wrap(errors.New("received shared directory request message but directory sharing is not allowed"))
			}

			if err := s.client.conn.WriteMessage(m); err != nil {
				s.client.cfg.Logger.ErrorContext(context.Background(), "failed to send shared directory request", "error", err, "operation", m.Operation)
				return trace.Wrap(err)
			}
		default:
			s.client.cfg.Logger.WarnContext(context.Background(), "Skipping unimplemented TDP message from Rust client", logutils.TypeAttr(msg))
		}
	}
}

func (s *desktopServiceServer) forwardTDPToRust(stream tdpbv1.DesktopService_SessionServer) error {
	// we will disable ping only if the env var is truthy
	disableDesktopPing, _ := strconv.ParseBool(os.Getenv("TELEPORT_DISABLE_DESKTOP_LATENCY_DETECTOR_PING"))

	var withheldResize *tdpb.ClientScreenSpec
	for {
		msg, err := s.client.conn.ReadMessage()
		if utils.IsOKNetworkError(err) {
			return nil
		} else if err != nil {
			s.client.cfg.Logger.WarnContext(context.Background(), "Failed reading TDPB input message", "error", err)
			return err
		}
		if m, ok := msg.(*tdpb.Ping); ok {
			go func() {
				// Upon receiving a ping message, we make a connection
				// to the host and send the same message back to the proxy.
				// The proxy will then compute the round trip time.
				if !disableDesktopPing {
					conn, err := net.Dial("tcp", s.client.cfg.Addr)
					if err == nil {
						conn.Close()
					}
				}
				if err := s.client.conn.WriteMessage(m); err != nil {
					s.client.cfg.Logger.WarnContext(context.Background(), "Failed writing TDPB ping message", "error", err)
				}
			}()
			continue
		}

		if atomic.LoadUint32(&s.client.readyForInput) == 0 {
			switch m := msg.(type) {
			case *tdpb.ClientScreenSpec:
				// Withhold the latest screen size until the client is ready for input. This ensures
				// that the client receives the correct screen size when it is ready.
				withheldResize = m
				s.client.cfg.Logger.DebugContext(context.Background(), "Withholding screen size until client is ready for input", "width", m.Width, "height", m.Height)
			default:
				// Ignore all messages except ClientScreenSpec until the client is ready for input.
				s.client.cfg.Logger.DebugContext(context.Background(), "Dropping TDP input message, not ready for input")
			}

			continue
		}

		// If the message was due to user input, then we update client activity
		// in order to refresh the client_idle_timeout checks.
		//
		// Note: we count some of the directory sharing messages as client activity
		// because we don't want a session to be closed due to inactivity during a large
		// file transfer.
		switch msg.(type) {
		case *tdpb.KeyboardButton, *tdpb.MouseMove, *tdpb.MouseButton, *tdpb.MouseWheel,
			*tdpb.SharedDirectoryAnnounce, *tdpb.SharedDirectoryRemove, *tdpb.SharedDirectoryResponse:

			s.client.UpdateClientActivity()
		}

		if withheldResize != nil {
			s.client.cfg.Logger.DebugContext(context.Background(), "Sending withheld screen size to client")
			if err := s.handleTDPInput(stream, withheldResize); err != nil {
				return trace.Wrap(err)
			}
			withheldResize = nil
		}

		if err := s.handleTDPInput(stream, msg); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (s *desktopServiceServer) handleTDPInput(stream tdpbv1.DesktopService_SessionServer, msg tdp.Message) error {
	switch m := msg.(type) {
	case *tdpb.ClientScreenSpec:
		// If the client has specified a fixed screen size, we don't
		// need to send a screen resize event.
		if s.client.cfg.hasSizeOverride() {
			return nil
		}

		// Update the display scale factor if the protobuf message carries one.
		if m.Scale > 0 {
			s.client.requestedScale = uint16(m.Scale)
		}

		w, h := applyScale(m.Width, m.Height, s.client.requestedScale)
		s.client.cfg.Logger.DebugContext(context.Background(), "Client changed screen size", "css_width", m.Width, "css_height", m.Height, "scale", s.client.requestedScale, "width", w, "height", h)
	case *tdpb.ClipboardData:
		if !s.client.cfg.AllowClipboard {
			s.client.cfg.Logger.DebugContext(context.Background(), "Received clipboard data, but clipboard is disabled")
			return nil
		}
		if len(m.Data) == 0 {
			s.client.cfg.Logger.WarnContext(context.Background(), "Received an empty clipboard message")
			return nil
		}
	case *tdpb.SharedDirectoryAnnounce:
        if !s.client.cfg.AllowDirectorySharing {
            return nil
        }
        if m.DirectoryId == 0 {
            return trace.BadParameter("Zero is not a valid directory identifier")
        }
	case *tdpb.SharedDirectoryRemove, *tdpb.SharedDirectoryResponse:
		if !s.client.cfg.AllowDirectorySharing {
			return nil
		}
	case *tdpb.RDPResponsePDU:
		if len(m.Response) == 0 {
			s.client.cfg.Logger.ErrorContext(context.Background(), "response PDU empty")
		}
	case *tdpb.MouseMove, *tdpb.MouseButton, *tdpb.MouseWheel, *tdpb.KeyboardButton, *tdpb.SyncKeys:
		// forwarded as-is
	default:
		s.client.cfg.Logger.WarnContext(
			context.Background(),
			"Skipping unimplemented TDP message",
			"type", logutils.TypeAttr(msg),
		)
		return nil
	}

	envelopeMsg := tdpb.MessageToEnvelope(msg)
	return trace.Wrap(stream.Send(envelopeMsg))
}

func (s *desktopServiceServer) ReadRdpLicense(ctx context.Context, meta *tdpbv1.LicenseMetadata) (*tdpbv1.ReadRdpLicenseResponse, error) {
	license, err := s.client.readRDPLicense(ctx, types.RDPLicenseKey{
		Version:   meta.GetVersion(),
		Issuer:    meta.GetIssuer(),
		Company:   meta.GetCompany(),
		ProductID: meta.GetProductId(),
	})

	if trace.IsNotFound(err) {
		return &tdpbv1.ReadRdpLicenseResponse{}, nil
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return tdpbv1.ReadRdpLicenseResponse_builder{
		LicenseInfo: wrapperspb.Bytes(license),
	}.Build(), nil
}

func (s *desktopServiceServer) WriteRdpLicense(ctx context.Context, license *tdpbv1.License) (*emptypb.Empty, error) {
	meta := license.GetMetadata()
	if meta == nil {
		return nil, trace.BadParameter("missing license metadata")
	}

	err := s.client.writeRDPLicense(ctx, types.RDPLicenseKey{
		Version:   meta.GetVersion(),
		Issuer:    meta.GetIssuer(),
		Company:   meta.GetCompany(),
		ProductID: meta.GetProductId(),
	}, license.GetLicenseInfo())

	return &emptypb.Empty{}, trace.Wrap(err)
}

func (s *desktopServiceServer) GetCertificateAndKey(_ context.Context, _ *emptypb.Empty) (*tdpbv1.CertificateAndKey, error) {
	return tdpbv1.CertificateAndKey_builder{
		Cert: s.client.certDER,
		Key:  s.client.keyDER,
	}.Build(), nil
}
