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

import { screen } from '@testing-library/react';
import { http, HttpResponse, ws } from 'msw';
import { setupServer } from 'msw/node';
import { MemoryRouter, Route } from 'react-router-dom';

import { render, testQueryClient } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { MockAuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket.mock';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  SessionRecordingMessageType,
  type RecordingType,
  type SessionRecordingMessage,
  type SessionRecordingMetadata,
  type SessionRecordingThumbnail,
} from 'teleport/services/recordings';

import { ViewSessionRecordingRoute } from './ViewSessionRecordingRoute';

jest.spyOn(cfg, 'getSessionRecordingMetadataUrl').mockImplementation(() => {
  return 'ws://localhost/v1/webapi/sites/:clusterId/sessionrecording/:sessionId/metadata/ws';
});

jest.mock('teleport/lib/AuthenticatedWebSocket', () => ({
  AuthenticatedWebSocket: MockAuthenticatedWebSocket,
}));

const server = setupServer();

beforeAll(() => {
  server.listen();
});

afterEach(() => {
  server.resetHandlers();
  testQueryClient.clear();
  jest.clearAllMocks();
});

afterAll(() => {
  server.close();
});

const mockMetadata: SessionRecordingMetadata = {
  startTime: 1609459200, // Jan 1, 2021
  endTime: 1609462800, // Jan 1, 2021
  duration: 3600000, // 1 hour in milliseconds
  user: 'testuser',
  resource: 'test-server',
  clusterName: 'test-cluster',
  events: [],
  startCols: 80,
  startRows: 24,
  type: 'ssh',
};

// mock the RecordingPlayer component
jest.mock('./RecordingPlayer', () => ({
  RecordingPlayer: ({ clusterId, sessionId, durationMs, recordingType }) => (
    <div data-testid="recording-player">
      RecordingPlayer: {clusterId}/{sessionId}/{durationMs}/{recordingType}
    </div>
  ),
}));

function setupTest(initialEntry?: string) {
  const ctx = createTeleportContext();

  return render(
    <MemoryRouter initialEntries={initialEntry ? [initialEntry] : undefined}>
      <ContextProvider ctx={ctx}>
        <Route path={cfg.routes.player}>
          <ViewSessionRecordingRoute />
        </Route>
      </ContextProvider>
    </MemoryRouter>
  );
}

function withSessionDuration(durationMs: number, recordingType: RecordingType) {
  server.use(
    http.get(cfg.api.sessionDurationPath, req =>
      HttpResponse.json({
        durationMs,
        recordingType,
      })
    )
  );
}

describe('with metadata', () => {
  it('renders metadata correctly if recordingType is in the URL', async () => {
    server.use(createMetadataHandler(mockMetadata, []));

    setupTest(
      cfg.getPlayerRoute(
        {
          clusterId: 'test-cluster',
          sid: 'test-session',
        },
        {
          recordingType: 'ssh',
          durationMs: 3600000,
        }
      )
    );

    expect(await screen.findByText('test-server')).toBeInTheDocument();
  });

  it('falls back to loading the duration if metadata is not available and no URL params', async () => {
    server.use(
      createMetadataHandler(mockMetadata, [], {
        shouldError: true,
        errorMessage: 'Metadata not available',
      })
    );

    withSessionDuration(3600000, 'ssh');

    setupTest(
      cfg.getPlayerRoute(
        {
          clusterId: 'test-cluster',
          sid: 'test-session',
        },
        {
          recordingType: undefined,
          durationMs: undefined,
        }
      )
    );

    expect(await screen.findByTestId('recording-player')).toBeInTheDocument();
  });

  it('falls back to the session player if metadata is not available', async () => {
    jest.spyOn(console, 'error').mockImplementation(() => {});

    server.use(
      createMetadataHandler(mockMetadata, [], {
        shouldError: true,
        errorMessage: 'Metadata not available',
      })
    );

    setupTest(
      cfg.getPlayerRoute(
        {
          clusterId: 'test-cluster',
          sid: 'test-session',
        },
        {
          recordingType: 'ssh',
          durationMs: 3600000,
        }
      )
    );

    expect(await screen.findByTestId('recording-player')).toBeInTheDocument();
  });
});

interface MetadataHandlerOptions {
  shouldError?: boolean;
  errorMessage?: string;
}

function createMetadataHandler(
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
        data: { ...mockMetadata, ...metadata },
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
