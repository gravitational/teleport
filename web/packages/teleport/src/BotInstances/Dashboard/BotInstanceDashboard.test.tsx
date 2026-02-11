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
import { setupServer } from 'msw/node';
import { ComponentProps, PropsWithChildren } from 'react';

import { darkTheme } from 'design/theme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  render,
  screen,
  testQueryClient,
  userEvent,
  waitForElementToBeRemoved,
  within,
} from 'design/utils/testing';

import {
  getBotInstanceMetricsError,
  getBotInstanceMetricsSuccess,
} from 'teleport/test/helpers/botInstances';

import { BotInstancesDashboard } from './BotInstanceDashboard';

const server = setupServer();

beforeAll(() => {
  server.listen();
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();

  jest.clearAllMocks();
});

afterAll(() => server.close());

describe('BotInstanceDashboard', () => {
  it('renders', async () => {
    withSuccessResponse();

    renderComponent();

    await waitForLoading();

    expect(screen.getByText('Insights')).toBeInTheDocument();
    expect(screen.getByText('Version Compatibility')).toBeInTheDocument();

    const upToDate = screen.getByLabelText('Up to date');
    expect(within(upToDate).getByText('100 (57%)')).toBeInTheDocument();

    const patch = screen.getByLabelText('Patch available');
    expect(within(patch).getByText('50 (29%)')).toBeInTheDocument();

    const upgrade = screen.getByLabelText('Upgrade required');
    expect(within(upgrade).getByText('25 (14%)')).toBeInTheDocument();

    const unsupported = screen.getByLabelText('Unsupported');
    expect(within(unsupported).getByText('0 (0%)')).toBeInTheDocument();

    expect(
      screen.getByText('Select a category above to filter bot instances.')
    ).toBeInTheDocument();
  });

  it('shows no data message', async () => {
    withSuccessResponse({
      upgrade_statuses: null,
      refresh_after_seconds: 60_000,
    });

    renderComponent();

    await waitForLoading();

    expect(screen.getByText('No data available')).toBeInTheDocument();
    expect(
      screen.queryByText('Select a status above to view instances.')
    ).not.toBeInTheDocument();
  });

  it('shows an error', async () => {
    withErrorResponse(500, 'something went wrong');

    renderComponent();

    await waitForLoading();

    expect(screen.getByText('something went wrong')).toBeInTheDocument();
    expect(
      screen.queryByText('Select a status above to view instances.')
    ).not.toBeInTheDocument();
  });

  it('items are selectable', async () => {
    const onFilterSelected = jest.fn();

    withSuccessResponse();

    const { user } = renderComponent({ props: { onFilterSelected } });

    await waitForLoading();

    {
      const item = screen.getByLabelText('Up to date');
      await user.click(item);
      expect(onFilterSelected).toHaveBeenCalledTimes(1);
      expect(onFilterSelected).toHaveBeenLastCalledWith(
        'mock up-to-date filter'
      );
    }

    {
      const item = screen.getByLabelText('Patch available');
      await user.click(item);
      expect(onFilterSelected).toHaveBeenCalledTimes(2);
      expect(onFilterSelected).toHaveBeenLastCalledWith('mock patch filter');
    }

    {
      const item = screen.getByLabelText('Upgrade required');
      await user.click(item);
      expect(onFilterSelected).toHaveBeenCalledTimes(3);
      expect(onFilterSelected).toHaveBeenLastCalledWith('mock upgrade filter');
    }

    {
      const item = screen.getByLabelText('Unsupported');
      await user.click(item);
      expect(onFilterSelected).toHaveBeenCalledTimes(4);
      expect(onFilterSelected).toHaveBeenLastCalledWith(
        'mock unsupported filter'
      );
    }
  });

  it('refreshes', async () => {
    const onFilterSelected = jest.fn();

    withSuccessResponse();

    const { user } = renderComponent({ props: { onFilterSelected } });

    await waitForLoading();

    {
      const upToDate = screen.getByLabelText('Up to date');
      expect(within(upToDate).getByText('100 (57%)')).toBeInTheDocument();
    }

    withSuccessResponse({
      upgrade_statuses: {
        up_to_date: {
          count: 99,
        },
        patch_available: {
          count: 0,
        },
        requires_upgrade: {
          count: 0,
        },
        unsupported: {
          count: 0,
        },
        updated_at: '1970-01-01T00:00:00Z',
      },
      refresh_after_seconds: 60_000,
    });

    const refreshButton = screen.getByLabelText('refresh');
    await user.click(refreshButton);

    {
      const upToDate = screen.getByLabelText('Up to date');
      expect(within(upToDate).getByText('99 (100%)')).toBeInTheDocument();
    }
  });
});

function renderComponent(options?: {
  props?: ComponentProps<typeof BotInstancesDashboard>;
}) {
  const { props } = options ?? {};
  const { onFilterSelected = jest.fn() } = props ?? {};

  const user = userEvent.setup();

  return {
    ...render(<BotInstancesDashboard onFilterSelected={onFilterSelected} />, {
      wrapper: makeWrapper(),
    }),
    user,
    history,
  };
}

function makeWrapper() {
  return ({ children }: PropsWithChildren) => {
    return (
      <QueryClientProvider client={testQueryClient}>
        <ConfiguredThemeProvider theme={darkTheme}>
          {children}
        </ConfiguredThemeProvider>
      </QueryClientProvider>
    );
  };
}

async function waitForLoading() {
  await waitForElementToBeRemoved(() =>
    screen.queryByTestId('loading-dashboard')
  );
}

function withSuccessResponse(
  mock: Parameters<typeof getBotInstanceMetricsSuccess>[0] = {
    upgrade_statuses: {
      up_to_date: {
        count: 100,
        filter: 'mock up-to-date filter',
      },
      patch_available: {
        count: 50,
        filter: 'mock patch filter',
      },
      requires_upgrade: {
        count: 25,
        filter: 'mock upgrade filter',
      },
      unsupported: {
        count: 0,
        filter: 'mock unsupported filter',
      },
      updated_at: new Date().toISOString(),
    },
    refresh_after_seconds: 60_000,
  }
) {
  server.use(getBotInstanceMetricsSuccess(mock));
}

function withErrorResponse(
  ...params: Parameters<typeof getBotInstanceMetricsError>
) {
  server.use(getBotInstanceMetricsError(...params));
}
