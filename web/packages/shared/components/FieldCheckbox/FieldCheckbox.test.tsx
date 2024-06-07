import { render, screen, userEvent } from 'design/utils/testing';
import React, { useRef, useState } from 'react';

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
