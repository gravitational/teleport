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

package sftp

import (
	"os"
	portablepath "path"

	"github.com/pkg/sftp"
)

// RemoteFS provides API for accessing the files on
// the local file system
type RemoteFS struct {
	*sftp.Client
}

// NewRemoteFilesystem creates a new FileSystem over SFTP.
func NewRemoteFilesystem(c *sftp.Client) *RemoteFS {
	return &RemoteFS{Client: c}
}

func (r *RemoteFS) Type() string {
	return "remote"
}

func (r *RemoteFS) ReadDir(path string) ([]os.FileInfo, error) {
	fileInfos, err := r.Client.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for i := range fileInfos {
		// If the file is a valid symlink, return the info of the linked file.
		if fileInfos[i].Mode()&os.ModeSymlink != 0 {
			resolvedInfo, err := r.Stat(portablepath.Join(path, fileInfos[i].Name()))
			if err == nil {
				fileInfos[i] = resolvedInfo
			}
		}
	}

	return fileInfos, nil
}

func (r *RemoteFS) Open(path string) (File, error) {
	return r.OpenFile(path, os.O_RDONLY)
}

func (r *RemoteFS) Create(path string, _ int64) (File, error) {
	return r.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
}

func (r *RemoteFS) OpenFile(path string, flags int) (File, error) {
	return r.Client.OpenFile(path, flags)
}

func (r *RemoteFS) Mkdir(path string) error {
	return r.Client.MkdirAll(path)
}

func (r *RemoteFS) Readlink(name string) (string, error) {
	return r.Client.ReadLink(name)
}
