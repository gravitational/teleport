import { screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { MemoryRouter, Route, Routes } from 'react-router';

import {
  enableMswServer,
  render,
  server,
  testQueryClient,
} from 'design/utils/testing';

import cfg from 'e-teleport/config';
import {
  RecordingSummaryState,
  type SessionRecordingSummary,
} from 'e-teleport/services/recordings/types';
import { ContextProvider } from 'teleport';
import { MockAuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket.mock';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { type SessionRecordingMetadata } from 'teleport/services/recordings';
import { createMetadataHandler } from 'teleport/SessionRecordings/view/mock';

import { ViewSessionRecordingRouteE } from './ViewSessionRecordingRouteE';

jest.spyOn(cfg.oss, 'getSessionRecordingMetadataUrl').mockImplementation(() => {
  return 'ws://localhost/v1/webapi/sites/:clusterId/sessionrecording/:sessionId/metadata/ws';
});

jest.mock('teleport/lib/AuthenticatedWebSocket', () => ({
  AuthenticatedWebSocket: MockAuthenticatedWebSocket,
}));

enableMswServer();

afterEach(() => {
  testQueryClient.clear();
  jest.clearAllMocks();
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
jest.mock('teleport/SessionRecordings/view/RecordingPlayer', () => ({
  RecordingPlayer: ({ clusterId, sessionId, durationMs, recordingType }) => (
    <div data-testid="recording-player">
      RecordingPlayer: {clusterId}/{sessionId}/{durationMs}/{recordingType}
    </div>
  ),
}));

function setupTest(initialEntry?: string, summarizerEnabled?: boolean) {
  const ctx = createTeleportContext();

  cfg.oss.sessionSummarizerEnabled = summarizerEnabled;

  return render(
    <MemoryRouter initialEntries={initialEntry ? [initialEntry] : undefined}>
      <ContextProvider ctx={ctx}>
        <Routes>
          <Route
            path={cfg.oss.routes.player}
            element={<ViewSessionRecordingRouteE />}
          />
        </Routes>
      </ContextProvider>
    </MemoryRouter>
  );
}

test('renders correctly', async () => {
  server.use(createMetadataHandler(mockMetadata, []));

  setupTest(
    cfg.oss.getPlayerRoute(
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
  expect(screen.getByText('1h')).toBeInTheDocument();
});

test('does not attempt to load the summary if the feature is disabled', async () => {
  const summaryHandler = jest.fn();

  server.use(
    createMetadataHandler(mockMetadata, []),
    http.get(cfg.api.sessionRecordingSummary, summaryHandler)
  );

  setupTest(
    cfg.oss.getPlayerRoute(
      {
        clusterId: 'test-cluster',
        sid: 'test-session',
      },
      {
        recordingType: 'ssh',
        durationMs: 3600000,
      }
    ),
    false /* summarizerEnabled */
  );

  expect(await screen.findByText('test-server')).toBeInTheDocument();

  expect(summaryHandler).not.toHaveBeenCalled();
});

test('shows the summary if enabled and it exists', async () => {
  server.use(createMetadataHandler(mockMetadata, []));

  server.use(
    http.get(cfg.api.sessionRecordingSummary, () =>
      HttpResponse.json({
        state: RecordingSummaryState.Success,
        content: 'This is a summary.',
        inferenceStartedAt: new Date().toISOString(),
        inferenceFinishedAt: new Date().toISOString(),
      })
    )
  );

  setupTest(
    cfg.oss.getPlayerRoute(
      {
        clusterId: 'test-cluster',
        sid: 'test-session',
      },
      {
        recordingType: 'ssh',
        durationMs: 3600000,
      }
    ),
    true /* summarizerEnabled */
  );

  expect(await screen.findByText('This is a summary.')).toBeInTheDocument();
});

test('shows the pending state', async () => {
  server.use(createMetadataHandler(mockMetadata, []));

  server.use(
    http.get(cfg.api.sessionRecordingSummary, () =>
      HttpResponse.json({
        state: RecordingSummaryState.Pending,
        inferenceStartedAt: new Date(Date.now() - 90 * 1000).toISOString(),
      })
    )
  );

  setupTest(
    cfg.oss.getPlayerRoute(
      {
        clusterId: 'test-cluster',
        sid: 'test-session',
      },
      {
        recordingType: 'ssh',
        durationMs: 3600000,
      }
    ),
    true /* summarizerEnabled */
  );

  expect(
    await screen.findByText('Session summary is currently being generated')
  ).toBeInTheDocument();

  expect(
    screen.getByText('Started summarizing 2 minutes ago')
  ).toBeInTheDocument();
});

test('shows the error whilst generating state', async () => {
  server.use(createMetadataHandler(mockMetadata, []));

  server.use(
    withSummary({
      state: RecordingSummaryState.Error,
      errorMessage: 'Some error message',
      sessionId: 'test-session',
    })
  );

  setupTest(
    cfg.oss.getPlayerRoute(
      {
        clusterId: 'test-cluster',
        sid: 'test-session',
      },
      {
        recordingType: 'ssh',
        durationMs: 3600000,
      }
    ),
    true /* summarizerEnabled */
  );

  expect(
    await screen.findByText('There was an error generating the session summary')
  ).toBeInTheDocument();

  expect(screen.getByText('Some error message')).toBeInTheDocument();
});

test('falls back to showing no summary if fetching the summary fails', async () => {
  jest.spyOn(console, 'error').mockImplementation(() => {});

  server.use(createMetadataHandler(mockMetadata, []));

  server.use(withSummaryError('Failed to fetch'));

  setupTest(
    cfg.oss.getPlayerRoute(
      {
        clusterId: 'test-cluster',
        sid: 'test-session',
      },
      {
        recordingType: 'ssh',
        durationMs: 3600000,
      }
    ),
    true /* summarizerEnabled */
  );

  // When the summary fails to load, the page renders normally but without the summary section
  expect(await screen.findByText('test-server')).toBeInTheDocument();

  // The error message is not displayed - instead the summary section is simply not shown
  expect(
    screen.queryByText('Error loading session summary')
  ).not.toBeInTheDocument();
});

function withSummary(summary: SessionRecordingSummary) {
  return http.get(cfg.api.sessionRecordingSummary, () =>
    HttpResponse.json(summary)
  );
}

function withSummaryError(errorMessage: string) {
  return http.get(cfg.api.sessionRecordingSummary, () =>
    HttpResponse.json(
      {
        error: {
          message: errorMessage,
        },
      },
      {
        status: 500,
      }
    )
  );
}
