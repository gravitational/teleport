import type { StoryObj } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router';

import Box from 'design/Box';

import cfg from 'e-teleport/config';
import { ListSessionRecordingsRouteE } from 'e-teleport/SessionRecordings/list/ListSessionRecordingsRouteE';
import {
  withPendingSummary,
  withSuccessSummary,
  withSummaryWithGenerationError,
} from 'e-teleport/SessionRecordings/mock';
import { ContextProvider } from 'teleport';
import {
  createTeleportContext,
  fullAccess,
  noAccess,
} from 'teleport/mocks/contexts';
import { KeysEnum } from 'teleport/services/storageService';
import type { Acl } from 'teleport/services/user';
import { makeAcl } from 'teleport/services/user/makeAcl';
import {
  withMockCluster,
  withMockEvents,
  withMockThumbnails,
} from 'teleport/SessionRecordings/mock';

export default {
  title: 'TeleportE/SessionRecordings',
};

export const ListSummariesEnabled: StoryObj = {
  name: 'List with session summaries enabled',
  beforeEach({ msw }) {
    msw.use(
      withMockCluster(),
      withMockEvents(),
      withMockThumbnails(),
      withSuccessSummary('session-001', 'This is a summary of session 001.'),
      withSummaryWithGenerationError('session-004'),
      withPendingSummary('session-005')
    );

    cfg.oss.sessionSummarizerEnabled = true;

    localStorage.setItem(KeysEnum.ACCESS_GRAPH_ENABLED, 'true');

    return () => {
      cfg.oss.sessionSummarizerEnabled = false;

      localStorage.removeItem(KeysEnum.ACCESS_GRAPH_ENABLED);
    };
  },
  parameters: {
    layout: 'fullscreen',
  },
  render: () =>
    render({
      accessGraph: fullAccess,
    }),
};

export const ListSessionSummariesDisabled: StoryObj = {
  name: 'List with session summaries disabled',
  beforeEach({ msw }) {
    msw.use(withMockCluster(), withMockEvents(), withMockThumbnails());

    localStorage.setItem(KeysEnum.ACCESS_GRAPH_ENABLED, 'true');

    return () => {
      localStorage.removeItem(KeysEnum.ACCESS_GRAPH_ENABLED);
    };
  },
  parameters: {
    layout: 'fullscreen',
  },
  render: () =>
    render({
      accessGraph: fullAccess,
    }),
};

export const ListSessionSummariesNoAccessToSetup: StoryObj = {
  name: 'List with session summaries disabled and no access to setup',
  beforeEach({ msw }) {
    msw.use(withMockCluster(), withMockEvents(), withMockThumbnails());

    localStorage.setItem(KeysEnum.ACCESS_GRAPH_ENABLED, 'true');

    return () => {
      localStorage.removeItem(KeysEnum.ACCESS_GRAPH_ENABLED);
    };
  },
  parameters: {
    layout: 'fullscreen',
  },
  render: () =>
    render({
      accessGraph: noAccess,
    }),
};

function render(acl?: Partial<Acl>) {
  const ctx = createTeleportContext({
    customAcl: makeAcl(acl),
  });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Box height="100vh">
          <ListSessionRecordingsRouteE />
        </Box>
      </ContextProvider>
    </MemoryRouter>
  );
}
