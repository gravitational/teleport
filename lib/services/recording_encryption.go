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

package services

import (
	"context"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
)

// RecordingEncryption handles CRUD operations for the RecordingEncryption resource.
type RecordingEncryption interface {
	// CreateRecordingEncryption creates a new RecordingEncryption in the backend if one
	// does not already exist.
	CreateRecordingEncryption(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) (*recordingencryptionv1.RecordingEncryption, error)
	// UpdateRecordingEncryption replaces the RecordingEncryption resource with the given one.
	UpdateRecordingEncryption(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) (*recordingencryptionv1.RecordingEncryption, error)
	// DeleteRecordingEncryption removes the RecordingEncryption from the cluster.
	DeleteRecordingEncryption(ctx context.Context) error
	// GetRecordingEncryption retrieves the RecordingEncryption for the cluster.
	GetRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error)
}
