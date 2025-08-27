/**
 * Copyright (C) 2025 Gravitational, Inc.
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

package recordingmetadata

import (
	"context"

	"github.com/gravitational/teleport/lib/session"
)

// Service defines an interface for processing session recordings.
type Service interface {
	// ProcessSessionRecording processes the session recording associated with the
	// provided session ID.
	ProcessSessionRecording(ctx context.Context, sessionID session.ID) error
}
