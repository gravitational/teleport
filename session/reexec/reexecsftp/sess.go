// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package reexecsftp

// FileTransferRequest is a request to upload or download a file from a node.
type FileTransferRequest struct {
	// ID is a UUID that uniquely identifies a file transfer request
	// and is unlikely to collide with another file transfer request
	ID string
	// Requester is the Teleport User that requested the file transfer
	Requester string
	// Download is true if the request is a download, false if its an upload
	Download bool
	// Filename is the name of the file to upload.
	Filename string
	// Location of the requested download or where a file will be uploaded
	Location string
}
