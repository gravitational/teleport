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

package clusters

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/sshutils/sftp"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

type FileTransferProgressSender = func(progress *api.FileTransferProgress) error

func (c *Cluster) TransferFile(ctx context.Context, clt *client.ClusterClient, request *api.FileTransferRequest, sendProgress FileTransferProgressSender) error {
	config, err := getSftpConfig(request)
	if err != nil {
		return trace.Wrap(err)
	}

	config.ProgressStream = func(fileInfo os.FileInfo) io.ReadWriter {
		return newFileTransferProgress(fileInfo.Size(), sendProgress)
	}

	// TODO(ravicious): Move URI parsing to the outermost layer.
	// https://github.com/gravitational/teleport/issues/15953
	serverURI := uri.New(request.GetServerUri())
	serverUUID := serverURI.GetServerUUID()
	if serverUUID == "" {
		return trace.BadParameter("server URI does not include server UUID")
	}

	err = AddMetadataToRetryableError(ctx, func() error {
		err := c.clusterClient.TransferFiles(ctx, clt, request.GetLogin(), serverUUID+":0", config)
		if errors.As(err, new(*sftp.NonRecursiveDirectoryTransferError)) {
			return trace.Errorf("transferring directories through Teleport Connect is not supported at the moment, please use tsh scp -r")
		}
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}

func getSftpConfig(request *api.FileTransferRequest) (*sftp.Config, error) {
	switch request.GetDirection() {
	case api.FileTransferDirection_FILE_TRANSFER_DIRECTION_DOWNLOAD:
		return sftp.CreateDownloadConfig(request.GetSource(), request.GetDestination(), sftp.Options{})
	case api.FileTransferDirection_FILE_TRANSFER_DIRECTION_UPLOAD:
		return sftp.CreateUploadConfig([]string{request.GetSource()}, request.GetDestination(), sftp.Options{})
	default:
		return nil, trace.BadParameter("Unexpected file transfer direction: %q", request.GetDirection())
	}
}

func newFileTransferProgress(fileSize int64, sendProgress FileTransferProgressSender) io.ReadWriter {
	return &fileTransferProgress{
		sendProgress: sendProgress,
		sentSize:     0,
		fileSize:     fileSize,
	}
}

type fileTransferProgress struct {
	sendProgress       FileTransferProgressSender
	sentSize           int64
	fileSize           int64
	lastSentPercentage uint32
	lastSentAt         time.Time
	lock               sync.Mutex
}

func (p *fileTransferProgress) Read(bytes []byte) (int, error) {
	return p.maybeUpdateProgress(bytes)
}

func (p *fileTransferProgress) Write(bytes []byte) (int, error) {
	return p.maybeUpdateProgress(bytes)
}

func (p *fileTransferProgress) maybeUpdateProgress(bytes []byte) (int, error) {
	bytesLength := len(bytes)

	p.lock.Lock()
	defer p.lock.Unlock()

	p.sentSize += int64(bytesLength)
	percentage := uint32(p.sentSize * 100 / p.fileSize)

	if p.shouldSendProgress(percentage) {
		err := p.sendProgress(&api.FileTransferProgress{Percentage: percentage})
		if err != nil {
			return bytesLength, trace.Wrap(err)
		}
		p.lastSentAt = time.Now()
		p.lastSentPercentage = percentage
	}

	return bytesLength, nil
}

func (p *fileTransferProgress) shouldSendProgress(percentage uint32) bool {
	hasIntervalPassed := time.Since(p.lastSentAt).Milliseconds() > 100
	hasPercentageChanged := percentage != p.lastSentPercentage
	return hasIntervalPassed && hasPercentageChanged
}
