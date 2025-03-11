/*
 * *
 *  * Teleport
 *  * Copyright (C) 2024 Gravitational, Inc.
 *  *
 *  * This program is free software: you can redistribute it and/or modify
 *  * it under the terms of the GNU Affero General Public License as published by
 *  * the Free Software Foundation, either version 3 of the License, or
 *  * (at your option) any later version.
 *  *
 *  * This program is distributed in the hope that it will be useful,
 *  * but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  * GNU Affero General Public License for more details.
 *  *
 *  * You should have received a copy of the GNU Affero General Public License
 *  * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package services

import (
	sessionrecordingmetatadav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionrecordingmetatada/v1"
)

type SessionRecordingMetadata interface {
}

// MarshalSessionRecordingMetadata marshals the SessionRecordingMetadata resource to JSON.
func MarshalSessionRecordingMetadata(s *sessionrecordingmetatadav1.SessionRecordingMetadata, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(s, opts...)
}

// UnmarshalSessionRecordingMetadata unmarshals the SessionRecordingMetadata resource from JSON.
func UnmarshalSessionRecordingMetadata(data []byte, opts ...MarshalOption) (*sessionrecordingmetatadav1.SessionRecordingMetadata, error) {
	return UnmarshalProtoResource[*sessionrecordingmetatadav1.SessionRecordingMetadata](data, opts...)
}
