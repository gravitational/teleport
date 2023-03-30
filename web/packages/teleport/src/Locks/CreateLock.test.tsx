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
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import '@testing-library/jest-dom';
import ThemeProvider from 'design/ThemeProvider';

import { CreateLock } from './CreateLock';
import { useLocks } from './useLocks';

import type { SelectedLockTarget } from './types';

jest.mock('teleport/useStickyClusterId', () => ({
  __esModule: true,
  default: () => ({ clusterId: 'cluster-id' }),
}));

const mockCreateLock = jest.fn(() => Promise.resolve());

jest.mock('./useLocks', () => ({
  useLocks: () => ({ createLock: mockCreateLock }),
}));

describe('component: CreateLock', () => {
  it('displays the list of targets to lock', () => {
    const selectedLockTargets: SelectedLockTarget[] = [
      { type: 'user', name: 'worker' },
      { type: 'role', name: 'contractor' },
    ];
    render(
      <ThemeProvider>
        <CreateLock
          panelPosition="open"
          setPanelPosition={() => {}}
          selectedLockTargets={selectedLockTargets}
          setSelectedLockTargets={() => {}}
        />
      </ThemeProvider>
    );
    // One of the rows is a header.
    expect(screen.getAllByRole('row')).toHaveLength(3);
  });

  it('can remove a target from the target list', async () => {
    const selectedLockTargets: SelectedLockTarget[] = [
      { type: 'user', name: 'worker' },
      { type: 'role', name: 'contractor' },
    ];
    const cb = jest.fn();
    render(
      <ThemeProvider>
        <CreateLock
          panelPosition="open"
          setPanelPosition={() => {}}
          selectedLockTargets={selectedLockTargets}
          setSelectedLockTargets={cb}
        />
      </ThemeProvider>
    );
    await userEvent.click(screen.getAllByTestId('trash-btn')[1]);
    expect(cb.mock.calls).toHaveLength(1);
    // The `contractor` role has been removed.
    expect(cb.mock.calls[0]).toEqual([[{ type: 'user', name: 'worker' }]]);
  });

  it('creates a lock with a message and ttl', async () => {
    const selectedLockTargets: SelectedLockTarget[] = [
      { type: 'user', name: 'worker' },
    ];
    render(
      <ThemeProvider>
        <CreateLock
          panelPosition="open"
          setPanelPosition={() => {}}
          selectedLockTargets={selectedLockTargets}
          setSelectedLockTargets={() => {}}
        />
      </ThemeProvider>
    );
    const createLock = useLocks('cluster-id').createLock as jest.Mock<any, any>;
    let testClusterId, testLockData;
    createLock.mockImplementation((clusterId, lockData) => {
      testClusterId = clusterId;
      testLockData = lockData;
      return Promise.resolve();
    });
    await userEvent.type(screen.getByTestId('description'), 'you were bad');
    await userEvent.type(screen.getByTestId('ttl'), '5h');
    await userEvent.click(screen.getByRole('button', { name: 'Create locks' }));
    expect(testClusterId).toBe('cluster-id');
    expect(testLockData).toStrictEqual({
      message: 'you were bad',
      targets: {
        user: 'worker',
      },
      ttl: '5h',
    });
  });

  it('displays errors when create fails', async () => {
    const selectedLockTargets: SelectedLockTarget[] = [
      { type: 'user', name: 'worker' },
    ];
    render(
      <ThemeProvider>
        <CreateLock
          panelPosition="open"
          setPanelPosition={() => {}}
          selectedLockTargets={selectedLockTargets}
          setSelectedLockTargets={() => {}}
        />
      </ThemeProvider>
    );
    const createLock = useLocks('cluster-id').createLock as jest.Mock<any, any>;
    // Reject the creation of the lock.
    createLock.mockImplementation(() =>
      Promise.reject({ message: 'unable to create lock' })
    );
    await userEvent.click(screen.getByRole('button', { name: 'Create locks' }));
    const alert = await screen.findByTestId('alert');
    expect(alert).toBeInTheDocument();
  });
});
