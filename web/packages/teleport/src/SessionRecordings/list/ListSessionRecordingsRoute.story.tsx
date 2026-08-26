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
  beforeEach({ msw }) {
    msw.use(withMockCluster(), withMockEvents(), withMockThumbnails());
  },

  parameters: {
    layout: 'fullscreen',
  },

  render,
};

export const ListError: StoryObj = {
  name: 'List with error fetching recordings',

  beforeEach({ msw }) {
    msw.use(
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
      })
    );
  },

  parameters: {
    layout: 'fullscreen',
  },

  render,
};

export const ListClusterError: StoryObj = {
  name: 'List with error fetching clusters',

  beforeEach({ msw }) {
    msw.use(
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
      withMockEvents()
    );
  },

  parameters: {
    layout: 'fullscreen',
  },

  render,
};

export const ListLoading: StoryObj = {
  name: 'List loading state',

  beforeEach({ msw }) {
    msw.use(
      withMockCluster(),
      http.get(cfg.api.clusterEventsRecordingsPath, () => delay('infinite'))
    );
  },

  parameters: {
    layout: 'fullscreen',
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
