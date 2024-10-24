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

import userEvent from '@testing-library/user-event';
import React, { useState } from 'react';

import { render, screen } from 'design/utils/testing';

import { FieldMultiInput, FieldMultiInputProps } from './FieldMultiInput';

const TestFieldMultiInput = ({
  onChange,
  ...rest
}: Partial<FieldMultiInputProps>) => {
  const [items, setItems] = useState<string[]>([]);
  const handleChange = (it: string[]) => {
    setItems(it);
    onChange?.(it);
  };
  return <FieldMultiInput value={items} onChange={handleChange} {...rest} />;
};

test('adding, editing, and removing items', async () => {
  const user = userEvent.setup();
  const onChange = jest.fn();
  render(<TestFieldMultiInput onChange={onChange} />);

  await user.type(screen.getByRole('textbox'), 'apples');
  expect(onChange).toHaveBeenLastCalledWith(['apples']);

  await user.click(screen.getByRole('button', { name: 'Add More' }));
  expect(onChange).toHaveBeenLastCalledWith(['apples', '']);

  await user.type(screen.getAllByRole('textbox')[1], 'oranges');
  expect(onChange).toHaveBeenLastCalledWith(['apples', 'oranges']);

  await user.click(screen.getAllByRole('button', { name: 'Remove Item' })[0]);
  expect(onChange).toHaveBeenLastCalledWith(['oranges']);

  await user.click(screen.getAllByRole('button', { name: 'Remove Item' })[0]);
  expect(onChange).toHaveBeenLastCalledWith([]);
});

test('keyboard handling', async () => {
  const user = userEvent.setup();
  const onChange = jest.fn();
  render(<TestFieldMultiInput onChange={onChange} />);

  await user.click(screen.getByRole('textbox'));
  await user.keyboard('apples{Enter}oranges');
  expect(onChange).toHaveBeenLastCalledWith(['apples', 'oranges']);

  await user.click(screen.getAllByRole('textbox')[0]);
  await user.keyboard('{Enter}bananas');
  expect(onChange).toHaveBeenLastCalledWith(['apples', 'bananas', 'oranges']);
});
