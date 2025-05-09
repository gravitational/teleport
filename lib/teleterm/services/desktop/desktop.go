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
	"context"
	"errors"
	"os"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/proxy"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

// Connect starts a remote desktop session.
func Connect(ctx context.Context, stream grpc.BidiStreamingServer[api.ConnectToDesktopRequest, api.ConnectToDesktopResponse], clusterClient *client.TeleportClient, proxyClient *proxy.Client, desktopName, login string, directoryAccess *DirectoryAccess) error {
	keyRing, err := clusterClient.IssueUserCertsWithMFA(ctx, client.ReissueParams{
		RouteToCluster: clusterClient.SiteName,
		TTL:            clusterClient.KeyTTL,
		RouteToWindowsDesktop: proto.RouteToWindowsDesktop{
			WindowsDesktop: desktopName,
			Login:          login,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	cert, err := keyRing.WindowsDesktopTLSCert(desktopName)
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConfig, err := clusterClient.LoadTLSConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	conn, err := proxyClient.ProxyWindowsDesktopSession(ctx, clusterClient.SiteName, desktopName, cert, tlsConfig.RootCAs)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	// Now that we have a connection to the desktop service, we can
	// send the username.
	tdpConn := tdp.NewConn(conn)
	defer tdpConn.Close()
	err = tdpConn.WriteMessage(tdp.ClientUsername{Username: login})
	if err != nil {
		return trace.Wrap(err)
	}

	downstreamRW, err := streamutils.NewReadWriter(&clientStream{
		stream: stream,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	interceptor := serverInterceptor{
		directoryAccess: directoryAccess,
	}

	tdpConnProxy := tdp.NewConnProxy(downstreamRW, conn, func(tdpConn *tdp.Conn, message tdp.Message) (tdp.Message, error) {
		msg, intErr := interceptor.process(message, func(message tdp.Message) error {
			return trace.Wrap(tdpConn.WriteMessage(message))
		})
		if intErr != nil {
			// Treat all file system errors as warnings, do not interrupt the connection.
			return tdp.Alert{
				Message:  intErr.Error(),
				Severity: tdp.SeverityWarning,
			}, nil
		}
		return msg, nil
	})

	return trace.Wrap(tdpConnProxy.Run(ctx))
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
	decoded, err := tdp.Decode(data)
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
	case tdp.SharedDirectoryInfoResponse,
		tdp.SharedDirectoryCreateResponse,
		tdp.SharedDirectoryDeleteResponse,
		tdp.SharedDirectoryReadResponse,
		tdp.SharedDirectoryWriteResponse,
		tdp.SharedDirectoryMoveResponse,
		tdp.SharedDirectoryListResponse,
		tdp.SharedDirectoryTruncateResponse:
		return trace.AccessDenied("file system messages are not allowed from the renderer process")
	default:
		return nil
	}
}

// serverInterceptor intercepts and processes messages sent from the server to the client.
type serverInterceptor struct {
	directoryAccess *DirectoryAccess
}

func (d *serverInterceptor) process(msg tdp.Message, sendToServer func(message tdp.Message) error) (tdp.Message, error) {
	switch r := msg.(type) {
	case tdp.SharedDirectoryInfoRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryInfoRequest(r, sendToServer))
	case tdp.SharedDirectoryListRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryListRequest(r, sendToServer))
	case tdp.SharedDirectoryReadRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryReadRequest(r, sendToServer))
	case tdp.SharedDirectoryMoveRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryMoveRequest(r, sendToServer))
	case tdp.SharedDirectoryWriteRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryWriteRequest(r, sendToServer))
	case tdp.SharedDirectoryTruncateRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryTruncateRequest(r, sendToServer))
	case tdp.SharedDirectoryCreateRequest:
		return nil, trace.Wrap(d.handleSharedDirectoryCreateRequest(r, sendToServer))
	case tdp.SharedDirectoryDeleteRequest:
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

