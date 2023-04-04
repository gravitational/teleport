/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import '@testing-library/jest-dom';
import ThemeProvider from 'design/ThemeProvider';
import { MemoryRouter } from 'react-router';

import api from 'teleport/services/api';

import { Locks } from './Locks';
import { HOOK_LIST as mockHookList } from './testFixtures';

jest.mock('teleport/useStickyClusterId', () => ({
  __esModule: true,
  default: () => ({ clusterId: 'cluster-id' }),
}));

jest.mock('teleport/services/api', () => ({
  get: () => new Promise(resolve => resolve(mockHookList)),
  delete: jest.fn(() => new Promise(() => Promise.resolve())),
}));

describe('component: Locks', () => {
  it('displays the fetched locks', async () => {
    render(
      <ThemeProvider>
        <MemoryRouter>
          <Locks />
        </MemoryRouter>
      </ThemeProvider>
    );
    await waitFor(() => expect(screen.getAllByRole('row')).toHaveLength(5));
  });

  it('can call to remove a lock', async () => {
    render(
      <ThemeProvider>
        <MemoryRouter>
          <Locks />
        </MemoryRouter>
      </ThemeProvider>
    );
    const deleteLock = api.delete as jest.Mock<any, any>;
    const mockDeleteLock = jest.fn(() => Promise.resolve());
    deleteLock.mockImplementation(mockDeleteLock);
    await waitFor(() => expect(screen.getAllByRole('row')).toHaveLength(5));
    await userEvent.click(screen.getAllByTestId('trash-btn')[1]);
    await waitFor(() => expect(mockDeleteLock.mock.calls).toHaveLength(1));

    expect(mockDeleteLock.mock.calls).toEqual([
      [
        '/v1/webapi/sites/cluster-id/locks/60626e99-e91b-41b2-89fe-bf5d16b0c622',
      ],
    ]);
  });
});
