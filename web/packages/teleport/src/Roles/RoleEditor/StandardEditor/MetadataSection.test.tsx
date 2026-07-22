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

import userEvent from '@testing-library/user-event';
import selectEvent from 'react-select-event';

import { act, render, screen } from 'design/utils/testing';
import { Validator } from 'shared/components/Validation';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { ApiError } from 'teleport/services/api/parseError';
import ResourceService from 'teleport/services/resources';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { MetadataSection } from './MetadataSection';
import { MetadataModel } from './standardmodel';
import { StatefulSectionWithDispatch } from './StatefulSection';
import { StandardModelDispatcher } from './useStandardModel';
import { MetadataValidationResult } from './validation';

beforeEach(() => {
  jest
    .spyOn(ResourceService.prototype, 'fetchRole')
    .mockImplementation(async (name: string) => {
      if (name === 'existing-role') {
        return {
          kind: 'role',
          id: '',
          name,
          content: '',
        };
      }
      throw new ApiError({
        message: `role ${name} is not found`,
        response: { status: 404 } as Response,
      });
    });
});

afterEach(() => {
  jest.restoreAllMocks();
});

const setup = () => {
  const modelRef = jest.fn();
  const ctx = createTeleportContext();
  let validator: Validator;
  let dispatch: StandardModelDispatcher;
  render(
    <TeleportContextProvider ctx={ctx}>
      <StatefulSectionWithDispatch<MetadataModel, MetadataValidationResult>
        selector={m => m.roleModel.metadata}
        validationSelector={m => m.validationResult.metadata}
        component={MetadataSection}
        validatorRef={v => {
          validator = v;
        }}
        modelRef={modelRef}
        dispatchRef={d => {
          dispatch = d;
        }}
      />
    </TeleportContextProvider>
  );
  return { modelRef, dispatch, validator };
};

test('basic editing', async () => {
  const user = userEvent.setup();
  const { modelRef } = setup();
  await user.clear(screen.getByLabelText('Role Name *'));
  await user.type(screen.getByLabelText('Role Name *'), 'some-name');
  await user.type(screen.getByLabelText('Description'), 'some-description');
  await user.type(screen.getByPlaceholderText('label key'), 'foo');
  await user.type(screen.getByPlaceholderText('label value'), 'bar');
  await selectEvent.select(screen.getByLabelText('Version'), 'v6');
  expect(modelRef).toHaveBeenLastCalledWith({
    name: 'some-name',
    nameCollision: false,
    description: 'some-description',
    labels: [{ name: 'foo', value: 'bar' }],
    version: { label: 'v6', value: 'v6' },
  } as MetadataModel);
});

test('basic validation', async () => {
  const user = userEvent.setup();
  const { validator } = setup();
  await user.clear(screen.getByLabelText('Role Name *'));
  await user.type(screen.getByPlaceholderText('label value'), 'some-value');
  act(() => validator.validate());

  expect(screen.getByLabelText('Role Name *')).toHaveAccessibleDescription(
    'Role name is required'
  );
  expect(screen.getByPlaceholderText('label key')).toHaveAccessibleDescription(
    'required'
  );
});

// We are testing debounced logic and we don't want the test to wait until the
// timer fires, so we are using fake timers. Because of that, we wrap the test
// with a custom pair of before/after routines.
describe('asynchronous validation', () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.runOnlyPendingTimers();
    jest.useRealTimers();
  });

  test('checking for existing roles', async () => {
    // Required for userEvent to cooperate nicely with fake timers.
    const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
    const { validator } = setup();
    await user.clear(screen.getByLabelText('Role Name *'));
    await user.type(screen.getByLabelText('Role Name *'), 'existing-role');

    await act(async () => {
      validator.validate();
      // Wait until the fetch is debounced and resolved.
      await jest.runAllTimersAsync();
    });
    expect(screen.getByLabelText('Role Name *')).toHaveAccessibleDescription(
      'Role with this name already exists'
    );

    await user.clear(screen.getByLabelText('Role Name *'));
    await user.type(screen.getByLabelText('Role Name *'), 'foo');
    await act(async () => {
      // Wait until the fetch is debounced and resolved.
      await jest.runAllTimersAsync();
    });
    expect(screen.getByLabelText('Role Name *')).toHaveAccessibleDescription(
      ''
    );
  });
});
