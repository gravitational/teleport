import type { Meta, StoryObj } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router';
import { Route } from 'react-router-dom';
import { mocked } from 'storybook/test';

import Box from 'design/Box';

import cfg from 'e-teleport/config';
import {
  withPendingSummary,
  withSuccessSummary,
  withSummaryWithGenerationError,
} from 'e-teleport/SessionRecordings/mock';
import { ViewSessionRecordingRouteE } from 'e-teleport/SessionRecordings/view/ViewSessionRecordingRouteE';
import { ContextProvider } from 'teleport';
import { createTeleportContext, noAccess } from 'teleport/mocks/contexts';
import { fetchSessionRecordingMetadata } from 'teleport/services/recordings/metadata';
import { KeysEnum } from 'teleport/services/storageService';
import type { Acl } from 'teleport/services/user';
import { makeAcl } from 'teleport/services/user/makeAcl';
import { MOCK_THUMBNAILS } from 'teleport/SessionRecordings/list/mock';
import {
  withMockCluster,
  withMockEvents,
  withMockThumbnails,
} from 'teleport/SessionRecordings/mock';
import { MOCK_METADATA } from 'teleport/SessionRecordings/view/mock';

const meta = {
  title: 'TeleportE/SessionRecordings',
  beforeEach() {
    mocked(fetchSessionRecordingMetadata).mockReturnValue(
      Promise.resolve({ metadata: MOCK_METADATA, frames: MOCK_THUMBNAILS })
    );

    cfg.oss.sessionSummarizerEnabled = true;

    localStorage.setItem(KeysEnum.ACCESS_GRAPH_ENABLED, 'true');

    return () => {
      cfg.oss.sessionSummarizerEnabled = false;

      localStorage.removeItem(KeysEnum.ACCESS_GRAPH_ENABLED);
    };
  },
} satisfies Meta<typeof ViewSessionRecordingRouteE>;

export default meta;

export const ViewWithSessionSummary: StoryObj = {
  name: 'View with session summary available',
  parameters: {
    layout: 'fullscreen',
    msw: {
      handlers: [
        withMockCluster(),
        withMockEvents(),
        withMockThumbnails(),
        withSuccessSummary('session-001', 'This is a summary of session 001.'),
      ],
    },
  },
  render: () =>
    render(
      '/web/cluster/teleport/session/session-001?recordingType=ssh&durationMs=20000',
      {
        accessGraph: noAccess,
      }
    ),
};

export const ViewWithSessionSummaryPending: StoryObj = {
  name: 'View with session summary generation pending',

  parameters: {
    layout: 'fullscreen',
    msw: {
      handlers: [
        withMockCluster(),
        withMockEvents(),
        withMockThumbnails(),
        withPendingSummary('session-001'),
      ],
    },
  },
  render: () =>
    render(
      '/web/cluster/teleport/session/session-001?recordingType=ssh&durationMs=20000',
      {
        accessGraph: noAccess,
      }
    ),
};

export const ViewWithSessionSummaryGenerationError: StoryObj = {
  name: 'View with session summary generation error',

  parameters: {
    layout: 'fullscreen',
    msw: {
      handlers: [
        withMockCluster(),
        withMockEvents(),
        withMockThumbnails(),
        withSummaryWithGenerationError('session-001'),
      ],
    },
  },
  render: () =>
    render(
      '/web/cluster/teleport/session/session-001?recordingType=ssh&durationMs=20000',
      {
        accessGraph: noAccess,
      }
    ),
};

function render(initialEntry: string, acl?: Partial<Acl>) {
  const ctx = createTeleportContext({
    customAcl: makeAcl(acl),
  });

  return (
    <MemoryRouter initialEntries={[initialEntry]}>
      <Route path={cfg.oss.routes.player}>
        <ContextProvider ctx={ctx}>
          <Box height="100vh">
            <ViewSessionRecordingRouteE />
          </Box>
        </ContextProvider>
      </Route>
    </MemoryRouter>
  );
}
