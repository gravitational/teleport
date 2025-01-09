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

import { darkTheme } from 'design/theme';
import { fireEvent, render } from 'design/utils/testing';

import * as useRule from '../Validation/useRule';
import FieldInput from './FieldInput';

test('valid values, autofocus, onChange, onKeyPress', () => {
  const rule = jest.fn();
  const onChange = jest.fn();
  const onKeyPress = jest.fn();

  // mock positive validation
  jest.spyOn(useRule, 'default').mockReturnValue({ valid: true, message: '' });

  render(
    <FieldInput
      placeholder="placeholderText"
      autoFocus={true}
      label="labelText"
      helperText="helperText"
      rule={rule}
      onChange={onChange}
      onKeyPress={onKeyPress}
    />
  );

  // test label is displayed
  expect(screen.getByText('labelText')).toBeInTheDocument();

  // helper text is displayed
  expect(screen.getByText('helperText')).toBeInTheDocument();

  // test autofocus prop is respected
  const inputEl = screen.getByPlaceholderText('placeholderText');
  expect(inputEl).toHaveFocus();

  // test onChange prop is respected
  fireEvent.change(inputEl, { target: { value: 'test' } });
  expect(onChange).toHaveBeenCalledTimes(1);

  // test onKeyPress prop is respected
  fireEvent.keyPress(inputEl, { key: 'Enter', keyCode: 13 });
  expect(onKeyPress).toHaveBeenCalledTimes(1);
});

test('input validation error state', () => {
  const rule = jest.fn();
  const errorColor = darkTheme.colors.interactive.solid.danger.default;

  // mock negative validation
  jest
    .spyOn(useRule, 'default')
    .mockReturnValue({ valid: false, message: 'errorMsg' });

  render(
    <FieldInput
      placeholder="placeholderText"
      label="labelText"
      rule={rule}
      onChange={jest.fn()}
    />
  );

  // error message is attached to the input
  const inputEl = screen.getByPlaceholderText('placeholderText');
  expect(screen.getByRole('textbox', { description: 'errorMsg' })).toBe(
    inputEl
  );

  // test !valid values renders with error message
  const labelEl = screen.getByText('errorMsg');
  expect(labelEl).toHaveStyle({ color: errorColor });

  // test !valid values renders error colors
  expect(inputEl).toHaveStyle({
    'border-color': errorColor,
  });
});
