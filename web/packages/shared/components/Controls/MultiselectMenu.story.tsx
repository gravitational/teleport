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

import React, { useState } from 'react';
import { Flex } from 'design';

import { MultiselectMenu } from './MultiselectMenu';

import type { Meta, StoryFn, StoryObj } from '@storybook/react';

type OptionValue = `option-${number}`;

const options: {
  value: OptionValue;
  label: string | React.ReactNode;
  disabled?: boolean;
  disabledTooltip?: string;
}[] = [
  { value: 'option-1', label: 'Option 1' },
  { value: 'option-2', label: 'Option 2' },
  { value: 'option-3', label: 'Option 3' },
  { value: 'option-4', label: 'Option 4' },
];

const optionsWithCustomLabels: typeof options = [
  {
    value: 'option-1',
    label: <strong>Bold Option 1</strong>,
  },
  {
    value: 'option-3',
    label: <em>Italic Option 3</em>,
  },
];

export default {
  title: 'Shared/Controls/MultiselectMenu',
  component: MultiselectMenu,
  argTypes: {
    buffered: {
      control: { type: 'boolean' },
      description: 'Buffer selections until "Apply" is clicked',
      table: { defaultValue: { summary: 'false' } },
    },
    showIndicator: {
      control: { type: 'boolean' },
      description: 'Show indicator when there are selected options',
      table: { defaultValue: { summary: 'true' } },
    },
    showSelectControls: {
      control: { type: 'boolean' },
      description: 'Show select controls (Select All/Select None)',
      table: { defaultValue: { summary: 'true' } },
    },
    label: {
      control: { type: 'text' },
      description: 'Label for the multiselect',
    },
    tooltip: {
      control: { type: 'text' },
      description: 'Tooltip for the label',
    },
    selected: {
      control: false,
      description: 'Currently selected options',
      table: { type: { summary: 'T[]' } },
    },
    onChange: {
      control: false,
      description: 'Callback when selection changes',
      table: { type: { summary: 'selected: T[]' } },
    },
    options: {
      control: false,
      description: 'Options to select from',
      table: {
        type: {
          summary:
            'Array<{ value: T; label: string | ReactNode; disabled?: boolean; disabledTooltip?: string; }>',
        },
      },
    },
  },
  args: {
    label: 'Select Options',
    tooltip: 'Choose multiple options',
    buffered: false,
    showIndicator: true,
    showSelectControls: true,
  },
  parameters: { controls: { expanded: true, exclude: ['userContext'] } },
  render: (args => {
    const [selected, setSelected] = useState<string[]>([]);
    return (
      <Flex alignItems="center" minHeight="100px">
        <MultiselectMenu {...args} selected={selected} onChange={setSelected} />
      </Flex>
    );
  }) satisfies StoryFn<typeof MultiselectMenu<OptionValue>>,
} satisfies Meta<typeof MultiselectMenu<OptionValue>>;

type Story = StoryObj<typeof MultiselectMenu<OptionValue>>;

const Default: Story = { args: { options } };

const WithCustomLabels: Story = { args: { options: optionsWithCustomLabels } };

const WithDisabledOption: Story = {
  args: {
    options: [
      ...options,
      {
        value: 'option-5',
        label: 'Option 5',
        disabled: true,
        disabledTooltip: 'Lorum ipsum dolor sit amet',
      },
    ],
  },
};

export { Default, WithCustomLabels, WithDisabledOption };
