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
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/sshutils/sftp"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

type FileTransferProgressSender = func(progress *api.FileTransferProgress) error

func (c *Cluster) TransferFile(ctx context.Context, request *api.FileTransferRequest, sendProgress FileTransferProgressSender) error {
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
		err := c.clusterClient.TransferFiles(ctx, request.GetLogin(), serverUUID+":0", config)
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
