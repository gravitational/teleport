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

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  render,
  screen,
  testQueryClient,
  waitForElementToBeRemoved,
} from 'design/utils/testing';

import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  listBotInstancesError,
  listBotInstancesSuccess,
} from 'teleport/test/helpers/botInstances';

import { InstancesPanel } from './InstancesPanel';

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

describe('InstancesPanel', () => {
  it('should show a fetch error state', async () => {
    withFetchError();
    render(<InstancesPanel botName="test-bot" />, {
      wrapper: makeWrapper(),
    });
    await waitForLoading();

    expect(screen.getByText('something went wrong')).toBeInTheDocument();
  });

  it('should show a no permissions state', async () => {
    withFetchError();
    render(<InstancesPanel botName="test-bot" />, {
      wrapper: makeWrapper({
        customAcl: makeAcl({
          botInstances: {
            ...defaultAccess,
            list: false,
          },
        }),
      }),
    });

    expect(
      screen.getByText('You do not have permission to view bot instances.', {
        exact: false,
      })
    ).toBeInTheDocument();
  });

  it('renders instance items', async () => {
    withFetchSuccess();

    render(<InstancesPanel botName="test-bot" />, {
      wrapper: makeWrapper(),
    });
    await waitForLoading();

    expect(
      screen.getByText('c11250e0-00c2-4f52-bcdf-b367f80b9461')
    ).toBeInTheDocument();
  });
});

const waitForLoading = async () => {
  await waitForElementToBeRemoved(() =>
    screen.queryByTestId('loading-instances')
  );
};

function withFetchSuccess() {
  server.use(
    listBotInstancesSuccess(
      {
        bot_instances: [
          {
            bot_name: 'ansible-worker',
            instance_id: 'c11250e0-00c2-4f52-bcdf-b367f80b9461',
            active_at_latest: '2025-07-22T10:54:00Z',
            host_name_latest: 'svr-lon-01-ab23cd',
            join_method_latest: 'github',
            os_latest: 'linux',
            version_latest: '4.4.16',
          },
        ],
        next_page_token: '',
      },
      'v1'
    )
  );
}

function withFetchError() {
  server.use(listBotInstancesError(500, 'something went wrong'));
}

function makeWrapper(options?: { customAcl?: ReturnType<typeof makeAcl> }) {
  const {
    customAcl = makeAcl({
      botInstances: {
        ...defaultAccess,
        list: true,
      },
    }),
  } = options ?? {};
  return ({ children }: PropsWithChildren) => {
    const ctx = createTeleportContext({
      customAcl,
    });
    return (
      <QueryClientProvider client={testQueryClient}>
        <ConfiguredThemeProvider theme={darkTheme}>
          <ContextProvider ctx={ctx}>{children}</ContextProvider>
        </ConfiguredThemeProvider>
      </QueryClientProvider>
    );
  };
}
