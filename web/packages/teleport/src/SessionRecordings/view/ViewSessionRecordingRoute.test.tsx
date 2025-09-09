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
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { MemoryRouter, Route } from 'react-router-dom';

import { render, testQueryClient } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { MockAuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket.mock';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  type RecordingType,
  type SessionRecordingMetadata,
} from 'teleport/services/recordings';

import { createMetadataHandler } from './mock';
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
  resourceName: 'test-server',
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
    http.get(cfg.api.sessionDurationPath, () =>
      HttpResponse.json({
        durationMs,
        recordingType,
      })
    )
  );
}

test('renders metadata correctly if recordingType is in the URL', async () => {
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

test('renders the duration correctly', async () => {
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

  expect(await screen.findByText('1h')).toBeInTheDocument();
});

test('renders non-SSH recordings correctly', async () => {
  server.use(createMetadataHandler({ ...mockMetadata, type: 'desktop' }, []));

  setupTest(
    cfg.getPlayerRoute(
      {
        clusterId: 'test-cluster',
        sid: 'test-session',
      },
      {
        recordingType: 'desktop',
        durationMs: 3600000,
      }
    )
  );

  expect(
    await screen.findByText(
      'RecordingPlayer: test-cluster/test-session/3600000/desktop'
    )
  ).toBeInTheDocument();
});

test('displays the username', async () => {
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

  expect(await screen.findByText('testuser')).toBeInTheDocument();
});

test('shows the start/end time', async () => {
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

  expect(await screen.findByText('Jan 01, 2021 00:00')).toBeInTheDocument();
  expect(await screen.findByText('Jan 01, 2021 01:00')).toBeInTheDocument();
});

test('falls back to loading the duration if metadata is not available and no URL params', async () => {
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

test('falls back to the session player if metadata is not available', async () => {
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

test('shows error if metadata and duration are not available', async () => {
  jest.spyOn(console, 'error').mockImplementation(() => {});

  server.use(
    createMetadataHandler(mockMetadata, [], {
      shouldError: true,
      errorMessage: 'Metadata not available',
    }),
    http.get(cfg.api.sessionDurationPath, () =>
      HttpResponse.json(
        {
          durationMs: null,
          recordingType: null,
        },
        {
          status: 404,
        }
      )
    )
  );

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

  expect(
    await screen.findByText('Unable to determine the length of this session', {
      exact: false,
    })
  ).toBeInTheDocument();
});
