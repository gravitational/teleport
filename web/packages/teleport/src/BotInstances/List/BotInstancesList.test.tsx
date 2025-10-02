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

import { QueryClientProvider } from '@tanstack/react-query';
import { ComponentProps, PropsWithChildren } from 'react';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import { BotInstancesList } from './BotInstancesList';

jest.mock('design/utils/copyToClipboard', () => {
  return {
    __esModule: true,
    copyToClipboard: jest.fn(),
  };
});

afterEach(async () => {
  jest.clearAllMocks();
});

describe('BotIntancesList', () => {
  it('renders items', async () => {
    renderComponent();

    expect(screen.getByText('ansible-worker/966c085')).toBeInTheDocument();
    expect(screen.getByText('ansible-worker/ac7135c')).toBeInTheDocument();
    expect(screen.getByText('ansible-worker/5283f4a')).toBeInTheDocument();
  });

  it('select an item', async () => {
    const onItemSelected = jest.fn();

    const { user } = renderComponent({
      props: {
        onItemSelected,
      },
    });

    const item2 = screen.getByRole('listitem', {
      name: 'ansible-worker/ac7135ce-fde6-4a91-bd77-ba7419e1c175',
    });
    await user.click(item2);

    expect(onItemSelected).toHaveBeenCalledTimes(1);
    expect(onItemSelected).toHaveBeenLastCalledWith({
      active_at_latest: '2025-07-22T10:54:00Z',
      bot_name: 'ansible-worker',
      host_name_latest: 'win-123a',
      instance_id: 'ac7135ce-fde6-4a91-bd77-ba7419e1c175',
      join_method_latest: 'tpm',
      os_latest: 'windows',
      version_latest: '4.3.18+ab12hd',
    });
  });

  it('Shows a loading state', async () => {
    renderComponent({
      props: {
        isLoading: true,
      },
    });

    expect(screen.getByTestId('loading')).toBeInTheDocument();
  });

  it('Shows an empty state', async () => {
    renderComponent({
      props: {
        data: [],
      },
    });

    expect(screen.getByText('No active instances')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Bot instances are ephemeral, and disappear once all issued credentials have expired.'
      )
    ).toBeInTheDocument();
  });

  it('Shows an error', async () => {
    renderComponent({
      props: {
        error: new Error('something went wrong'),
      },
    });

    expect(screen.getByText('something went wrong')).toBeInTheDocument();
  });

  it('Allows fetch more action', async () => {
    const onLoadNextPage = jest.fn();

    const { user } = renderComponent({
      props: {
        hasNextPage: true,
        onLoadNextPage,
      },
    });

    const action = screen.getByText('Load More');
    await user.click(action);

    expect(onLoadNextPage).toHaveBeenCalledTimes(1);
  });

  it('Prevents next page action when no page', async () => {
    const onLoadNextPage = jest.fn();

    const { user } = renderComponent({
      props: {
        hasNextPage: false,
        onLoadNextPage,
      },
    });

    const action = screen.getByText('Load More');
    await user.click(action);

    expect(action).toBeDisabled();
    expect(onLoadNextPage).not.toHaveBeenCalled();
  });

  it('Prevents next page action when loading next page', async () => {
    const onLoadNextPage = jest.fn();

    const { user } = renderComponent({
      props: {
        hasNextPage: true,
        isFetchingNextPage: true,
        onLoadNextPage,
      },
    });

    const action = screen.getByText('Load More');
    await user.click(action);

    expect(action).toBeDisabled();
    expect(onLoadNextPage).not.toHaveBeenCalled();
  });

  it('Allows sort change', async () => {
    const onSortChanged = jest.fn();

    const { user } = renderComponent({
      props: {
        onSortChanged,
        sortField: 'active_at_latest',
        sortDir: 'DESC',
      },
    });

    const fieldAction = screen.getByRole('button', { name: 'Sort by' });
    await user.click(fieldAction);

    expect(
      screen.getByRole('menuitem', { name: 'Bot name' })
    ).toBeInTheDocument();
    expect(
      screen.getByRole('menuitem', { name: 'Recent' })
    ).toBeInTheDocument();
    expect(
      screen.getByRole('menuitem', { name: 'Hostname' })
    ).toBeInTheDocument();
    const versionOption = screen.getByRole('menuitem', { name: 'Version' });
    await user.click(versionOption);

    expect(onSortChanged).toHaveBeenLastCalledWith('version_latest', 'DESC');

    const dirAction = screen.getByRole('button', { name: 'Sort direction' });
    await user.click(dirAction);

    // The component under test does not keep sort state so the sort field will
    // be 'active_at_latest' on the next change.
    expect(onSortChanged).toHaveBeenLastCalledWith('active_at_latest', 'ASC');
  });

  it('Shows an unsupported sort error', async () => {
    const onSortChanged = jest.fn();

    const { user } = renderComponent({
      props: {
        onSortChanged,
        sortField: 'active_at_latest',
        sortDir: 'DESC',
        error: new Error('unsupported sort: foo'),
      },
    });

    expect(screen.getByText('unsupported sort: foo')).toBeInTheDocument();

    const resetAction = screen.getByRole('button', { name: 'Reset sort' });
    await user.click(resetAction);

    expect(onSortChanged).toHaveBeenLastCalledWith('bot_name', 'ASC');
  });
});

const renderComponent = (options?: {
  props: Partial<ComponentProps<typeof BotInstancesList>>;
}) => {
  const { props } = options ?? {};
  const {
    data = mockData,
    isLoading = false,
    isFetchingNextPage = false,
    error = null,
    hasNextPage = true,
    sortField = 'bot_name',
    sortDir = 'ASC',
    selectedItem = null,
    onSortChanged = jest.fn(),
    onLoadNextPage = jest.fn(),
    onItemSelected = jest.fn(),
  } = props ?? {};

  const user = userEvent.setup();
  return {
    ...render(
      <BotInstancesList
        data={data}
        isLoading={isLoading}
        isFetchingNextPage={isFetchingNextPage}
        error={error}
        hasNextPage={hasNextPage}
        sortField={sortField}
        sortDir={sortDir}
        selectedItem={selectedItem}
        onSortChanged={onSortChanged}
        onLoadNextPage={onLoadNextPage}
        onItemSelected={onItemSelected}
      />,
      {
        wrapper: makeWrapper(),
      }
    ),
    user,
  };
};

function makeWrapper() {
  const ctx = createTeleportContext();
  return (props: PropsWithChildren) => {
    return (
      <QueryClientProvider client={testQueryClient}>
        <TeleportProviderBasic teleportCtx={ctx}>
          <ConfiguredThemeProvider theme={darkTheme}>
            {props.children}
          </ConfiguredThemeProvider>
        </TeleportProviderBasic>
      </QueryClientProvider>
    );
  };
}

const mockData = [
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
];
