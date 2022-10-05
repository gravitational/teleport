// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clusters

import (
	"io"
	"os"
	"time"

	"github.com/gravitational/teleport/lib/sshutils/sftp"
	api "github.com/gravitational/teleport/lib/teleterm/api/protogen/golang/v1"

	"github.com/gravitational/trace"
)

func (c *Cluster) TransferFile(request *api.FileTransferRequest, server api.TerminalService_TransferFileServer) error {
	var config *sftp.Config
	var configErr error

	direction := request.GetDirection()
	if direction == api.FileTransferDirection_FILE_TRANSFER_DIRECTION_UNSPECIFIED {
		return trace.BadParameter("Unexpected file transfer direction: %q", direction)
	}

	if direction == api.FileTransferDirection_FILE_TRANSFER_DIRECTION_DOWNLOAD {
		config, configErr = sftp.CreateDownloadConfig(request.GetSource(), request.GetDestination(), sftp.Options{})
	}
	if direction == api.FileTransferDirection_FILE_TRANSFER_DIRECTION_UPLOAD {
		config, configErr = sftp.CreateUploadConfig([]string{request.GetSource()}, request.GetDestination(), sftp.Options{})
	}
	if configErr != nil {
		return trace.Wrap(configErr)
	}

	config.ProgressWriter = func(fileInfo os.FileInfo) io.Writer {
		return newGrpcFileTransferProgress(fileInfo.Size(), server)
	}

	err := addMetadataToRetryableError(server.Context(), func() error {
		err := c.clusterClient.TransferFiles(server.Context(), request.GetLogin(), request.GetHostname()+":0", config)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}

func newGrpcFileTransferProgress(fileSize int64, writer api.TerminalService_TransferFileServer) io.Writer {
	return &GrpcFileTransferProgress{
		transferFileServer: writer,
		sentSize:           0,
		fileSize:           fileSize,
	}
}

type GrpcFileTransferProgress struct {
	transferFileServer api.TerminalService_TransferFileServer
	sentSize           int64
	fileSize           int64
	lastSentPercentage uint32
	lastSentAt         time.Time
}

func (p *GrpcFileTransferProgress) Write(bytes []byte) (n int, err error) {
	bytesLength := len(bytes)
	p.sentSize += int64(bytesLength)
	percentage := uint32(p.sentSize * 100 / p.fileSize)

	if p.canSendProgress(percentage) {
		writeErr := p.transferFileServer.Send(&api.FileTransferProgress{Percentage: percentage})
		if writeErr != nil {
			return bytesLength, writeErr
		}
		p.lastSentAt = time.Now()
		p.lastSentPercentage = percentage
	}

	return bytesLength, nil
}

func (p *GrpcFileTransferProgress) canSendProgress(percentage uint32) bool {
	hasIntervalPassed := time.Since(p.lastSentAt).Milliseconds() > 100
	hasPercentageChanged := percentage != p.lastSentPercentage
	return hasIntervalPassed && hasPercentageChanged
}
