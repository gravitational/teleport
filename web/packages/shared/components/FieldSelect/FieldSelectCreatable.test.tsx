/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { screen } from '@testing-library/react';
import selectEvent from 'react-select-event';

import { render } from 'design/utils/testing';

import useRule from '../Validation/useRule';
import { FieldSelectCreatableAsync } from './FieldSelectCreatable';

jest.mock('../Validation/useRule');
const mockedUseRule = jest.mocked(useRule);

describe('FieldSelectCreatableAsync', () => {
  beforeEach(() => {
    mockedUseRule.mockReturnValue({ valid: true, message: '' });
  });
  it('loads options', async () => {
    const loadOptions = () =>
      Promise.resolve([
        { label: 'Apples', value: 'apples' },
        { label: 'Bananas', value: 'bananas' },
      ]);
    render(
      <FieldSelectCreatableAsync loadOptions={loadOptions} defaultOptions />
    );
    selectEvent.openMenu(screen.getByRole('combobox'));
    expect(await screen.findByRole('option', { name: 'Apples' })).toBeVisible();
    expect(
      await screen.findByRole('option', { name: 'Bananas' })
    ).toBeVisible();
  });

  it('supports empty option lists', async () => {
    const loadOptions = () => Promise.resolve([]);
    render(
      <FieldSelectCreatableAsync loadOptions={loadOptions} defaultOptions />
    );
    selectEvent.openMenu(screen.getByRole('combobox'));
    expect(await screen.findByText('No options')).toBeVisible();
  });

  it('supports void option lists', async () => {
    // We may never use this case, but react-select allows `loadOptions` to
    // return void, so we need to be prepared.
    const loadOptions = () => {};
    render(
      <FieldSelectCreatableAsync loadOptions={loadOptions} defaultOptions />
    );
    selectEvent.openMenu(screen.getByRole('combobox'));
    expect(await screen.findByText('No options')).toBeVisible();
  });

  it('displays no options message', async () => {
    const loadOptions = () => Promise.resolve([]);
    render(
      <FieldSelectCreatableAsync
        loadOptions={loadOptions}
        defaultOptions
        noOptionsMessage={() => 'This is sad'}
      />
    );
    selectEvent.openMenu(screen.getByRole('combobox'));
    expect(await screen.findByText('This is sad')).toBeVisible();
  });

  it('displays error message', async () => {
    const loadOptions = () => Promise.reject(new Error('oops'));
    render(
      <FieldSelectCreatableAsync loadOptions={loadOptions} defaultOptions />
    );
    selectEvent.openMenu(screen.getByRole('combobox'));
    expect(
      await screen.findByText('Could not load options: Error: oops')
    ).toBeVisible();
  });
});
