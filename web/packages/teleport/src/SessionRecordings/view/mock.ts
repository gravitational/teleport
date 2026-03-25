/**
 * Teleport
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

import { ws } from 'msw';

import {
  SessionRecordingMessageType,
  type SessionRecordingMessage,
  type SessionRecordingMetadata,
  type SessionRecordingThumbnail,
} from 'teleport/services/recordings';

interface MetadataHandlerOptions {
  shouldError?: boolean;
  errorMessage?: string;
}

export function createMetadataHandler(
  metadata: SessionRecordingMetadata,
  frames: SessionRecordingThumbnail[],
  options?: MetadataHandlerOptions
) {
  return ws
    .link(
      'ws://localhost/v1/webapi/sites/:clusterId/sessionrecording/:sessionId/metadata/ws'
    )
    .addEventListener('connection', ({ client }) => {
      function sendMessage(message: SessionRecordingMessage) {
        client.send(JSON.stringify(message));
      }

      // Send messages immediately after connection
      if (options?.shouldError) {
        sendMessage({
          type: SessionRecordingMessageType.Error,
          data: { message: options?.errorMessage },
        });
        client.close();
        return;
      }

      sendMessage({
        type: SessionRecordingMessageType.Metadata,
        data: metadata,
      });

      for (const frame of frames) {
        sendMessage({
          type: SessionRecordingMessageType.Thumbnail,
          data: frame,
        });
      }

      setTimeout(() => {
        client.close();
      }, 100);
    });
}

export const MOCK_METADATA: SessionRecordingMetadata = {
  startTime: 1609459200, // Jan 1, 2021
  endTime: 1609462800, // Jan 1, 2021
  duration: 3600000, // 1 hour in milliseconds
  user: 'testuser',
  resourceName: 'test-server',
  clusterName: 'test-cluster',
  events: [],
  startCols: 80,
  startRows: 24,
  type: 'ssh',
};
