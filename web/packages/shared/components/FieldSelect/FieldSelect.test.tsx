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

import FieldSelect from './FieldSelect';

test('valid values and onChange prop', () => {
  const rule = jest.fn();
  const onChange = jest.fn(e => e);
  const options = [
    { value: 'a', label: 'A' },
    { value: 'b', label: 'B' },
  ];

  jest.spyOn(useRule, 'default').mockReturnValue({ valid: true, message: '' });

  render(
    <FieldSelect
      label="labelText"
      placeholder="placeholderText"
      options={options}
      rule={rule}
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

  jest
    .spyOn(useRule, 'default')
    .mockReturnValue({ valid: false, message: 'errorMsg' });

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
