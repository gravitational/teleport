/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { Meta, StoryFn, StoryObj } from '@storybook/react-vite';
import { action } from 'storybook/actions';
import { useArgs } from 'storybook/preview-api';

import Flex from 'design/Flex';
import { H2 } from 'design/Text';

import { ButtonSelect } from './ButtonSelect';

const defaultArgs = {
  fullWidth: false,
  disabled: false,
  onChange: action('onChange'),
} as const;

type StoryMeta = Meta<typeof ButtonSelect>;

type Story = StoryObj<typeof ButtonSelect>;

export default {
  title: 'Design/ButtonSelect',
  component: ButtonSelect,
  argTypes: {
    options: {
      control: false,
      description: 'Array of options to display in the button select',
      table: {
        type: {
          summary:
            '{ value: (string | number | bigint), label: string, disabled?: boolean, tooltip?: string }[]',
        },
      },
    },
    activeValue: {
      control: false,
      description: 'The value of the currently active option',
      table: { type: { summary: 'string | number | bigint' } },
    },
    onChange: {
      control: false,
      description: 'Callback function called when the active button changes',
      table: {
        type: { summary: '(selectedValue: string | number | bigint) => void' },
      },
    },
    fullWidth: {
      control: { type: 'boolean' },
      description:
        'If true, the button select will take the full width of its container',
      table: { defaultValue: { summary: 'false' } },
    },
    disabled: {
      control: { type: 'boolean' },
      description: 'If true, all options will be disabled',
      table: { defaultValue: { summary: 'false' } },
    },
  },
  args: defaultArgs,
  render: (args => {
    const [{ activeValue }, updateArgs] =
      useArgs<NonNullable<StoryMeta['args']>>();
    const onChange = (value: typeof activeValue) => {
      value ??= args.options[0]?.value;
      updateArgs({ activeValue: value });
      args.onChange?.(value);
    };

    return (
      <Flex flexDirection="column" gap={3} maxWidth="600px">
        <ButtonSelect
          {...args}
          activeValue={activeValue ?? args.options[0]?.value}
          onChange={onChange}
        />
        <H2>{`Active Value: ${activeValue ?? args.options[0]?.value}`}</H2>
      </Flex>
    );
  }) satisfies StoryFn<typeof ButtonSelect>,
} satisfies StoryMeta;

const WithTwoOptions: Story = {
  args: {
    options: [
      { value: '1', label: 'Option 1' },
      { value: '2', label: 'Option 2' },
    ],
  },
};

const WithFourOptions: Story = {
  args: {
    options: [
      { value: '1', label: 'Option 1' },
      { value: '2', label: 'Option 2' },
      { value: '3', label: 'Option 3' },
      { value: '4', label: 'Option 4' },
    ],
  },
};

const WithDisabledOption: Story = {
  args: {
    options: [
      { value: '1', label: 'Option 1' },
      { value: '2', label: 'Option 2' },
      {
        value: '3',
        label: 'Option 3',
        disabled: true,
        tooltip: 'Option 3 is disabled',
      },
      { value: '4', label: 'Option 4' },
    ],
  },
};

const WithNonStringValues: Story = {
  args: {
    options: [
      { value: 1, label: 'Option 1' },
      { value: 2, label: 'Option 2' },
      { value: 3, label: 'Option 3' },
    ],
  },
};

export {
  WithTwoOptions,
  WithFourOptions,
  WithDisabledOption,
  WithNonStringValues,
};
