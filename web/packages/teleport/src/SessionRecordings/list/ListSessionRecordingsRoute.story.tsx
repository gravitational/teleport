import type { StoryObj } from '@storybook/react-vite';
import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import Box from 'design/Box';

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ListSessionRecordingsRoute } from 'teleport/SessionRecordings/list/ListSessionRecordingsRoute';
import {
  withMockCluster,
  withMockEvents,
  withMockThumbnails,
} from 'teleport/SessionRecordings/mock';

export default {
  title: 'Teleport/SessionRecordings',
};

export const List: StoryObj = {
  parameters: {
    layout: 'fullscreen',
    msw: {
      handlers: [withMockCluster(), withMockEvents(), withMockThumbnails()],
    },
  },
  render,
};

export const ListError: StoryObj = {
  name: 'List with error fetching recordings',
  parameters: {
    layout: 'fullscreen',
    msw: {
      handlers: [
        withMockCluster(),
        http.get(cfg.api.clusterEventsRecordingsPath, () => {
          return HttpResponse.json(
            {
              error: {
                message: 'Internal Server Error',
              },
            },
            { status: 500 }
          );
        }),
      ],
    },
  },
  render,
};

export const ListClusterError: StoryObj = {
  name: 'List with error fetching clusters',
  parameters: {
    layout: 'fullscreen',
    msw: {
      handlers: [
        http.get(cfg.api.clustersPath, () => {
          return HttpResponse.json(
            {
              error: {
                message: 'Failed to fetch clusters',
              },
            },
            { status: 500 }
          );
        }),
        withMockEvents(),
      ],
    },
  },
  render,
};

export const ListLoading: StoryObj = {
  name: 'List loading state',
  parameters: {
    layout: 'fullscreen',
    msw: {
      handlers: [
        withMockCluster(),
        http.get(cfg.api.clusterEventsRecordingsPath, () => delay('infinite')),
      ],
    },
  },
  render,
};

function render() {
  const ctx = createTeleportContext();

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Box height="100vh">
          <ListSessionRecordingsRoute />
        </Box>
      </ContextProvider>
    </MemoryRouter>
  );
}
