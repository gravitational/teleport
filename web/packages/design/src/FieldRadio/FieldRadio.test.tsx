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

import { ChangeEvent, createRef, useState } from 'react';

import { render, screen, userEvent } from 'design/utils/testing';

import { FieldRadio } from './FieldRadio';

test('controlled flow', async () => {
  const onChange = jest.fn();

  function TestField() {
    const [val, setVal] = useState('');
    function onRbChange(e: ChangeEvent<HTMLInputElement>) {
      const v = e.currentTarget.value;
      setVal(v);
      onChange(v);
    }
    return (
      <>
        <FieldRadio
          label="Foo"
          value="foo"
          name="val"
          checked={val === 'foo'}
          onChange={onRbChange}
        />
        <FieldRadio
          label="Bar"
          value="bar"
          name="val"
          checked={val === 'bar'}
          onChange={onRbChange}
        />
      </>
    );
  }

  const user = userEvent.setup();
  render(<TestField />);

  await user.click(screen.getByLabelText('Foo'));
  expect(screen.getByLabelText('Foo')).toBeChecked();
  expect(screen.getByLabelText('Bar')).not.toBeChecked();
  expect(onChange).toHaveBeenLastCalledWith('foo');

  await user.click(screen.getByLabelText('Bar'));
  expect(screen.getByLabelText('Foo')).not.toBeChecked();
  expect(screen.getByLabelText('Bar')).toBeChecked();
  expect(onChange).toHaveBeenLastCalledWith('bar');
});

test('uncontrolled flow', async () => {
  const fooRef = createRef<HTMLInputElement>();
  const barRef = createRef<HTMLInputElement>();

  function TestForm() {
    return (
      <form data-testid="form">
        <FieldRadio ref={fooRef} name="val" value="foo" label="Foo" />
        <FieldRadio ref={barRef} name="val" value="bar" label="Bar" />
      </form>
    );
  }

  const user = userEvent.setup();
  render(<TestForm />);
  expect(screen.getByTestId('form')).toHaveFormValues({});

  await user.click(screen.getByLabelText('Foo'));
  expect(screen.getByTestId('form')).toHaveFormValues({ val: 'foo' });
  expect(fooRef.current?.checked).toBe(true);
  expect(barRef.current?.checked).toBe(false);

  await user.click(screen.getByLabelText('Bar'));
  expect(screen.getByTestId('form')).toHaveFormValues({ val: 'bar' });
  expect(fooRef.current?.checked).toBe(false);
  expect(barRef.current?.checked).toBe(true);
});
