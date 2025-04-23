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

import { action } from '@storybook/addon-actions';
import { useArgs } from '@storybook/preview-api';
import type { Meta, StoryFn, StoryObj } from '@storybook/react';

import { Flex } from 'design';

import { SortMenu } from './SortMenu';

const STUB_FIELDS = ['name', 'created', 'updated', 'status'] as const;

export default {
  title: 'Shared/Controls/SortMenu',
  component: SortMenu<any>,
  argTypes: {
    current: {
      control: { type: 'select' },
      options: STUB_FIELDS.reduce(
        (acc, v) => [...acc, `${v} (Asc)`, `${v} (Desc)`],
        []
      ),
      mapping: STUB_FIELDS.reduce(
        (acc, v) => ({
          ...acc,
          [`${v} (Asc)`]: { fieldName: v, dir: 'ASC' },
          [`${v} (Desc)`]: { fieldName: v, dir: 'DESC' },
        }),
        {}
      ),
      description: 'Current sort',
      table: {
        type: {
          summary:
            "Array<{ fieldName: Exclude<keyof T, symbol | number>; dir: 'ASC' | 'DESC' }>",
        },
      },
    },
    fields: {
      control: false,
      description: 'Fields to sort by',
      table: {
        type: {
          summary:
            'Array<{ value: Exclude<keyof T, symbol | number>; label: string }>',
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
    current: { fieldName: STUB_FIELDS[0], dir: 'ASC' },
    fields: STUB_FIELDS.map(v => ({
      value: v,
      label: `${v.charAt(0).toUpperCase()}${v.slice(1)}`,
    })),
    onChange: action('onChange'),
  },
  render: (args => {
    const [{ current }, updateArgs] =
      useArgs<Meta<typeof SortMenu<any>>['args']>();
    const onChange = (value: typeof current) => {
      updateArgs({ current: value });
      args.onChange?.(value);
    };
    return (
      <Flex alignItems="center" minHeight="100px">
        <SortMenu current={current} fields={args.fields} onChange={onChange} />
      </Flex>
    );
  }) satisfies StoryFn<typeof SortMenu>,
} satisfies Meta<typeof SortMenu<any>>;

const Default: StoryObj<typeof SortMenu> = { args: {} };

export { Default as SortMenu };
