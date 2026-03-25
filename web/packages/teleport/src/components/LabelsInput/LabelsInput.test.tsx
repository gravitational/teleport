/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { act } from '@testing-library/react';
import { useState } from 'react';

import { fireEvent, render, screen, userEvent } from 'design/utils/testing';
import Validation, { Validator } from 'shared/components/Validation';

import {
  Label,
  LabelsInput,
  LabelsInputProps,
  LabelsRule,
  nonEmptyLabels,
} from './LabelsInput';
import {
  AtLeastOneRequired,
  Custom,
  Default,
  Disabled,
} from './LabelsInput.story';

/** Marks asterisks in the required column headings. */
const requiredMarkRegexp = /\*/;

test('defaults, with empty labels', async () => {
  render(<Default />);

  expect(screen.queryByText(/key/i)).not.toBeInTheDocument();
  expect(screen.queryByText(/value/i)).not.toBeInTheDocument();
  expect(screen.queryByText(requiredMarkRegexp)).not.toBeInTheDocument();
  expect(screen.queryByPlaceholderText('label key')).not.toBeInTheDocument();
  expect(screen.queryByPlaceholderText('label value')).not.toBeInTheDocument();

  fireEvent.click(screen.getByText(/add a label/i));

  expect(screen.getByText(/key/i)).toBeInTheDocument();
  expect(screen.getByText(/value/i)).toBeInTheDocument();
  expect(screen.getAllByText(requiredMarkRegexp)).toHaveLength(2);
  expect(screen.getByPlaceholderText('label key')).toBeInTheDocument();
  expect(screen.getByPlaceholderText('label value')).toBeInTheDocument();
  expect(screen.getByTitle(/remove label/i)).toBeInTheDocument();

  fireEvent.click(screen.getByText(/add another label/i));

  expect(screen.getAllByPlaceholderText('label key')).toHaveLength(2);
  expect(screen.getAllByPlaceholderText('label value')).toHaveLength(2);
  expect(screen.getAllByTitle(/remove label/i)).toHaveLength(2);
});

test('with custom texts', async () => {
  render(<Custom />);

  fireEvent.click(screen.getByText(/add a custom adjective/i));

  expect(screen.getByText(/custom key name/i)).toBeInTheDocument();
  expect(screen.getByText(/custom value/i)).toBeInTheDocument();
  expect(screen.getAllByText(requiredMarkRegexp)).toHaveLength(2);
  expect(
    screen.getByPlaceholderText('custom key placeholder')
  ).toBeInTheDocument();
  expect(
    screen.getByPlaceholderText('custom value placeholder')
  ).toBeInTheDocument();

  expect(
    screen.getByRole('button', { name: 'Add another Custom Adjective' })
  ).toBeInTheDocument();

  // Delete the only row.
  fireEvent.click(screen.getByTitle(/remove custom adjective/i));
  expect(
    screen.getByRole('button', { name: 'Add a Custom Adjective' })
  ).toBeInTheDocument();
  expect(
    screen.queryByPlaceholderText('custom key placeholder')
  ).not.toBeInTheDocument();
  expect(
    screen.queryByPlaceholderText('custom value placeholder')
  ).not.toBeInTheDocument();
});

test('disabled buttons', async () => {
  const user = userEvent.setup();
  render(<Disabled />);

  expect(screen.getByTitle(/remove label/i)).toBeDisabled();
  expect(
    screen.getByRole('button', { name: 'Add another Label' })
  ).toBeDisabled();

  await user.click(screen.getByTitle(/remove label/i));
  expect(
    screen.getAllByRole('textbox').map((t: HTMLInputElement) => t.value)
  ).toEqual(['some-name', 'some-value']);
});

test('removing last label is not possible due to requiring labels', async () => {
  render(<AtLeastOneRequired />);

  expect(screen.getByPlaceholderText('label key')).toBeInTheDocument();
  expect(screen.getByPlaceholderText('label value')).toBeInTheDocument();

  fireEvent.click(screen.getByTitle(/remove label/i));

  expect(screen.getByPlaceholderText('label key')).toBeInTheDocument();
  expect(screen.getByPlaceholderText('label value')).toBeInTheDocument();
});

test('at least one row', async () => {
  function TestCase({ onLabelsChange }: { onLabelsChange(l: Label[]): void }) {
    const [labels, setLabels] = useState<Label[]>([]);
    function handleSetLabels(l) {
      setLabels(l);
      onLabelsChange(l);
    }
    return (
      <Validation>
        <LabelsInput
          legend="Labels"
          labels={labels}
          setLabels={handleSetLabels}
          atLeastOneRow
        />
      </Validation>
    );
  }

  const user = userEvent.setup();
  const onLabelsChange = jest.fn();
  render(<TestCase onLabelsChange={onLabelsChange} />);

  expect(screen.getByTitle(/remove label/i)).toBeDisabled();

  // Set one label.
  await user.type(screen.getByPlaceholderText('label key'), 'foo');
  await user.type(screen.getByPlaceholderText('label value'), 'bar');
  expect(onLabelsChange).toHaveBeenLastCalledWith([
    { name: 'foo', value: 'bar' },
  ]);

  // Remove the label, expect an empty model.
  await user.click(screen.getByTitle(/remove label/i));
  expect(
    screen.getAllByRole('textbox').map((t: HTMLInputElement) => t.value)
  ).toEqual(['', '']);
  expect(onLabelsChange).toHaveBeenLastCalledWith([]);

  // Start typing, expect a non-empty model.
  await user.type(screen.getAllByRole('textbox')[0], 'foo');
  expect(onLabelsChange).toHaveBeenLastCalledWith([{ name: 'foo', value: '' }]);

  // Clear the text box, expect an empty model.
  await user.clear(screen.getAllByRole('textbox')[0]);
  expect(onLabelsChange).toHaveBeenLastCalledWith([]);

  // Set the first label, add another empty one, and then remove the first one.
  // Expect an empty model.
  await user.type(screen.getByPlaceholderText('label key'), 'foo');
  await user.type(screen.getByPlaceholderText('label value'), 'bar');
  await user.click(screen.getByRole('button', { name: 'Add another Label' }));
  await user.click(screen.getAllByTitle(/remove label/i)[0]);
  expect(onLabelsChange).toHaveBeenLastCalledWith([]);
});

