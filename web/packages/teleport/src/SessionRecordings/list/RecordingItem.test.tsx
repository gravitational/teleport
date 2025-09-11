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

import { format } from 'date-fns';
import { setupServer } from 'msw/node';
import { act } from 'react';
import { MemoryRouter } from 'react-router';

import { render, screen, testQueryClient, tick } from 'design/utils/testing';

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import type { Recording } from 'teleport/services/recordings';

import { getThumbnail, MOCK_THUMBNAIL, thumbnailError } from './mock';
import { RecordingItem, type RecordingItemProps } from './RecordingItem';
import { Density, ViewMode } from './ViewSwitcher';

const mockRecording: Recording = {
  sid: 'test-session',
  user: 'alice',
  hostname: 'server-01',
  duration: 125000, // 2m 5s
  createdDate: new Date('2025-01-15T10:30:00Z'),
  recordingType: 'ssh',
  playable: true,
  description: '',
  durationText: '',
  users: 'alice',
};

async function setupTest({
  recording = mockRecording,
  viewMode = ViewMode.List,
  density = Density.Comfortable,
  actionSlot,
}: Partial<RecordingItemProps>) {
  const ctx = createTeleportContext();

  const view = render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <RecordingItem
          actionSlot={actionSlot}
          recording={recording}
          viewMode={viewMode}
          density={density}
          thumbnailStyles=""
        />
      </ContextProvider>
    </MemoryRouter>
  );

  // Wait for thumbnail to load
  await act(tick);

  return view;
}

const server = setupServer();

beforeAll(() => server.listen());

beforeEach(() => {
  server.use(getThumbnail(MOCK_THUMBNAIL));
});

afterEach(async () => {
  server.resetHandlers();
  testQueryClient.clear();
});

afterAll(() => server.close());

test('renders recording item with basic information', async () => {
  await setupTest({});

  expect(screen.getByTestId('recording-item')).toBeInTheDocument();
  expect(screen.getByText('SSH Session')).toBeInTheDocument();
  expect(screen.getByText('alice')).toBeInTheDocument();
  expect(screen.getByText('server-01')).toBeInTheDocument();
  expect(screen.getByText('test-session')).toBeInTheDocument();
  expect(screen.getByText('2m 5s')).toBeInTheDocument();
  expect(
    screen.getByText(format(mockRecording.createdDate, 'MMM dd, yyyy HH:mm'))
  ).toBeInTheDocument();
});

test('renders with compact density', async () => {
  await setupTest({
    recording: mockRecording,
    viewMode: ViewMode.List,
    density: Density.Compact,
  });

  expect(screen.getByTestId('recording-item')).toBeInTheDocument();
  expect(screen.getByText('SSH Session')).toBeInTheDocument();
});

test('renders with comfortable density', async () => {
  await setupTest({
    recording: mockRecording,
    viewMode: ViewMode.List,
    density: Density.Comfortable,
  });

  expect(screen.getByTestId('recording-item')).toBeInTheDocument();
  expect(screen.getByText('SSH Session')).toBeInTheDocument();
});

test('renders SSH recording type correctly', async () => {
  await setupTest({ recording: { ...mockRecording, recordingType: 'ssh' } });

  expect(screen.getByText('SSH Session')).toBeInTheDocument();
});

test('renders Desktop recording type correctly', async () => {
  await setupTest({
    recording: { ...mockRecording, recordingType: 'desktop' },
  });

  expect(screen.getByText('Desktop Session')).toBeInTheDocument();
});

test('renders Database recording type correctly', async () => {
  await setupTest({
    recording: { ...mockRecording, recordingType: 'database' },
  });

  expect(screen.getByText('Database Session')).toBeInTheDocument();
});

test('renders Kubernetes recording type correctly', async () => {
  await setupTest({ recording: { ...mockRecording, recordingType: 'k8s' } });

  expect(screen.getByText('Kubernetes Session')).toBeInTheDocument();
});

test('formats duration correctly for seconds only', async () => {
  await setupTest({ recording: { ...mockRecording, duration: 45000 } }); // 45s

  expect(screen.getByText('45s')).toBeInTheDocument();
});

test('formats duration correctly for minutes and seconds', async () => {
  await setupTest({ recording: { ...mockRecording, duration: 125000 } }); // 2m 5s

  expect(screen.getByText('2m 5s')).toBeInTheDocument();
});

test('formats duration correctly for hours', async () => {
  await setupTest({ recording: { ...mockRecording, duration: 7265000 } }); // 2h 1m 5s

  expect(screen.getByText('2h 1m 5s')).toBeInTheDocument();
});

test('formats duration correctly for days', async () => {
  await setupTest({ recording: { ...mockRecording, duration: 93665000 } }); // 1d 2h 1m 5s

  expect(screen.getByText('1d 2h 1m 5s')).toBeInTheDocument();
});

test('formats duration as 0s for zero duration', async () => {
  await setupTest({ recording: { ...mockRecording, duration: 0 } });

  expect(screen.getByText('0s')).toBeInTheDocument();
});

test('generates correct link URL', async () => {
  await setupTest({});

  const link = screen.getByTestId('recording-item');
  const expectedUrl = cfg.getPlayerRoute(
    { clusterId: 'localhost', sid: mockRecording.sid },
    {
      recordingType: mockRecording.recordingType,
      durationMs: mockRecording.duration,
    }
  );

  expect(link).toHaveAttribute('href', expectedUrl);
  expect(link).toHaveAttribute('target', '_blank');
});

test('renders all view modes with all densities', async () => {
  const viewModes = [ViewMode.List, ViewMode.Card];
  const densities = [Density.Compact, Density.Comfortable];

  for (const viewMode of viewModes) {
    for (const density of densities) {
      const { unmount } = await setupTest({
        recording: mockRecording,
        viewMode,
        density,
      });

      expect(screen.getByTestId('recording-item')).toBeInTheDocument();
      expect(screen.getByText('SSH Session')).toBeInTheDocument();
      expect(screen.getByText('alice')).toBeInTheDocument();

      unmount();
    }
  }
});

test('shows thumbnail not available if it fails to load', async () => {
  jest.spyOn(console, 'error').mockImplementation();

  server.use(thumbnailError());

  await setupTest({});

  expect(
    await screen.findByText('Thumbnail not available')
  ).toBeInTheDocument();
  expect(screen.queryByTestId('recording-thumbnail')).not.toBeInTheDocument();
});

test('shows non-interactive session message for non-playable recordings', async () => {
  const nonPlayableRecording: Recording = {
    ...mockRecording,
    playable: false,
    recordingType: 'desktop',
  };

  await setupTest({ recording: nonPlayableRecording });

  expect(
    screen.getByText('Non-interactive session, no playback available')
  ).toBeInTheDocument();
  expect(screen.queryByTestId('recording-thumbnail')).not.toBeInTheDocument();
});

test('allows custom action slot', async () => {
  const actionSlot = jest
    .fn()
    .mockReturnValue(
      <button data-testid="custom-action">Custom Action</button>
    );

  await setupTest({
    recording: mockRecording,
    viewMode: ViewMode.List,
    density: Density.Comfortable,
    actionSlot,
  });

  const item = screen.getByTestId('recording-item');
  expect(item).toBeInTheDocument();

  const customAction = screen.getByTestId('custom-action');

  expect(customAction).toBeInTheDocument();
  expect(actionSlot).toHaveBeenCalledWith(mockRecording.sid, 'ssh');
  expect(customAction).toHaveTextContent('Custom Action');
});
