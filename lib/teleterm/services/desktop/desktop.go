// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package desktop

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/proxy"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/utils/slices"
)

// Session uniquely describes a desktop session.
// There can be only one session for the given desktop and login pair.
type Session struct {
	desktopURI uri.ResourceURI
	login      string

	dirAccess   *DirectoryAccess
	dirAccessMu sync.RWMutex
}

// NewSession initializes a Session struct for a given desktop and login.
func NewSession(desktopURI uri.ResourceURI, login string) (*Session, error) {
	if desktopURI.GetWindowsDesktopName() == "" {
		return nil, trace.BadParameter("invalid desktop URI %q", desktopURI)
	}
	if login == "" {
		return nil, trace.BadParameter("login cannot be empty")
	}

	return &Session{
		desktopURI: desktopURI,
		login:      login,
	}, nil
}

func (s *Session) desktopName() string {
	return s.desktopURI.GetWindowsDesktopName()
}

func (s *Session) SetSharedDirectory(basePath string) error {
	s.dirAccessMu.Lock()
	defer s.dirAccessMu.Unlock()

	if s.dirAccess != nil {
		return trace.AlreadyExists("directory is already shared for desktop %q and %q login", s.desktopName(), s.login)
	}

	dirAccess, err := NewDirectoryAccess(basePath)
	if err != nil {
		return trace.Wrap(err)
	}
	s.dirAccess = dirAccess

	return nil
}

func (s *Session) GetDirectoryAccess() (*DirectoryAccess, error) {
	s.dirAccessMu.RLock()
	defer s.dirAccessMu.RUnlock()

	if s.dirAccess == nil {
		return nil, trace.NotFound("directory sharing has not been initialized for desktop %q and login %q", s.desktopName(), s.login)
	}

	return s.dirAccess, nil
}