describe('validation rules', () => {
  function setup(props: Partial<LabelsInputProps>) {
    let validator: Validator;
    render(
      <Validation>
        {({ validator: v }) => {
          validator = v;
          return <LabelsInput labels={[]} setLabels={() => {}} {...props} />;
        }}
      </Validation>
    );
    return { validator };
  }

  describe.each([
    { name: 'explicitly enforced standard rule', rule: nonEmptyLabels },
    { name: 'implicit standard rule', rule: undefined },
  ])('$name', ({ rule }) => {
    test('invalid', () => {
      const { validator } = setup({
        labels: [
          { name: '', value: 'foo' },
          { name: 'bar', value: '' },
          { name: 'asdf', value: 'qwer' },
        ],
        rule,
      });
      act(() => validator.validate());
      expect(validator.state.valid).toBe(false);
      expect(screen.getAllByRole('textbox')[0]).toHaveAccessibleDescription(
        'required'
      ); // ''
      expect(screen.getAllByRole('textbox')[1]).toHaveAccessibleDescription(''); // 'foo'
      expect(screen.getAllByRole('textbox')[2]).toHaveAccessibleDescription(''); // 'bar'
      expect(screen.getAllByRole('textbox')[3]).toHaveAccessibleDescription(
        'required'
      ); // ''
      expect(screen.getAllByRole('textbox')[4]).toHaveAccessibleDescription(''); // 'asdf'
      expect(screen.getAllByRole('textbox')[5]).toHaveAccessibleDescription(''); // 'qwer'
    });

    test('valid', () => {
      const { validator } = setup({
        labels: [
          { name: '', value: 'foo' },
          { name: 'bar', value: '' },
          { name: 'asdf', value: 'qwer' },
        ],
        rule,
      });
      act(() => validator.validate());
      expect(validator.state.valid).toBe(false);
      expect(screen.getAllByRole('textbox')[0]).toHaveAccessibleDescription(
        'required'
      ); // ''
      expect(screen.getAllByRole('textbox')[1]).toHaveAccessibleDescription(''); // 'foo'
      expect(screen.getAllByRole('textbox')[2]).toHaveAccessibleDescription(''); // 'bar'
      expect(screen.getAllByRole('textbox')[3]).toHaveAccessibleDescription(
        'required'
      ); // ''
      expect(screen.getAllByRole('textbox')[4]).toHaveAccessibleDescription(''); // 'asdf'
      expect(screen.getAllByRole('textbox')[5]).toHaveAccessibleDescription(''); // 'qwer'
    });

    test('at least one row, no labels', () => {
      const { validator } = setup({ labels: [], rule, atLeastOneRow: true });
      act(() => validator.validate());
      expect(screen.getAllByRole('textbox')[0]).toHaveAccessibleDescription('');
      expect(screen.getAllByRole('textbox')[1]).toHaveAccessibleDescription('');
    });
  });

  const nameNotFoo: LabelsRule = (labels: Label[]) => () => {
    const results = labels.map(label => ({
      name:
        label.name === 'foo'
          ? { valid: false, message: 'no foo please' }
          : { valid: true },
      value: { valid: true },
    }));
    return {
      valid: results.every(r => r.name.valid && r.value.valid),
      results: results,
    };
  };

  test('custom rule, invalid', async () => {
    const { validator } = setup({
      labels: [
        { name: 'foo', value: 'bar' },
        { name: 'bar', value: 'foo' },
      ],
      rule: nameNotFoo,
    });
    act(() => validator.validate());
    expect(validator.state.valid).toBe(false);
    expect(screen.getAllByRole('textbox')[0]).toHaveAccessibleDescription(
      'no foo please'
    ); // 'foo' key
    expect(screen.getAllByRole('textbox')[1]).toHaveAccessibleDescription('');
    expect(screen.getAllByRole('textbox')[2]).toHaveAccessibleDescription('');
    expect(screen.getAllByRole('textbox')[3]).toHaveAccessibleDescription('');
  });

  test('custom rule, valid', async () => {
    const { validator } = setup({
      labels: [
        { name: 'xyz', value: 'bar' },
        { name: 'bar', value: 'foo' },
      ],
      rule: nameNotFoo,
    });
    act(() => validator.validate());
    expect(validator.state.valid).toBe(true);
    expect(screen.getAllByRole('textbox')[0]).toHaveAccessibleDescription('');
    expect(screen.getAllByRole('textbox')[1]).toHaveAccessibleDescription('');
    expect(screen.getAllByRole('textbox')[2]).toHaveAccessibleDescription('');
    expect(screen.getAllByRole('textbox')[3]).toHaveAccessibleDescription('');
  });
});
