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

import { setupServer } from 'msw/node';

import { render, screen, testQueryClient } from 'design/utils/testing';

import { getThumbnail, MOCK_THUMBNAIL } from './mock';
import { RecordingThumbnail } from './RecordingThumbnail';

const server = setupServer();

beforeAll(() => server.listen());

afterEach(async () => {
  server.resetHandlers();
  testQueryClient.clear();
});

afterAll(() => server.close());

function setupTest(clusterId = 'localhost', sessionId = 'test-session') {
  return render(
    <RecordingThumbnail clusterId={clusterId} sessionId={sessionId} styles="" />
  );
}

test('renders thumbnail with correct background position for centered cursor', async () => {
  server.use(getThumbnail(MOCK_THUMBNAIL));
  setupTest();

  const thumbnail = await screen.findByTestId('recording-thumbnail');

  expect(thumbnail).toHaveStyle({
    backgroundPosition: '25% 25%',
    backgroundSize: '200%',
  });
});

test('renders thumbnail with correct background position for top-left cursor', async () => {
  server.use(
    getThumbnail({
      ...MOCK_THUMBNAIL,
      cursorX: 0,
      cursorY: 0,
    })
  );
  setupTest();

  const thumbnail = await screen.findByTestId('recording-thumbnail');

  expect(thumbnail).toHaveStyle({
    backgroundPosition: '0% 0%',
    backgroundSize: '200%',
  });
});

test('renders thumbnail with correct background position for bottom-right cursor', async () => {
  server.use(
    getThumbnail({
      ...MOCK_THUMBNAIL,
      cursorX: 100,
      cursorY: 100,
    })
  );
  setupTest();

  const thumbnail = await screen.findByTestId('recording-thumbnail');

  expect(thumbnail).toHaveStyle({
    backgroundPosition: '75% 75%',
    backgroundSize: '200%',
  });
});

test('renders thumbnail with different aspect ratio', async () => {
  server.use(
    getThumbnail({
      ...MOCK_THUMBNAIL,
      cols: 200,
      rows: 50,
      cursorX: 100,
      cursorY: 25,
    })
  );
  setupTest();

  const thumbnail = await screen.findByTestId('recording-thumbnail');

  expect(thumbnail).toHaveStyle({
    backgroundPosition: '25% 25%',
    backgroundSize: '200%',
  });
});

test('clamps background position when cursor is near edges', async () => {
  server.use(
    getThumbnail({
      ...MOCK_THUMBNAIL,
      cursorX: 95,
      cursorY: 5,
    })
  );
  setupTest();

  const thumbnail = await screen.findByTestId('recording-thumbnail');

  expect(thumbnail).toHaveStyle({
    backgroundPosition: '70% 0%',
    backgroundSize: '200%',
  });
});

test('centers the thumbnail when cursor is not visible', async () => {
  server.use(
    getThumbnail({
      ...MOCK_THUMBNAIL,
      cursorVisible: false,
    })
  );
  setupTest();

  const thumbnail = await screen.findByTestId('recording-thumbnail');

  expect(thumbnail).toHaveStyle({
    backgroundPosition: '50% 50%',
    backgroundSize: '200%',
  });
});
