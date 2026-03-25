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
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
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

	conn, err := proxyClient.ProxyWindowsDesktopSession(ctx, clusterClient.SiteName, s.desktopName(), cert, tlsConfig.RootCAs)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	// Now that we have a connection to the desktop service, we can
	// send the username.
	tdpConn := tdp.NewConn(conn, legacy.Decode)
	defer tdpConn.Close()
	err = tdpConn.WriteMessage(legacy.ClientUsername{Username: s.login})
	if err != nil {
		return trace.Wrap(err)
	}

	downstreamRW, err := streamutils.NewReadWriter(
		&clientStream{
			stream: stream,
		},
		streamutils.WithDisabledChunking(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	fsHandle := fsRequestHandler{
		directoryAccessProvider: s,
	}

	serverConn := tdp.NewConn(conn, legacy.Decode)
	tdpConnProxy := tdp.NewConnProxy(tdp.NewConn(downstreamRW, legacy.Decode), tdp.NewReadWriteInterceptor(serverConn, func(message tdp.Message) ([]tdp.Message, error) {
		msg, intErr := fsHandle.process(message, func(message tdp.Message) error {
			return trace.Wrap(serverConn.WriteMessage(message))
		})
		if intErr != nil {
			// Treat all file system errors as warnings, do not interrupt the connection.
			return []tdp.Message{legacy.Alert{
				Message:  intErr.Error(),
				Severity: legacy.SeverityWarning,
			}}, nil
		}

		if msg != nil {
			return []tdp.Message{msg}, nil
		}
		return nil, nil
	}, nil))

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
	decoded, err := legacy.Decode(bytes.NewBuffer(data))
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
	case legacy.SharedDirectoryInfoResponse,
		legacy.SharedDirectoryCreateResponse,
		legacy.SharedDirectoryDeleteResponse,
		legacy.SharedDirectoryReadResponse,
		legacy.SharedDirectoryWriteResponse,
		legacy.SharedDirectoryMoveResponse,
		legacy.SharedDirectoryListResponse,
		legacy.SharedDirectoryTruncateResponse:
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
	case legacy.SharedDirectoryInfoRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryInfoRequest(r, sendToServer))
	case legacy.SharedDirectoryListRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryListRequest(r, sendToServer))
	case legacy.SharedDirectoryReadRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryReadRequest(r, sendToServer))
	case legacy.SharedDirectoryMoveRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryMoveRequest(r, sendToServer))
	case legacy.SharedDirectoryWriteRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryWriteRequest(r, sendToServer))
	case legacy.SharedDirectoryTruncateRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryTruncateRequest(r, sendToServer))
	case legacy.SharedDirectoryCreateRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryCreateRequest(r, sendToServer))
	case legacy.SharedDirectoryDeleteRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryDeleteRequest(r, sendToServer))
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

func (d *fsRequestHandler) handleSharedDirectoryInfoRequest(r legacy.SharedDirectoryInfoRequest, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}

	info, err := dirAccess.Stat(r.Path)
	if err == nil {
		return trace.Wrap(sendToServer(legacy.SharedDirectoryInfoResponse{
			CompletionID: r.CompletionID,
			ErrCode:      uint32(SharedDirectoryErrCodeNil),
			Fso:          toFso(info),
		}))
	}
	if errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(sendToServer(legacy.SharedDirectoryInfoResponse{
			CompletionID: r.CompletionID,
			ErrCode:      uint32(SharedDirectoryErrCodeDoesNotExist),
			Fso: legacy.FileSystemObject{
				LastModified: 0,
				Size:         0,
				FileType:     0,
				IsEmpty:      0,
				Path:         "",
			}}))
	}
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryListRequest(r legacy.SharedDirectoryListRequest, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}
	contents, err := dirAccess.ReadDir(r.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	fsoList := make([]legacy.FileSystemObject, len(contents))
	for i, content := range contents {
		fsoList[i] = toFso(content)
	}

	err = sendToServer(legacy.SharedDirectoryListResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeNil),
		FsoList:      fsoList,
	})
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryReadRequest(r legacy.SharedDirectoryReadRequest, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}

	buf := make([]byte, r.Length)
	n, err := dirAccess.Read(r.Path, int64(r.Offset), buf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(legacy.SharedDirectoryReadResponse{
		CompletionID:   r.CompletionID,
		ErrCode:        uint32(SharedDirectoryErrCodeNil),
		ReadDataLength: uint32(n),
		ReadData:       buf[:n],
	})
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryMoveRequest(r legacy.SharedDirectoryMoveRequest, sendToServer func(message tdp.Message) error) error {
	err := sendToServer(legacy.SharedDirectoryMoveResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeFailed),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.NotImplemented("Moving or renaming files and directories within a shared directory is not supported.")
}

func (d *fsRequestHandler) handleSharedDirectoryWriteRequest(r legacy.SharedDirectoryWriteRequest, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}
	bytesWritten, err := dirAccess.Write(r.Path, int64(r.Offset), r.WriteData)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(legacy.SharedDirectoryWriteResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeNil),
		BytesWritten: uint32(bytesWritten),
	})
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryTruncateRequest(r legacy.SharedDirectoryTruncateRequest, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}
	err = dirAccess.Truncate(r.Path, int64(r.EndOfFile))
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(legacy.SharedDirectoryTruncateResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeNil),
	})
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryCreateRequest(r legacy.SharedDirectoryCreateRequest, sendToServer func(message tdp.Message) error) error {
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

	err = sendToServer(legacy.SharedDirectoryCreateResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeNil),
		Fso:          toFso(info),
	})
	return trace.Wrap(err)
}

func (d *fsRequestHandler) handleSharedDirectoryDeleteRequest(r legacy.SharedDirectoryDeleteRequest, sendToServer func(message tdp.Message) error) error {
	dirAccess, err := d.directoryAccessProvider.GetDirectoryAccess()
	if err != nil {
		return trace.Wrap(err)
	}
	err = dirAccess.Delete(r.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(legacy.SharedDirectoryDeleteResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeNil),
	})
	return trace.Wrap(err)
}

func toFso(info *FileOrDirInfo) legacy.FileSystemObject {
	obj := legacy.FileSystemObject{
		LastModified: uint64(info.LastModified),
		Size:         uint64(info.Size),
		FileType:     uint32(info.FileType),
		IsEmpty:      1,
		Path:         info.Path,
	}
	if info.IsEmpty {
		obj.IsEmpty = 0
	}

	return obj
}
