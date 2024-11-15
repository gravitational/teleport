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

import { SortMenu } from './SortMenu';

import type { Meta, StoryFn, StoryObj } from '@storybook/react';

export default {
  title: 'Shared/Controls/SortMenu',
  component: SortMenu<any>,
  argTypes: {
    current: {
      control: false,
      description: 'Current sort',
      table: {
        type: {
          summary:
            "Array<{ fieldName: Exclude<keyof T, symbol | number>; dir: 'ASC' | 'DESC'>",
        },
      },
    },
    fields: {
      control: false,
      description: 'Fields to sort by',
      table: {
        type: {
          summary:
            '{ value: Exclude<keyof T, symbol | number>; label: string }[]',
        },
      },
    },
    onChange: {
      control: false,
      description: 'Callback when fieldName or dir is changed',
      table: {
        type: {
          summary:
            "(value: { fieldName: Exclude<keyof T, symbol | number>; dir: 'ASC' | 'DESC' }) => void",
        },
      },
    },
  },
  args: {
    current: { fieldName: 'name', dir: 'ASC' },
    fields: [
      { value: 'name', label: 'Name' },
      { value: 'created', label: 'Created' },
      { value: 'updated', label: 'Updated' },
    ],
  },
  parameters: { controls: { expanded: true, exclude: ['userContext'] } },
} satisfies Meta<typeof SortMenu<any>>;

const Default: StoryObj<typeof SortMenu> = {
  render: (({ current, fields }) => {
    const [sort, setSort] = useState(current);
    return (
      <Flex alignItems="center" minHeight="100px">
        <SortMenu current={sort} fields={fields} onChange={setSort} />
      </Flex>
    );
  }) satisfies StoryFn<typeof SortMenu>,
};

export { Default as SortMenu };
