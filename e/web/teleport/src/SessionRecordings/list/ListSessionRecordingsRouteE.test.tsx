import { http, HttpResponse } from 'msw';
import { generatePath, MemoryRouter } from 'react-router';

import {
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
} from 'design/utils/testing';

import cfg from 'e-teleport/config';
import { ListSessionRecordingsRouteE } from 'e-teleport/SessionRecordings/list/ListSessionRecordingsRouteE';
import { ContextProvider } from 'teleport';
import { createTeleportContext, fullAccess } from 'teleport/mocks/contexts';
import { eventCodes } from 'teleport/services/audit';
import type { SessionRecordingThumbnail } from 'teleport/services/recordings';
import type { Acl } from 'teleport/services/user';
import { makeAcl } from 'teleport/services/user/makeAcl';

enableMswServer();

let originalSessionSummarizerEnabled: boolean;
let originalSessionSummariesEntitlement: (typeof cfg.oss.entitlements)['SessionSummaries'];

beforeEach(() => {
  testQueryClient.clear();

  // backup original flag values
  originalSessionSummarizerEnabled = cfg.oss.sessionSummarizerEnabled;
  originalSessionSummariesEntitlement = cfg.oss.entitlements.SessionSummaries;

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
afterEach(() => {
  testQueryClient.clear();

  // restore original flag values
  cfg.oss.sessionSummarizerEnabled = originalSessionSummarizerEnabled;
  cfg.oss.entitlements.SessionSummaries = originalSessionSummariesEntitlement;
});

const listRecordingsUrl = generatePath(
  cfg.oss.api.clusterEventsRecordingsPath,
  {
    clusterId: 'localhost',
  }
);

function setupTest({
  summarizerEnabled,
  acl,
  sessionSummariesLicensed = false,
}: {
  summarizerEnabled: boolean;
  acl?: Partial<Acl>;
  sessionSummariesLicensed?: boolean;
}) {
  const ctx = createTeleportContext({
    customAcl: makeAcl(acl),
  });

  cfg.oss.sessionSummarizerEnabled = summarizerEnabled;
  cfg.oss.entitlements.SessionSummaries = {
    enabled: sessionSummariesLicensed,
    limit: 0,
  };

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

  setupTest({
    summarizerEnabled: false,
    acl: {
      inferencePolicy: fullAccess,
      inferenceSecret: fullAccess,
      inferenceModel: fullAccess,
    },
  });

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

  setupTest({ summarizerEnabled: true });

  await screen.findByText('server-01');

  const buttons = screen.getAllByRole('button', {
    name: 'View session summary',
  });

  expect(buttons).toHaveLength(3);
});

test('should show the CTA when Session Summaries is disabled', async () => {
  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json({
        events: MOCK_EVENTS,
      });
    })
  );

  setupTest({ summarizerEnabled: false });

  await screen.findByText('server-01');

  expect(
    screen.getByText('Summarize session recordings with AI')
  ).toBeInTheDocument();
});

test('should show the session summaries status as enabled when licensed and the feature is enabled', async () => {
  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json({
        events: MOCK_EVENTS,
      });
    })
  );

  setupTest({
    summarizerEnabled: true,
    acl: {
      accessGraph: fullAccess,
      inferencePolicy: fullAccess,
      inferenceSecret: fullAccess,
      inferenceModel: fullAccess,
    },
    sessionSummariesLicensed: true,
  });

  await screen.findByText('server-01');

  expect(screen.getByTestId('session-summaries-configure')).toHaveTextContent(
    'Configure Session Summaries'
  );
  expect(screen.getByTestId('session-summaries-configure')).toHaveTextContent(
    'Enabled'
  );
  expect(
    screen.getByTestId('session-summaries-configure')
  ).not.toHaveTextContent('Not Enabled');
});

test('should show a link to set up session summaries when licensed but the feature is disabled', async () => {
  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json({
        events: MOCK_EVENTS,
      });
    })
  );

  setupTest({
    summarizerEnabled: false,
    acl: {
      accessGraph: fullAccess,
      inferencePolicy: fullAccess,
      inferenceSecret: fullAccess,
      inferenceModel: fullAccess,
    },
    sessionSummariesLicensed: true,
  });

  await screen.findByText('server-01');

  expect(screen.getByTestId('session-summaries-configure')).toHaveTextContent(
    'Configure Session Summaries'
  );
  expect(screen.getByTestId('session-summaries-configure')).toHaveTextContent(
    'Not Enabled'
  );
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
