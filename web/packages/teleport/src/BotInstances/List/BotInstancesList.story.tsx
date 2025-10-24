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

import { Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient } from '@tanstack/react-query';
import { ComponentProps, useRef, useState } from 'react';

import { CardTile } from 'design/CardTile/CardTile';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { BotInstanceSummary } from 'teleport/services/bot/types';

import { BotInstancesList, BotInstancesListControls } from './BotInstancesList';

const meta = {
  title: 'Teleport/BotInstances/List',
  component: Wrapper,
  beforeEach: () => {
    queryClient.clear(); // Prevent cached data sharing between stories
  },
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

export const Happy: Story = {};

export const Empty: Story = {
  args: {
    data: [],
  },
};

export const ErrorLoadingList: Story = {
  args: {
    error: new Error('something went wrong'),
  },
};

export const NoMoreToLoad: Story = {
  args: {
    hasNextPage: false,
  },
};

export const LoadingMore: Story = {
  args: {
    isFetchingNextPage: true,
  },
};

export const UnsupportedSort: Story = {
  args: {
    error: new Error('unsupported sort: something went wrong'),
  },
};

export const StillLoadingList: Story = {
  args: {
    isLoading: true,
  },
};

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: false,
    },
  },
});

function Wrapper(
  props?: Partial<
    Pick<
      ComponentProps<typeof BotInstancesList>,
      'error' | 'isLoading' | 'hasNextPage' | 'isFetchingNextPage' | 'data'
    >
  >
) {
  const {
    data = [
      {
        bot_name: 'ansible-worker',
        instance_id: `966c0850-9bb5-4ed7-af2d-4b1f202a936a`,
        active_at_latest: '2025-07-22T10:54:00Z',
        host_name_latest: 'my-svc.my-namespace.svc.cluster-domain.example',
        join_method_latest: 'github',
        os_latest: 'linux',
        version_latest: '2.4.0',
      },
      {
        bot_name: 'ansible-worker',
        instance_id: 'ac7135ce-fde6-4a91-bd77-ba7419e1c175',
        active_at_latest: '2025-07-22T10:54:00Z',
        host_name_latest: 'win-123a',
        join_method_latest: 'tpm',
        os_latest: 'windows',
        version_latest: '4.3.18+ab12hd',
      },
      {
        bot_name: 'ansible-worker',
        instance_id: '5283f4a9-c49b-4876-be48-b5f83000e612',
        active_at_latest: '2025-07-22T10:54:00Z',
        host_name_latest: 'mac-007',
        join_method_latest: 'kubernetes',
        os_latest: 'darwin',
        version_latest: '3.9.99',
      },
    ],
    error = null,
    hasNextPage = true,
    isFetchingNextPage = false,
    isLoading = false,
  } = props ?? {};

  const [allData, setAllData] = useState(data);
  const [selected, setSelected] = useState<string | null>(null);
  const [sortField, setSortField] = useState<string>('active_at_latest');
  const [sortDir, setSortDir] = useState<'ASC' | 'DESC'>('ASC');

  const listRef = useRef<BotInstancesListControls | null>(null);

  const ctx = createTeleportContext();

  return (
    <TeleportProviderBasic teleportCtx={ctx}>
      <CardTile
        height={600}
        width={400}
        overflow={'auto'}
        p={0}
        flexDirection={'row'}
      >
        <BotInstancesList
          ref={listRef}
          data={allData}
          isLoading={isLoading}
          isFetchingNextPage={isFetchingNextPage}
          error={error}
          hasNextPage={hasNextPage}
          sortField={sortField}
          sortDir={sortDir}
          selectedItem={selected}
          onSortChanged={(sortField: string, sortDir: 'ASC' | 'DESC') => {
            setSortField(sortField);
            setSortDir(sortDir);
            listRef.current?.scrollToTop();
          }}
          onLoadNextPage={() =>
            setAllData(existing => {
              const newData = (data ?? []).map(i => {
                return {
                  ...i,
                  instance_id: crypto.randomUUID(),
                };
              });
              return [...(existing ?? []), ...newData];
            })
          }
          onItemSelected={function (item: BotInstanceSummary | null): void {
            setSelected(item ? `${item.bot_name}/${item.instance_id}` : null);
          }}
        />
      </CardTile>
    </TeleportProviderBasic>
  );
}
