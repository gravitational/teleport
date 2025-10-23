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

import type { Meta, StoryObj } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router';
import { Route } from 'react-router-dom';
import { mocked } from 'storybook/test';

import Box from 'design/Box';

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { fetchSessionRecordingMetadata } from 'teleport/services/recordings/metadata';
import { MOCK_THUMBNAILS } from 'teleport/SessionRecordings/list/mock';
import { MOCK_METADATA } from 'teleport/SessionRecordings/view/mock';
import { ViewSessionRecordingRoute } from 'teleport/SessionRecordings/view/ViewSessionRecordingRoute';

const meta = {
  title: 'Teleport/SessionRecordings',
} satisfies Meta<typeof ViewSessionRecordingRoute>;

export default meta;

export const ViewWithMetadata: StoryObj = {
  name: 'View with metadata',
  beforeEach: async () => {
    mocked(fetchSessionRecordingMetadata).mockReturnValue(
      Promise.resolve({ metadata: MOCK_METADATA, frames: MOCK_THUMBNAILS })
    );
  },
  parameters: {
    layout: 'fullscreen',
  },
  render: () =>
    render(
      '/web/cluster/teleport/session/session-id?recordingType=ssh&durationMs=20000'
    ),
};

export const ViewWithoutMetadata: StoryObj = {
  name: 'View without metadata (fallback to player)',
  beforeEach: async () => {
    mocked(fetchSessionRecordingMetadata).mockReturnValue(
      Promise.reject(new Error('Failed to fetch metadata'))
    );
  },
  parameters: {
    layout: 'fullscreen',
  },
  render: () =>
    render(
      '/web/cluster/teleport/session/session-id?recordingType=ssh&durationMs=20000'
    ),
};

function render(initialEntry: string) {
  const ctx = createTeleportContext();

  return (
    <MemoryRouter initialEntries={[initialEntry]}>
      <Route path={cfg.routes.player}>
        <ContextProvider ctx={ctx}>
          <Box height="100vh">
            <ViewSessionRecordingRoute />
          </Box>
        </ContextProvider>
      </Route>
    </MemoryRouter>
  );
}
