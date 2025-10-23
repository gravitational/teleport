import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { generatePath, MemoryRouter } from 'react-router';

import {
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import cfg from 'e-teleport/config';
import { ListSessionRecordingsRouteE } from 'e-teleport/SessionRecordings/list/ListSessionRecordingsRouteE';
import { ContextProvider } from 'teleport';
import { createTeleportContext, fullAccess } from 'teleport/mocks/contexts';
import { eventCodes } from 'teleport/services/audit';
import type { SessionRecordingThumbnail } from 'teleport/services/recordings';
import { storageService } from 'teleport/services/storageService';
import type { Acl } from 'teleport/services/user';
import { makeAcl } from 'teleport/services/user/makeAcl';

const server = setupServer();

beforeAll(() => server.listen());
beforeEach(() => {
  server.use(
    getThumbnail(MOCK_THUMBNAIL),
    http.get(cfg.oss.api.clustersPath, () => {
      return HttpResponse.json([
        {
          name: 'teleport',
          lastConnected: '2025-08-14T14:36:07.976470934Z',
          status: 'online',
          publicURL: '',
          authVersion: '',
          proxyVersion: '',
        },
      ]);
    })
  );
});
afterEach(async () => {
  server.resetHandlers();

  testQueryClient.clear();
});
afterAll(() => server.close());

const listRecordingsUrl = generatePath(
  cfg.oss.api.clusterEventsRecordingsPath,
  {
    clusterId: 'localhost',
  }
);

function setupTest(summarizerEnabled: boolean, acl?: Partial<Acl>) {
  const ctx = createTeleportContext({
    customAcl: makeAcl(acl),
  });

  cfg.oss.sessionSummarizerEnabled = summarizerEnabled;

  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ListSessionRecordingsRouteE />
      </ContextProvider>
    </MemoryRouter>
  );
}

test('should not show a view summary button when the feature is disabled', async () => {
  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json({
        events: MOCK_EVENTS,
      });
    })
  );

  setupTest(false);

  await screen.findByText('server-01');

  expect(screen.queryByTestId('view-summary-button')).not.toBeInTheDocument();
});

test('should show a view summary button when the feature is enabled', async () => {
  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json({
        events: MOCK_EVENTS,
      });
    })
  );

  setupTest(true);

  await screen.findByText('server-01');

  const buttons = screen.getAllByRole('button', {
    name: 'View session summary',
  });

  expect(buttons).toHaveLength(3);
});

test('should show the CTA when identity security is disabled', async () => {
  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json({
        events: MOCK_EVENTS,
      });
    })
  );

  setupTest(false);

  await screen.findByText('server-01');

  expect(
    screen.getByText('Summarize session recordings with AI')
  ).toBeInTheDocument();
});

test('should show the session summaries status as enabled when identity security and the feature are enabled', async () => {
  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json({
        events: MOCK_EVENTS,
      });
    })
  );

  jest.spyOn(storageService, 'getAccessGraphEnabled').mockReturnValue(true);

  setupTest(true, {
    accessGraph: fullAccess,
  });

  await screen.findByText('server-01');

  expect(screen.getByText('AI Session Summaries Enabled')).toBeInTheDocument();
});

test('should show a link to set up session summaries when identity security is enabled but the feature is disabled', async () => {
  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json({
        events: MOCK_EVENTS,
      });
    })
  );

  jest.spyOn(storageService, 'getAccessGraphEnabled').mockReturnValue(true);

  setupTest(false, {
    accessGraph: fullAccess,
  });

  await screen.findByText('server-01');

  expect(screen.getByText('Set up AI Session Summaries')).toBeInTheDocument();
});

test('session summaries setup link should be dismissible', async () => {
  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json({
        events: MOCK_EVENTS,
      });
    })
  );

  jest.spyOn(storageService, 'getAccessGraphEnabled').mockReturnValue(true);

  setupTest(false, {
    accessGraph: fullAccess,
  });

  await screen.findByText('server-01');

  const link = screen.getByText('Set up AI Session Summaries');

  expect(link).toBeInTheDocument();

  const dismissButton = screen.getByRole('button', { name: 'Dismiss' });

  expect(dismissButton).toBeInTheDocument();

  await userEvent.click(dismissButton);

  expect(link).not.toBeInTheDocument();
});

// TODO(ryan): use the mocks from the OSS tests once merged
const MOCK_THUMBNAIL: SessionRecordingThumbnail = {
  svg: '<svg><rect width="100" height="100" fill="blue"/></svg>',
  cursorX: 50,
  cursorY: 50,
  cursorVisible: true,
  cols: 100,
  rows: 100,
  startOffset: 0,
  endOffset: 0,
};

function getThumbnail(thumbnail: SessionRecordingThumbnail) {
  return http.get(cfg.oss.api.sessionRecording.thumbnail, () => {
    return HttpResponse.json(thumbnail);
  });
}

function createMockSessionEndEvent(overrides?: Record<string, string>) {
  return {
    'addr.remote': '100.119.121.67:55655',
    cluster_name: 'teleport',
    code: eventCodes.SESSION_END,
    ei: 41,
    enhanced_recording: false,
    event: 'session.end',
    interactive: true,
    login: 'root',
    namespace: 'default',
    participants: ['admin'],
    private_key_policy: 'none',
    proto: 'ssh',
    server_hostname: 'server-1',
    server_id: 'dc247eee-742d-445b-bb30-04bfc4604061',
    server_labels: {
      hostname: 'instance-1',
    },
    server_version: '19.0.0-dev',
    session_recording: 'node',
    session_start: '2025-08-13T12:42:23.099789612Z',
    session_stop: '2025-08-13T12:42:41.151765713Z',
    sid: 'ed794e43-57a3-461c-a4a6-1a375a5bbbba',
    time: '2025-08-13T12:42:41.152Z',
    uid: '4a0fdcfb-fa14-4446-9c06-52c18797fdbe',
    user: 'admin',
    user_kind: 1,
    ...overrides,
  };
}

const MOCK_EVENTS = [
  createMockSessionEndEvent({
    sid: 'session-001',
    user: 'alice',
    server_hostname: 'server-01',
    proto: 'ssh',
  }),
  createMockSessionEndEvent({
    sid: 'session-002',
    user: 'bob',
    desktop_name: 'desktop-02',
    code: eventCodes.DESKTOP_SESSION_ENDED,
    proto: 'desktop',
    time: '2025-01-15T11:00:00Z',
  }),
  createMockSessionEndEvent({
    sid: 'session-003',
    user: 'charlie',
    db_service: 'database-01',
    code: eventCodes.DATABASE_SESSION_ENDED,
    proto: 'database',
    time: '2025-01-15T12:00:00Z',
  }),
  createMockSessionEndEvent({
    sid: 'session-004',
    user: 'alice',
    kubernetes_cluster: 'k8s-cluster',
    proto: 'kube',
    time: '2025-01-15T13:00:00Z',
  }),
  createMockSessionEndEvent({
    sid: 'session-005',
    user: 'david',
    server_hostname: 'server-03',
    proto: 'ssh',
    time: '2025-01-15T14:00:00Z',
  }),
];
