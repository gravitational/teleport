/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { screen } from '@testing-library/react';

import { render, fireEvent } from 'design/utils/testing';

import { darkTheme } from 'design/theme';

import * as useRule from '../Validation/useRule';

import FieldInput from './FieldInput';
import { Fields } from './FieldInput.story';

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
      rule={rule}
      onChange={onChange}
      onKeyPress={onKeyPress}
    />
  );

  // test label is displayed
  expect(screen.getByText('labelText')).toBeInTheDocument();

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
  const errorColor = darkTheme.colors.error.main;

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

  // test !valid values renders with error message
  const labelEl = screen.getByText('errorMsg');
  expect(labelEl).toHaveStyle({ color: errorColor });

  // test !valid values renders error colors
  const inputEl = screen.getByPlaceholderText('placeholderText');
  expect(inputEl).toHaveStyle({
    'border-color': errorColor,
  });
});

test('snapshot tests', () => {
  const { container } = render(<Fields />);
  expect(container).toMatchSnapshot();
});
