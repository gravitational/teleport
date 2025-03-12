/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { darkTheme } from 'design/theme';
import { fireEvent, render } from 'design/utils/testing';

import useRule from '../Validation/useRule';
import { FieldSelect, FieldSelectAsync } from './FieldSelect';

jest.mock('../Validation/useRule');
const mockedUseRule = jest.mocked(useRule);

test('valid values and onChange prop', () => {
  const onChange = jest.fn(e => e);
  const options = [
    { value: 'a', label: 'A' },
    { value: 'b', label: 'B' },
  ];

  mockedUseRule.mockReturnValue({ valid: true, message: '' });

  render(
    <FieldSelect
      label="labelText"
      placeholder="placeholderText"
      options={options}
      onChange={onChange}
      value={null}
    />
  );
  // test placeholder is rendered
  expect(screen.getByText('placeholderText')).toBeInTheDocument();

  // test onChange is respected
  const selectEl = screen.getByLabelText('labelText');
  fireEvent.focus(selectEl);
  fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
  fireEvent.click(screen.getByText('B'));
  expect(onChange).toHaveReturnedWith({ value: 'b', label: 'B' });
});

test('select element validation error state', () => {
  const rule = jest.fn();
  const errorColor = darkTheme.colors.error.main;

  mockedUseRule.mockReturnValue({ valid: false, message: 'errorMsg' });

  const { container } = render(
    <FieldSelect
      label="labelText"
      placeholder="placeholderText"
      rule={rule}
      onChange={jest.fn()}
      value={null}
      options={null}
    />
  );

  // test !valid values renders with error message
  const labelEl = screen.getByText('errorMsg');
  expect(labelEl).toHaveStyle({ color: errorColor });

  // test !valid values renders error colors
  // "react-select__control" defined by react-select library
  // eslint-disable-next-line testing-library/no-container, testing-library/no-node-access
  const selectEl = container.getElementsByClassName('react-select__control')[0];
  expect(selectEl).toHaveStyle({
    'border-color': errorColor,
  });
});

describe('FieldSelectAsync', () => {
  beforeEach(() => {
    mockedUseRule.mockReturnValue({ valid: true, message: '' });
  });

  it('loads options', async () => {
    const loadOptions = () =>
      Promise.resolve([
        { label: 'Apples', value: 'apples' },
        { label: 'Bananas', value: 'bananas' },
      ]);
    render(<FieldSelectAsync loadOptions={loadOptions} />);
    selectEvent.openMenu(screen.getByRole('combobox'));
    expect(await screen.findByRole('option', { name: 'Apples' })).toBeVisible();
    expect(
      await screen.findByRole('option', { name: 'Bananas' })
    ).toBeVisible();
  });

  it('supports empty option lists', async () => {
    const loadOptions = () => Promise.resolve([]);
    render(<FieldSelectAsync loadOptions={loadOptions} />);
    selectEvent.openMenu(screen.getByRole('combobox'));
    expect(await screen.findByText('No options')).toBeVisible();
  });

  it('supports void option lists', async () => {
    // We may never use this case, but react-select allows `loadOptions` to
    // return void, so we need to be prepared.
    const loadOptions = () => {};
    render(<FieldSelectAsync loadOptions={loadOptions} />);
    selectEvent.openMenu(screen.getByRole('combobox'));
    expect(await screen.findByText('No options')).toBeVisible();
  });

  it('displays no options message', async () => {
    const loadOptions = () => Promise.resolve([]);
    render(
      <FieldSelectAsync
        loadOptions={loadOptions}
        noOptionsMessage={() => 'This is sad'}
      />
    );
    selectEvent.openMenu(screen.getByRole('combobox'));
    expect(await screen.findByText('This is sad')).toBeVisible();
  });

  it('displays error message', async () => {
    const loadOptions = () => Promise.reject(new Error('oops'));
    render(<FieldSelectAsync loadOptions={loadOptions} />);
    selectEvent.openMenu(screen.getByRole('combobox'));
    expect(
      await screen.findByText('Could not load options: oops')
    ).toBeVisible();
  });
});
