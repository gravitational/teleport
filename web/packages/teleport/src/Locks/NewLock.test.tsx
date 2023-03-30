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
import { MemoryRouter } from 'react-router';

import { useGetTargetData } from './useGetTargetData';

import NewLock from './NewLock';
import { CreateLock } from './CreateLock';
import { USER_RESULT } from './testFixtures';

jest.mock('teleport/useStickyClusterId', () => ({
  __esModule: true,
  default: () => ({ clusterId: 'cluster-id' }),
}));

jest.mock('./CreateLock', () => ({
  CreateLock: jest.fn().mockReturnValue(<div></div>),
}));

jest.mock('./useGetTargetData', () => ({
  useGetTargetData: jest.fn(),
}));

describe('component: NewLock', () => {
  it('defaults to displaying the user targets', () => {
    const ugt = useGetTargetData as jest.Mock<any, any>;
    ugt.mockImplementation(() => USER_RESULT);
    render(
      <ThemeProvider>
        <MemoryRouter>
          <NewLock />
        </MemoryRouter>
      </ThemeProvider>
    );
    // eslint-disable-next-line testing-library/no-node-access
    expect(document.querySelectorAll('tbody tr')).toHaveLength(3);
  });

  // These tests can be completed once react-select has been updated
  // to enable testing with our current testing strategy.
  it.todo('allows you to switch to other targets');
  it.todo('supports displaying additional targets');
  describe('displays target type', () => {
    it.todo('role');
    it.todo('login');
    it.todo('node');
    it.todo('mfa device');
    it.todo('windows desktop');
  });
  it.todo('does not show a table for "logins"');
  it.todo('allows the collection of multiple target types');

  it('disables submit if there are no targets added', () => {
    const ugt = useGetTargetData as jest.Mock<any, any>;
    ugt.mockImplementation(() => USER_RESULT);
    render(
      <ThemeProvider>
        <MemoryRouter>
          <NewLock />
        </MemoryRouter>
      </ThemeProvider>
    );
    expect(screen.getByText('Lock targets added (0)')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Proceed to lock' })
    ).toBeDisabled();
  });

  it('passes selected targets to the CreateLock component', async () => {
    const ugt = useGetTargetData as jest.Mock<any, any>;
    const cl = CreateLock as jest.Mock<any, any>;
    ugt.mockImplementation(() => USER_RESULT);
    render(
      <ThemeProvider>
        <MemoryRouter>
          <NewLock />
        </MemoryRouter>
      </ThemeProvider>
    );
    expect(screen.getByText('Lock targets added (0)')).toBeInTheDocument();
    await userEvent.click(screen.getAllByRole('button', { name: '+ Add' })[1]);
    await screen.findByText('Lock targets added (1)');
    await userEvent.click(
      screen.getByRole('button', { name: 'Proceed to lock' })
    );
    expect(
      cl.mock.calls[cl.mock.calls.length - 1][0].selectedLockTargets
    ).toStrictEqual([
      {
        type: 'user',
        name: 'admin',
      },
    ]);
  });

  it('allows freeform target id inputs', async () => {
    const ugt = useGetTargetData as jest.Mock<any, any>;
    const cl = CreateLock as jest.Mock<any, any>;
    ugt.mockImplementation(() => USER_RESULT);
    render(
      <ThemeProvider>
        <MemoryRouter>
          <NewLock />
        </MemoryRouter>
      </ThemeProvider>
    );
    expect(screen.getByText('Lock targets added (0)')).toBeInTheDocument();
    await userEvent.type(
      screen.getByPlaceholderText('Quick add user'),
      'I am a uuid'
    );
    await userEvent.click(screen.getAllByRole('button', { name: '+ Add' })[0]);
    await screen.findByText('Lock targets added (1)');
    await userEvent.click(
      screen.getByRole('button', { name: 'Proceed to lock' })
    );
    expect(
      cl.mock.calls[cl.mock.calls.length - 1][0].selectedLockTargets
    ).toStrictEqual([
      {
        type: 'user',
        name: 'I am a uuid',
      },
    ]);
  });

  it.todo('allows freeform target id inputs depending on selected target type');
});