func (d *serverInterceptor) handleSharedDirectoryInfoRequest(r tdp.SharedDirectoryInfoRequest, sendToServer func(message tdp.Message) error) error {
	info, err := d.directoryAccess.Stat(r.Path)
	if err == nil {
		return trace.Wrap(sendToServer(tdp.SharedDirectoryInfoResponse{
			CompletionID: r.CompletionID,
			ErrCode:      uint32(SharedDirectoryErrCodeNil),
			Fso:          toFso(info),
		}))
	}
	if errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(sendToServer(tdp.SharedDirectoryInfoResponse{
			CompletionID: r.CompletionID,
			ErrCode:      uint32(SharedDirectoryErrCodeDoesNotExist),
			Fso: tdp.FileSystemObject{
				LastModified: 0,
				Size:         0,
				FileType:     0,
				IsEmpty:      0,
				Path:         "",
			}}))
	}
	return trace.Wrap(err)
}

func (d *serverInterceptor) handleSharedDirectoryListRequest(r tdp.SharedDirectoryListRequest, sendToServer func(message tdp.Message) error) error {
	contents, err := d.directoryAccess.ReadDir(r.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	fsoList := make([]tdp.FileSystemObject, len(contents))
	for i, content := range contents {
		fsoList[i] = toFso(content)
	}

	err = sendToServer(tdp.SharedDirectoryListResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeNil),
		FsoList:      fsoList,
	})
	return trace.Wrap(err)
}

func (d *serverInterceptor) handleSharedDirectoryReadRequest(r tdp.SharedDirectoryReadRequest, sendToServer func(message tdp.Message) error) error {
	contents, err := d.directoryAccess.Read(r.Path, int64(r.Offset), r.Length)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(tdp.SharedDirectoryReadResponse{
		CompletionID:   r.CompletionID,
		ErrCode:        uint32(SharedDirectoryErrCodeNil),
		ReadDataLength: uint32(len(contents)),
		ReadData:       contents,
	})
	return trace.Wrap(err)
}

func (d *serverInterceptor) handleSharedDirectoryMoveRequest(r tdp.SharedDirectoryMoveRequest, sendToServer func(message tdp.Message) error) error {
	err := sendToServer(tdp.SharedDirectoryMoveResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeFailed),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.NotImplemented("Moving or renaming files and directories within a shared directory is not supported.")
}

func (d *serverInterceptor) handleSharedDirectoryWriteRequest(r tdp.SharedDirectoryWriteRequest, sendToServer func(message tdp.Message) error) error {
	bytesWritten, err := d.directoryAccess.Write(r.Path, int64(r.Offset), r.WriteData)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(tdp.SharedDirectoryWriteResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeNil),
		BytesWritten: uint32(bytesWritten),
	})
	return trace.Wrap(err)
}

func (d *serverInterceptor) handleSharedDirectoryTruncateRequest(r tdp.SharedDirectoryTruncateRequest, sendToServer func(message tdp.Message) error) error {
	err := d.directoryAccess.Truncate(r.Path, int64(r.EndOfFile))
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(tdp.SharedDirectoryTruncateResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeNil),
	})
	return trace.Wrap(err)
}

func (d *serverInterceptor) handleSharedDirectoryCreateRequest(r tdp.SharedDirectoryCreateRequest, sendToServer func(message tdp.Message) error) error {
	err := d.directoryAccess.Create(r.Path, FileType(r.FileType))
	if err != nil {
		return trace.Wrap(err)
	}

	info, err := d.directoryAccess.Stat(r.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(tdp.SharedDirectoryCreateResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeNil),
		Fso:          toFso(info),
	})
	return trace.Wrap(err)
}

func (d *serverInterceptor) handleSharedDirectoryDeleteRequest(r tdp.SharedDirectoryDeleteRequest, sendToServer func(message tdp.Message) error) error {
	err := d.directoryAccess.Delete(r.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	err = sendToServer(tdp.SharedDirectoryDeleteResponse{
		CompletionID: r.CompletionID,
		ErrCode:      uint32(SharedDirectoryErrCodeNil),
	})
	return trace.Wrap(err)
}

func toFso(info *FileOrDirInfo) tdp.FileSystemObject {
	obj := tdp.FileSystemObject{
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
