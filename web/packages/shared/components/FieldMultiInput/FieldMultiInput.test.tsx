import userEvent from '@testing-library/user-event';
import React, { useState } from 'react';
import { FieldMultiInput, FieldMultiInputProps } from './FieldMultiInput';
import { render, screen } from 'design/utils/testing';

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
