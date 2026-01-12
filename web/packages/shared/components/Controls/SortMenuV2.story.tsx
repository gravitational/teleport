/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import Flex from 'design/Flex/Flex';

import { SortMenu, SortOrder } from './SortMenuV2';

export default {
  title: 'Shared/Controls/SortMenuV2',
  component: SortMenu,
  argTypes: {},
  args: {
    items: [
      {
        key: 'name',
        label: 'Name',
        ascendingLabel: 'Name, A - Z',
        descendingLabel: 'Name, Z - A',
        ascendingOptionLabel: 'Alphabetical, A - Z',
        descendingOptionLabel: 'Alphabetical, Z - A',
      },
      {
        key: 'created',
        label: 'Created',
        ascendingLabel: 'Oldest',
        descendingLabel: 'Newest',
        ascendingOptionLabel: 'Oldest',
        descendingOptionLabel: 'Newest',
        defaultOrder: 'DESC',
      },
      {
        key: 'status',
        label: 'Status',
        ascendingOptionLabel: 'Alphabetical, A - Z',
        descendingOptionLabel: 'Alphabetical, Z - A',
      },
      {
        key: 'relevance',
        label: 'Relevance',
        disableSort: true,
        defaultOrder: 'DESC',
      },
    ],
    selectedKey: 'name',
    selectedOrder: 'ASC',
    onChange: action('onChange'),
  },
  render: (args => {
    const [, updateArgs] =
      useArgs<NonNullable<Meta<typeof SortMenu>['args']>>();
    const onChange = (key: string, order: SortOrder) => {
      updateArgs({ selectedKey: key, selectedOrder: order });
      args.onChange?.(key, order);
    };
    return (
      <Flex flexDirection="column" alignItems="flex-end" minWidth="100%">
        <SortMenu
          items={args.items}
          selectedKey={args.selectedKey}
          selectedOrder={args.selectedOrder}
          onChange={onChange}
        />
      </Flex>
    );
  }) satisfies StoryFn<typeof SortMenu>,
} satisfies Meta<typeof SortMenu>;

const Default: StoryObj<typeof SortMenu> = { args: {} };

export { Default as SortMenuV2 };
