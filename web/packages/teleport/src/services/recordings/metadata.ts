/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import cfg from 'teleport/config';
import { AuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket';
import { getHostName } from 'teleport/services/api';

import {
  SessionRecordingMessageType,
  type SessionRecordingMessage,
  type SessionRecordingMetadata,
  type SessionRecordingThumbnail,
} from './types';

export interface SessionRecordingMetadataWithFrames {
  metadata: SessionRecordingMetadata;
  frames: SessionRecordingThumbnail[];
}

interface FetchSessionRecordingMetadataVariables {
  clusterId: string;
  sessionId: string;
}

// fetchSessionRecordingMetadata fetches metadata and thumbnails for a session recording
// using a WebSocket connection, to avoid gRPC maximum message size limits.
// It returns a promise that resolves with the metadata and an array of thumbnails.
export function fetchSessionRecordingMetadata(
  { clusterId, sessionId }: FetchSessionRecordingMetadataVariables,
  signal?: AbortSignal
) {
  return new Promise<SessionRecordingMetadataWithFrames>((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException('Aborted', 'AbortError'));
      return;
    }

    const ws = new AuthenticatedWebSocket(
      cfg.getSessionRecordingMetadataUrl(clusterId, sessionId, getHostName())
    );

    let metadata: SessionRecordingMetadata | null = null;

    const frames: SessionRecordingThumbnail[] = [];

    function handleAbort() {
      ws.close();
      reject(new DOMException('Aborted', 'AbortError'));
    }

    signal?.addEventListener('abort', handleAbort);

    ws.onmessage = event => {
      const decoded = JSON.parse(event.data) as SessionRecordingMessage;

      switch (decoded.type) {
        case SessionRecordingMessageType.Thumbnail:
          frames.push(decoded.data);

          break;

        case SessionRecordingMessageType.Metadata:
          metadata = decoded.data;

          break;

        case SessionRecordingMessageType.Error:
          signal?.removeEventListener('abort', handleAbort);
          reject(new Error(decoded.data.message));

          return;
      }
    };

    ws.onerror = () => {
      signal?.removeEventListener('abort', handleAbort);
      reject(new Error('WebSocket connection failed'));
    };

    ws.onclose = () => {
      signal?.removeEventListener('abort', handleAbort);

      if (signal?.aborted) {
        return;
      }

      if (!metadata) {
        reject(new Error('No metadata received'));
        return;
      }

      resolve({
        metadata,
        frames,
      });
    };
  });
}
