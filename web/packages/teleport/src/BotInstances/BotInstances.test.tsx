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
import { PropsWithChildren } from 'react';
import { MemoryRouter } from 'react-router';

import { darkTheme } from 'design/theme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  fireEvent,
  render,
  screen,
  testQueryClient,
  userEvent,
  waitForElementToBeRemoved,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { BotInstanceSummary } from 'teleport/services/bot/types';
import {
  listBotInstancesError,
  listBotInstancesSuccess,
} from 'teleport/test/helpers/botInstances';

import { BotInstances } from './BotInstances';
import { semverExpand } from './List/BotInstancesList';

const server = setupServer();

beforeEach(() => {
  server.listen();

  jest.useFakeTimers().setSystemTime(new Date('2025-05-19T08:00:00Z'));
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();

  jest.useRealTimers();
});

afterAll(() => server.close());

describe('BotInstances', () => {
  it('Shows an empty state', async () => {
    server.use(
      listBotInstancesSuccess({
        bot_instances: [],
        next_page_token: '',
      })
    );

    render(<BotInstances />, { wrapper: Wrapper });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('No active instances found')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Bot instances are ephemeral, and disappear once all issued credentials have expired.'
      )
    ).toBeInTheDocument();
  });

  it('Shows an error state', async () => {
    server.use(listBotInstancesError(500));

    render(<BotInstances />, { wrapper: Wrapper });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(
      screen.getByText('Error: 500', { exact: false })
    ).toBeInTheDocument();
  });

  it('Shows a list', async () => {
    server.use(
      listBotInstancesSuccess({
        bot_instances: [
          {
            bot_name: 'test-bot-1',
            instance_id: '5e885c66-1af3-4a36-987d-a604d8ee49d2',
            active_at_latest: '2025-05-19T07:32:00Z',
            host_name_latest: 'test-hostname',
            join_method_latest: 'test-join-method',
            version_latest: '1.0.0-dev-a12b3c',
          },
        ],
        next_page_token: '',
      })
    );

    render(<BotInstances />, { wrapper: Wrapper });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('test-bot-1')).toBeInTheDocument();
    expect(screen.getByText('5e885c6')).toBeInTheDocument();
    expect(screen.getByText('28 minutes ago')).toBeInTheDocument();
    expect(screen.getByText('test-hostname')).toBeInTheDocument();
    expect(screen.getByText('test-join-method')).toBeInTheDocument();
    expect(screen.getByText('v1.0.0-dev-a12b3c')).toBeInTheDocument();
  });

  it('Allows paging', async () => {
    const instances = Array.from({ length: 21 }, (_, i): BotInstanceSummary => {
      const num = i.toString().padStart(2, '0');
      return {
        bot_name: `test-bot-${num}`,
        instance_id: `000000${num}-0000-4000-0000-000000000000`,
        active_at_latest: `2025-05-19T07:32:${num}Z`,
        host_name_latest: 'test-hostname',
        join_method_latest: 'test-join-method',
        version_latest: `1.0.${num}-dev-a12b3c`,
      };
    });

    server.use(
      listBotInstancesSuccess({
        bot_instances: instances,
        next_page_token: '',
      })
    );

    render(<BotInstances />, { wrapper: Wrapper });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(
      screen.getAllByText(
        (_, element) => element?.textContent === 'Showing 1 - 20 of 21'
      ).length > 0
    ).toBeTruthy();

    expect(screen.getByText('test-bot-00')).toBeInTheDocument();
    expect(screen.getByText('test-bot-19')).toBeInTheDocument();

    const nextButtons = screen.getAllByTitle('Next page');
    fireEvent.click(nextButtons[0]);

    expect(
      screen.getAllByText(
        (_, element) => element?.textContent === 'Showing 21 - 21 of 21'
      ).length > 0
    ).toBeTruthy();

    expect(screen.getByText('test-bot-20')).toBeInTheDocument();

    const prevButtons = screen.getAllByTitle('Previous page');
    fireEvent.click(prevButtons[0]);

    expect(screen.getByText('test-bot-00')).toBeInTheDocument();
    expect(screen.getByText('test-bot-19')).toBeInTheDocument();
  });

  describe('Allows sorting', () => {
    it.each`
      name             | dataPrefix            | header
      ${'bot name'}    | ${'test-bot'}         | ${'Bot'}
      ${'join method'} | ${'test-join-method'} | ${'Method'}
      ${'hostname'}    | ${'test-hostname'}    | ${'Version (tbot)'}
    `('by $name', async ({ dataPrefix, header }) => {
      const instances = Array.from(
        { length: 3 },
        (_, i): BotInstanceSummary => {
          const num = i.toString().padStart(2, '0');
          return {
            bot_name: `test-bot-${num}`,
            instance_id: `000000${num}-0000-4000-0000-000000000000`,
            active_at_latest: `2025-05-19T07:32:${num}Z`,
            host_name_latest: `test-hostname-${num}`,
            join_method_latest: `test-join-method-${num}`,
            version_latest: `1.0.${num}-dev-a12b3c`,
          };
        }
      );

      server.use(
        listBotInstancesSuccess({
          bot_instances: instances,
          next_page_token: '',
        })
      );

      render(<BotInstances />, { wrapper: Wrapper });

      await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

      let row1 = screen.getByText(`${dataPrefix}-00`);
      let row2 = screen.getByText(`${dataPrefix}-01`);
      let row3 = screen.getByText(`${dataPrefix}-02`);

      expect(row1.compareDocumentPosition(row2)).toBe(
        Node.DOCUMENT_POSITION_FOLLOWING
      );
      expect(row2.compareDocumentPosition(row3)).toBe(
        Node.DOCUMENT_POSITION_FOLLOWING
      );

      fireEvent.click(screen.getByText(header));

      row1 = screen.getByText(`${dataPrefix}-00`);
      row2 = screen.getByText(`${dataPrefix}-01`);
      row3 = screen.getByText(`${dataPrefix}-02`);

      expect(row1.compareDocumentPosition(row2)).toBe(
        Node.DOCUMENT_POSITION_PRECEDING
      );
      expect(row2.compareDocumentPosition(row3)).toBe(
        Node.DOCUMENT_POSITION_PRECEDING
      );
    });

    it('by version (semver)', async () => {
      const instances = Array.from(
        { length: 3 },
        (_, i): BotInstanceSummary => {
          const num = i.toString().padStart(2, '0');
          return {
            bot_name: `test-bot-${num}`,
            instance_id: `000000${num}-0000-4000-0000-000000000000`,
            active_at_latest: `2025-05-19T07:32:${num}Z`,
            host_name_latest: `test-hostname-${num}`,
            join_method_latest: `test-join-method-${num}`,
            version_latest: `1.0.${num}-dev-a12b3c`,
          };
        }
      );

      server.use(
        listBotInstancesSuccess({
          bot_instances: instances,
          next_page_token: '',
        })
      );

      render(<BotInstances />, { wrapper: Wrapper });

      await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

      let row1 = screen.getByText('v1.0.00-dev-a12b3c');
      let row2 = screen.getByText('v1.0.01-dev-a12b3c');
      let row3 = screen.getByText('v1.0.02-dev-a12b3c');

      expect(row1.compareDocumentPosition(row2)).toBe(
        Node.DOCUMENT_POSITION_FOLLOWING
      );
      expect(row2.compareDocumentPosition(row3)).toBe(
        Node.DOCUMENT_POSITION_FOLLOWING
      );

      fireEvent.click(screen.getByText('Version (tbot)'));

      row1 = screen.getByText('v1.0.00-dev-a12b3c');
      row2 = screen.getByText('v1.0.01-dev-a12b3c');
      row3 = screen.getByText('v1.0.02-dev-a12b3c');

      expect(row1.compareDocumentPosition(row2)).toBe(
        Node.DOCUMENT_POSITION_PRECEDING
      );
      expect(row2.compareDocumentPosition(row3)).toBe(
        Node.DOCUMENT_POSITION_PRECEDING
      );
    });

    it('by last active', async () => {
      const instances = Array.from(
        { length: 3 },
        (_, i): BotInstanceSummary => {
          const num = i.toString().padStart(2, '0');
          return {
            bot_name: `test-bot-${num}`,
            instance_id: `000000${num}-0000-4000-0000-000000000000`,
            active_at_latest: `2025-05-19T07:${32 + i}:00Z`,
            host_name_latest: `test-hostname-${num}`,
            join_method_latest: `test-join-method-${num}`,
            version_latest: `1.0.${num}-dev-a12b3c`,
          };
        }
      );

      server.use(
        listBotInstancesSuccess({
          bot_instances: instances,
          next_page_token: '',
        })
      );

      render(<BotInstances />, { wrapper: Wrapper });

      await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

      let row1 = screen.getByText('28 minutes ago');
      let row2 = screen.getByText('27 minutes ago');
      let row3 = screen.getByText('26 minutes ago');

      expect(row1.compareDocumentPosition(row2)).toBe(
        Node.DOCUMENT_POSITION_FOLLOWING
      );
      expect(row2.compareDocumentPosition(row3)).toBe(
        Node.DOCUMENT_POSITION_FOLLOWING
      );

      fireEvent.click(screen.getByText('Version (tbot)'));

      row1 = screen.getByText('28 minutes ago');
      row2 = screen.getByText('27 minutes ago');
      row3 = screen.getByText('26 minutes ago');

      expect(row1.compareDocumentPosition(row2)).toBe(
        Node.DOCUMENT_POSITION_PRECEDING
      );
      expect(row2.compareDocumentPosition(row3)).toBe(
        Node.DOCUMENT_POSITION_PRECEDING
      );
    });
  });

  describe('Allows filtering (search)', () => {
    it.each`
      name             | query                                     | elementText
      ${'bot name'}    | ${'test-bot-01'}                          | ${'test-bot-01'}
      ${'instance id'} | ${'00000020-0000-4000-0000-000000000000'} | ${'0000002'}
      ${'last active'} | ${'27 minutes'}                           | ${'27 minutes ago'}
      ${'hostname'}    | ${'test-hostname-01'}                     | ${'test-hostname-01'}
      ${'join method'} | ${'test-join-method-02'}                  | ${'test-join-method-02'}
      ${'version'}     | ${'1.0.01'}                               | ${'v1.0.01-dev-a12b3c'}
    `('by $name', async ({ query, elementText }) => {
      const instances = Array.from(
        { length: 3 },
        (_, i): BotInstanceSummary => {
          const num = i.toString().padStart(2, '0');
          return {
            bot_name: `test-bot-${num}`,
            instance_id: `00000${num}0-0000-4000-0000-000000000000`,
            active_at_latest: `2025-05-19T07:${32 + i}:00Z`,
            host_name_latest: `test-hostname-${num}`,
            join_method_latest: `test-join-method-${num}`,
            version_latest: `1.0.${num}-dev-a12b3c`,
          };
        }
      );

      server.use(
        listBotInstancesSuccess({
          bot_instances: instances,
          next_page_token: '',
        })
      );

      render(<BotInstances />, { wrapper: Wrapper });

      await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

      jest.useRealTimers(); // Required as userEvent.type() uses setTimeout internally

      const search = screen.getByPlaceholderText('Search...');
      await userEvent.type(search, query);
      await userEvent.type(search, '{enter}');

      expect(screen.getByText(elementText)).toBeInTheDocument();

      expect(
        screen.getAllByText(
          (_, element) => element?.textContent === 'Showing 1 - 1 of 1'
        ).length > 0
      ).toBeTruthy();
    });
  });
});

function Wrapper({ children }: PropsWithChildren) {
  return (
    <MemoryRouter>
      <QueryClientProvider client={testQueryClient}>
        <ConfiguredThemeProvider theme={darkTheme}>
          <InfoGuidePanelProvider data-testid="blah">
            {children}
          </InfoGuidePanelProvider>
        </ConfiguredThemeProvider>
      </QueryClientProvider>
    </MemoryRouter>
  );
}

describe('semverExpand', () => {
  it.each`
    input                 | expected
    ${''}                 | ${'000000.000000.000000-Z+Z'}
    ${'1'}                | ${'000001.000000.000000-Z+Z'}
    ${'1.1'}              | ${'000001.000001.000000-Z+Z'}
    ${'1.1.1'}            | ${'000001.000001.000001-Z+Z'}
    ${'1.1.1-dev'}        | ${'000001.000001.000001-dev+Z'}
    ${'1.1.1-dev+a1b2c3'} | ${'000001.000001.000001-dev+a1b2c3'}
  `('$input', ({ input, expected }) => {
    expect(semverExpand(input)).toBe(expected);
  });
});
