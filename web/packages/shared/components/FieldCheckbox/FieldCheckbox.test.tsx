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

import React, { useRef, useState } from 'react';

import { render, screen, userEvent } from 'design/utils/testing';

import { FieldCheckbox } from './FieldCheckbox';

test('controlled flow', async () => {
  const onChange = jest.fn();

  function TestField() {
    const [checked, setChecked] = useState(false);
    function onCbChange(e: React.ChangeEvent<HTMLInputElement>) {
      const c = e.currentTarget.checked;
      setChecked(c);
      onChange(c);
    }
    return (
      <FieldCheckbox label="I agree" checked={checked} onChange={onCbChange} />
    );
  }

  const user = userEvent.setup();
  render(<TestField />);

  await user.click(screen.getByLabelText('I agree'));
  expect(screen.getByLabelText('I agree')).toBeChecked();
  expect(onChange).toHaveBeenLastCalledWith(true);

  await user.click(screen.getByLabelText('I agree'));
  expect(screen.getByLabelText('I agree')).not.toBeChecked();
  expect(onChange).toHaveBeenLastCalledWith(false);
});

test('uncontrolled flow', async () => {
  let checkboxRef;
  function TestForm() {
    const cbRefInternal = useRef();
    checkboxRef = cbRefInternal;
    return (
      <form data-testid="form">
        <FieldCheckbox ref={cbRefInternal} name="ack" label="Make it so" />
      </form>
    );
  }

  const user = userEvent.setup();
  render(<TestForm />);
  expect(screen.getByTestId('form')).toHaveFormValues({});

  await user.click(screen.getByLabelText('Make it so'));
  expect(screen.getByTestId('form')).toHaveFormValues({ ack: true });
  expect(checkboxRef.current.checked).toBe(true);

  await user.click(screen.getByLabelText('Make it so'));
  expect(screen.getByTestId('form')).toHaveFormValues({});
  expect(checkboxRef.current.checked).toBe(false);
});