// Start starts a remote desktop session.
func (s *Session) Start(ctx context.Context, stream grpc.BidiStreamingServer[api.ConnectToDesktopRequest, api.ConnectToDesktopResponse], clusterClient *client.TeleportClient, proxyClient *proxy.Client) error {
	keyRing, err := clusterClient.IssueUserCertsWithMFA(ctx, client.ReissueParams{
		RouteToCluster: clusterClient.SiteName,
		TTL:            clusterClient.KeyTTL,
		RouteToWindowsDesktop: proto.RouteToWindowsDesktop{
			WindowsDesktop: s.desktopName(),
			Login:          s.login,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	cert, err := keyRing.WindowsDesktopTLSCert(s.desktopName())
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConfig, err := clusterClient.LoadTLSConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	// conn is the server connection
	conn, err := proxyClient.ProxyWindowsDesktopSession(ctx, clusterClient.SiteName, s.desktopName(), cert, tlsConfig.RootCAs)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	downstreamRW, err := streamutils.NewReadWriter(
		&clientStream{
			stream: stream,
		},
		streamutils.WithDisabledChunking(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// Client always speaks TDPB.
	clientConn := tdp.NewConn(downstreamRW, tdp.DecoderAdapter(tdpb.DecodePermissive))
	// Receive, enrich, and forward the ClientHello message
	msg, err := clientConn.ReadMessage()
	if err != nil {
		return trace.WrapWithMessage(err, "error listening for client hello")
	}

	hello, ok := msg.(*tdpb.ClientHello)
	if !ok {
		return trace.Errorf("expected ClientHello message but received %T", msg)
	}

	// Enrich with username
	hello.Username = s.login

	// Whether we forward the ClientHello as-is, or send a triple
	// (Username, ClientScreenSpec, ClientKeyboardLayout) depends on
	// the server's serverProtocol selection.
	serverProtocol := conn.ConnectionState().NegotiatedProtocol
	var tdpServerConn *tdp.Conn
	if serverProtocol == "teleport-tdpb-1.0" {
		// Use TDPB decoder
		tdpServerConn = tdp.NewConn(conn, tdp.DecoderAdapter(tdpb.DecodePermissive))
		// Send the client hello
		if err := tdpServerConn.WriteMessage(hello); err != nil {
			return trace.Wrap(err)
		}
	} else {
		// Use default TDP decoder
		tdpServerConn = tdp.NewConn(conn, legacy.Decode)
		defer tdpServerConn.Close()

		// Now that we have a connection to the desktop service, we can
		// send the username, clientScreenSpec, and clientKeyboardlayout.
		for _, msg := range []tdp.Message{
			legacy.ClientUsername{Username: s.login},
			legacy.ClientScreenSpec{Width: hello.ScreenSpec.Width, Height: hello.ScreenSpec.Height},
			legacy.ClientKeyboardLayout{KeyboardLayout: hello.KeyboardLayout},
		} {
			err = tdpServerConn.WriteMessage(msg)
			if err != nil {
				return trace.Wrap(err, "error sending %T message", msg)
			}
		}
	}

	fsHandle := fsRequestHandler{
		directoryAccessProvider: s,
	}

	// Install FS interceptor
	serverConn := tdp.NewReadWriteInterceptor(tdpServerConn, func(message tdp.Message) ([]tdp.Message, error) {
		msg, intErr := fsHandle.process(message, func(message tdp.Message) error {
			return trace.Wrap(tdpServerConn.WriteMessage(message))
		})
		if intErr != nil {
			// Treat all file system errors as warnings, do not interrupt the connection.
			return []tdp.Message{&tdpb.Alert{
				Message:  intErr.Error(),
				Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_WARNING,
			}}, nil
		}

		if msg != nil {
			return []tdp.Message{msg}, nil
		}
		return nil, nil
	}, nil)

	if serverProtocol != "teleport-tdpb-1.0" {
		// Wrap this in another interceptor to handle TDP translation
		serverConn = tdp.NewReadWriteInterceptor(serverConn, tdpb.TranslateToModern, tdpb.TranslateToLegacy)
	}

	tdpConnProxy := tdp.NewConnProxy(clientConn, serverConn)

	return trace.Wrap(tdpConnProxy.Run())
}

// clientStream implements the [streamutils.Source] interface
// for a [teletermv1.TerminalService_ConnectToDesktopClient].
type clientStream struct {
	stream grpc.BidiStreamingServer[api.ConnectToDesktopRequest, api.ConnectToDesktopResponse]
}

func (d clientStream) Send(p []byte) error {
	return trace.Wrap(d.stream.Send(&api.ConnectToDesktopResponse{Data: p}))
}

func (d clientStream) Recv() ([]byte, error) {
	msg, err := d.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if msg.GetTargetDesktop().GetDesktopUri() != "" || msg.GetTargetDesktop().GetLogin() != "" {
		return nil, trace.BadParameter("target desktop can be send only in the first message")
	}

	data := msg.GetData()
	if data == nil {
		return nil, trace.BadParameter("received invalid message")
	}

	// Check if the message sent from the renderer is allowed.
	decoded, err := tdpb.DecodeStrict(bytes.NewBuffer(data))
	if err != nil {
		return nil, trace.Wrap(err, "could not decode desktop message")
	}
	err = isClientMessageAllowed(decoded)
	if err != nil {
		return nil, trace.Wrap(err, "disallowed desktop message")
	}

	return data, nil
}

// isClientMessageAllowed checks whether a message from the client is allowed
// to be forwarded to the server.
//
// Responses related to shared directory operations are handled exclusively
// by tshd and should not originate from the renderer process.
func isClientMessageAllowed(msg tdp.Message) error {
	switch msg.(type) {
	case *tdpb.SharedDirectoryResponse:
		return trace.AccessDenied("file system messages are not allowed from the renderer process")
	default:
		return nil
	}
}

// fsRequestHandler handles file system messages sent from the server to the client.
//
// If a message is a file system request, it is handled internally via DirectoryAccess instead of being
// forwarded to the Electron app. The response is then sent back to the server.
// If the message is not a file system request, it is returned as-is to be forwarded to the client.
type fsRequestHandler struct {
	directoryAccessProvider directoryAccessProvider
}

type directoryAccessProvider interface {
	GetDirectoryAccess() (*DirectoryAccess, error)
}

func (d *fsRequestHandler) process(msg tdp.Message, sendToServer func(message tdp.Message) error) (tdp.Message, error) {
	switch r := msg.(type) {
	case *tdpb.SharedDirectoryRequest:
		switch op := r.Operation.(type) {
		case *tdpbv1.SharedDirectoryRequest_Info_:
			return nil, trace.Wrap(d.handleSharedDirectoryInfoRequest(r.CompletionId, op.Info, sendToServer))
		case *tdpbv1.SharedDirectoryRequest_Create_:
			return nil, trace.Wrap(d.handleSharedDirectoryCreateRequest(r.CompletionId, op.Create, sendToServer))
		case *tdpbv1.SharedDirectoryRequest_Delete_:
			return nil, trace.Wrap(d.handleSharedDirectoryDeleteRequest(r.CompletionId, op.Delete, sendToServer))
		case *tdpbv1.SharedDirectoryRequest_List_:
			return nil, trace.Wrap(d.handleSharedDirectoryListRequest(r.CompletionId, op.List, sendToServer))
		case *tdpbv1.SharedDirectoryRequest_Read_:
			return nil, trace.Wrap(d.handleSharedDirectoryReadRequest(r.CompletionId, op.Read, sendToServer))
		case *tdpbv1.SharedDirectoryRequest_Write_:
			return nil, trace.Wrap(d.handleSharedDirectoryWriteRequest(r.CompletionId, op.Write, sendToServer))
		case *tdpbv1.SharedDirectoryRequest_Move_:
			return nil, trace.Wrap(d.handleSharedDirectoryMoveRequest(r.CompletionId, op.Move, sendToServer))
		case *tdpbv1.SharedDirectoryRequest_Truncate_:
			return nil, trace.Wrap(d.handleSharedDirectoryTruncateRequest(r.CompletionId, op.Truncate, sendToServer))
		default:
			return msg, nil
		}
	default:
		return msg, nil
	}
}

type SharedDirectoryErrCode uint32

const (
	SharedDirectoryErrCodeNil SharedDirectoryErrCode = iota
	SharedDirectoryErrCodeFailed
	SharedDirectoryErrCodeDoesNotExist
	SharedDirectoryErrCodeAlreadyExists
)

func (d *fsRequestHandler) handleSharedDirectoryInfoRequest(completionID uint32, r *tdpbv1.SharedDirectoryRequest_Info, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}

	info, err := dirAccess.Stat(r.Path)
	if err == nil {
		return trace.Wrap(sendToServer(&tdpb.SharedDirectoryResponse{
			CompletionId: completionID,
			ErrorCode:    uint32(SharedDirectoryErrCodeNil),
			Operation: &tdpbv1.SharedDirectoryResponse_Info_{
				Info: &tdpbv1.SharedDirectoryResponse_Info{
					Fso: toFso(info),
				},
			},
		}))
	}
	if errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(sendToServer(&tdpb.SharedDirectoryResponse{
			CompletionId: completionID,
			ErrorCode:    uint32(SharedDirectoryErrCodeDoesNotExist),
			Operation: &tdpbv1.SharedDirectoryResponse_Info_{
				Info: &tdpbv1.SharedDirectoryResponse_Info{},
			},
		}))
	}
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryListRequest(completionID uint32, r *tdpbv1.SharedDirectoryRequest_List, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}
	contents, err := dirAccess.ReadDir(r.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(&tdpb.SharedDirectoryResponse{
		CompletionId: completionID,
		ErrorCode:    uint32(SharedDirectoryErrCodeNil),
		Operation: &tdpbv1.SharedDirectoryResponse_List_{
			List: &tdpbv1.SharedDirectoryResponse_List{
				FsoList: slices.Map(contents, toFso),
			},
		},
	})
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryReadRequest(completionID uint32, r *tdpbv1.SharedDirectoryRequest_Read, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}

	buf := make([]byte, r.Length)
	n, err := dirAccess.Read(r.Path, int64(r.Offset), buf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(&tdpb.SharedDirectoryResponse{
		CompletionId: completionID,
		ErrorCode:    uint32(SharedDirectoryErrCodeNil),
		Operation: &tdpbv1.SharedDirectoryResponse_Read_{
			Read: &tdpbv1.SharedDirectoryResponse_Read{
				Data: buf[:n],
			},
		},
	})
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryMoveRequest(completionID uint32, _ *tdpbv1.SharedDirectoryRequest_Move, sendToServer func(message tdp.Message) error) error {
	err := sendToServer(&tdpb.SharedDirectoryResponse{
		CompletionId: completionID,
		ErrorCode:    uint32(SharedDirectoryErrCodeFailed),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.NotImplemented("Moving or renaming files and directories within a shared directory is not supported.")
}

func (d *fsRequestHandler) handleSharedDirectoryWriteRequest(completionID uint32, r *tdpbv1.SharedDirectoryRequest_Write, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}
	bytesWritten, err := dirAccess.Write(r.Path, int64(r.Offset), r.Data)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(&tdpb.SharedDirectoryResponse{
		CompletionId: completionID,
		ErrorCode:    uint32(SharedDirectoryErrCodeNil),
		Operation: &tdpbv1.SharedDirectoryResponse_Write_{
			Write: &tdpbv1.SharedDirectoryResponse_Write{
				BytesWritten: uint32(bytesWritten),
			},
		},
	})
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryTruncateRequest(completionID uint32, r *tdpbv1.SharedDirectoryRequest_Truncate, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}
	err = dirAccess.Truncate(r.Path, int64(r.EndOfFile))
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(&tdpb.SharedDirectoryResponse{
		CompletionId: completionID,
		ErrorCode:    uint32(SharedDirectoryErrCodeNil),
		Operation: &tdpbv1.SharedDirectoryResponse_Truncate_{
			Truncate: &tdpbv1.SharedDirectoryResponse_Truncate{},
		},
	})
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryCreateRequest(completionID uint32, r *tdpbv1.SharedDirectoryRequest_Create, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}
	err = dirAccess.Create(r.Path, FileType(r.FileType))
	if err != nil {
		return trace.Wrap(err)
	}

	info, err := dirAccess.Stat(r.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(&tdpb.SharedDirectoryResponse{
		CompletionId: completionID,
		ErrorCode:    uint32(SharedDirectoryErrCodeNil),
		Operation: &tdpbv1.SharedDirectoryResponse_Create_{
			Create: &tdpbv1.SharedDirectoryResponse_Create{
				Fso: toFso(info),
			},
		},
	})
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryDeleteRequest(completionID uint32, r *tdpbv1.SharedDirectoryRequest_Delete, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}
	err = dirAccess.Delete(r.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(&tdpb.SharedDirectoryResponse{
		CompletionId: completionID,
		ErrorCode:    uint32(SharedDirectoryErrCodeNil),
		Operation: &tdpbv1.SharedDirectoryResponse_Delete_{
			Delete: &tdpbv1.SharedDirectoryResponse_Delete{},
		},
	})
	return trace.Wrap(err)
}

func toFso(info *FileOrDirInfo) *tdpbv1.FileSystemObject {
	return &tdpbv1.FileSystemObject{
		LastModified: uint64(info.LastModified),
		Size:         uint64(info.Size),
		FileType:     uint32(info.FileType),
		IsEmpty:      info.IsEmpty,
		Path:         info.Path,
	}
}
