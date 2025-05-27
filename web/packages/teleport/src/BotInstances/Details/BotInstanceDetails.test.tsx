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
import { MemoryRouter, useHistory } from 'react-router';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import {
  fireEvent,
  render,
  screen,
  testQueryClient,
  waitForElementToBeRemoved,
} from 'design/utils/testing';

import {
  getBotInstanceError,
  getBotInstanceSuccess,
} from 'teleport/test/helpers/botInstances';

import { BotInstanceDetails } from './BotInstanceDetails';

jest.mock('react-router', () => {
  const actual = jest.requireActual('react-router');
  return {
    ...actual,
    useHistory: jest.fn(),
    useParams: jest.fn(() => ({
      botName: 'test-bot-name',
      instanceId: '4fa10e68-f2e0-4cf9-ad5b-1458febcd827',
    })),
  };
});

jest.mock('shared/components/TextEditor/TextEditor', () => {
  return {
    __esModule: true,
    default: MockTextEditor,
  };
});

jest.mock('design/utils/copyToClipboard', () => {
  return {
    __esModule: true,
    copyToClipboard: jest.fn(),
  };
});

const server = setupServer();

beforeEach(() => {
  server.listen();
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();

  jest.clearAllMocks();
});

afterAll(() => server.close());

const withSuccessResponse = () => {
  server.use(
    getBotInstanceSuccess({
      bot_instance: {
        spec: {
          instance_id: '4fa10e68-f2e0-4cf9-ad5b-1458febcd827',
        },
      },
      yaml: 'kind: bot_instance\nversion: v1\n',
    })
  );
};

const withErrorResponse = () => {
  server.use(getBotInstanceError(500));
};

describe('BotIntanceDetails', () => {
  it('Allows back navigation', async () => {
    const goBack = jest.fn();
    jest.mocked(useHistory).mockImplementation(
      () =>
        ({
          goBack,
        }) as unknown as ReturnType<typeof useHistory>
    );

    withSuccessResponse();

    render(<BotInstanceDetails />, { wrapper: Wrapper });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    const backButton = screen.getByLabelText('back');
    fireEvent.click(backButton);

    expect(goBack).toHaveBeenCalledTimes(1);
  });

  it('Shows the short instance id', async () => {
    withSuccessResponse();

    render(<BotInstanceDetails />, { wrapper: Wrapper });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('4fa10e6')).toBeInTheDocument();
  });

  it('Allows the full instance id to be copied', async () => {
    withSuccessResponse();

    render(<BotInstanceDetails />, { wrapper: Wrapper });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    const copyButton = screen.getByLabelText('copy');
    fireEvent.click(copyButton);

    expect(copyToClipboard).toHaveBeenCalledTimes(1);
    expect(copyToClipboard).toHaveBeenLastCalledWith(
      '4fa10e68-f2e0-4cf9-ad5b-1458febcd827'
    );
  });

  it('Shows a docs link', async () => {
    const onClick = jest.fn(e => {
      e.preventDefault();
    });

    withSuccessResponse();

    render(<BotInstanceDetails onDocsLinkClickedForTesting={onClick} />, {
      wrapper: Wrapper,
    });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    const docsButton = screen.getByText('View Documentation');
    fireEvent.click(docsButton);

    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('Shows full yaml', async () => {
    withSuccessResponse();

    render(<BotInstanceDetails />, { wrapper: Wrapper });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(
      screen.getByText('kind: bot_instance version: v1')
    ).toBeInTheDocument();
  });

  it('Shows an error', async () => {
    withErrorResponse();

    render(<BotInstanceDetails />, { wrapper: Wrapper });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(
      screen.getByText('Error: 500', { exact: false })
    ).toBeInTheDocument();
  });
});

function Wrapper(props: PropsWithChildren) {
  return (
    <MemoryRouter>
      <QueryClientProvider client={testQueryClient}>
        <ConfiguredThemeProvider theme={darkTheme}>
          {props.children}
        </ConfiguredThemeProvider>
      </QueryClientProvider>
    </MemoryRouter>
  );
}

function MockTextEditor(props: { data?: [{ content: string }] }) {
  return (
    <div data-testid="mock-text-editor">
      {props.data?.map(d => <div key={d.content}>{d.content}</div>)}
    </div>
  );
}
