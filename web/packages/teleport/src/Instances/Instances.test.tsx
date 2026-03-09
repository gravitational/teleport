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
import { createMemoryHistory } from 'history';
import { http, HttpResponse } from 'msw';
import { PropsWithChildren } from 'react';
import { MemoryRouter, Route, Router } from 'react-router';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  listInstancesError,
  listInstancesSuccess,
  listOnlyBotInstances,
  listOnlyRegularInstances,
  mockInstances,
} from 'teleport/test/helpers/instances';

import { Instances } from './Instances';

enableMswServer();

beforeAll(() => {
  global.IntersectionObserver = class IntersectionObserver {
    constructor() {}
    disconnect() {}
    observe() {}
    takeRecords() {
      return [];
    }
    unobserve() {}
  } as any;
});

afterEach(async () => {
  await testQueryClient.resetQueries();
  jest.clearAllMocks();
});

it('having no permissions should show correct error', async () => {
  renderComponent({
    customAcl: makeAcl({
      instances: {
        ...defaultAccess,
        list: false,
        read: false,
      },
      botInstances: {
        ...defaultAccess,
        list: false,
        read: false,
      },
    }),
  });

  expect(
    screen.getByText(
      'You do not have permission to view the instance inventory.',
      { exact: false }
    )
  ).toBeInTheDocument();
});

it('having only bot instances permissions should show warning banner', async () => {
  server.use(listOnlyBotInstances);
  renderComponent({
    customAcl: makeAcl({
      instances: {
        ...defaultAccess,
        list: false,
        read: false,
      },
      botInstances: {
        ...defaultAccess,
        list: true,
        read: true,
      },
    }),
  });

  await waitFor(() => {
    expect(
      screen.getByText('You do not have permission to view instances.', {
        exact: false,
      })
    ).toBeInTheDocument();
  });
});

it('having only instances permissions should show warning banner', async () => {
  server.use(listOnlyRegularInstances);
  renderComponent({
    customAcl: makeAcl({
      instances: {
        ...defaultAccess,
        list: true,
        read: true,
      },
      botInstances: {
        ...defaultAccess,
        list: false,
        read: false,
      },
    }),
  });

  await waitFor(() => {
    expect(
      screen.getByText('You do not have permission to view bot instances.', {
        exact: false,
      })
    ).toBeInTheDocument();
  });
});

it('cache still initializing error should show correct error', async () => {
  server.use(
    listInstancesError(
      503,
      'inventory cache is not yet healthy, please try again in a few minutes'
    )
  );
  renderComponent();

  await waitFor(() => {
    expect(
      screen.getByText(
        'The instance inventory is not yet ready to be displayed',
        { exact: false }
      )
    ).toBeInTheDocument();
  });
});

it('listing successfully should show instances', async () => {
  server.use(listInstancesSuccess);
  renderComponent();

  expect(
    await screen.findByText('ip-10-1-1-100.ec2.internal')
  ).toBeInTheDocument();
  expect(screen.getByText('teleport-auth-01')).toBeInTheDocument();
  expect(screen.getByText('app-server-prod')).toBeInTheDocument();
  expect(screen.getByText('github-actions-bot')).toBeInTheDocument();
  expect(screen.getByText('ci-cd-bot')).toBeInTheDocument();
});

it('no instances should show empty state', async () => {
  server.use(
    http.get('/v1/webapi/sites/:clusterId/instances', () => {
      return HttpResponse.json({
        instances: [],
        startKey: '',
      });
    })
  );
  renderComponent();

  await waitFor(() => {
    expect(screen.getByText('No instances found')).toBeInTheDocument();
  });
});

it('search query param in the URL should be populated in the search input', async () => {
  server.use(listInstancesSuccess);

  renderComponent({ initialUrl: cfg.routes.instances + '?query=test-server' });

  const searchInput = screen.getByPlaceholderText(/search/i);
  expect(searchInput).toHaveValue('test-server');
});

it('version filter query param URL should be populated in the version filter control', async () => {
  server.use(listInstancesSuccess);

  renderComponent({
    initialUrl: cfg.routes.instances + '?version_filter=up-to-date',
  });

  const versionButton = screen.getByRole('button', {
    name: /Version \(1\)/i,
  });
  expect(versionButton).toBeInTheDocument();
});

it('selecting a version filter should append the version predicate expression to an existing advanced query', async () => {
  let lastRequestUrl: string;

  server.use(
    http.get('/v1/webapi/sites/:clusterId/instances', ({ request }) => {
      lastRequestUrl = request.url;
      return HttpResponse.json(mockInstances);
    })
  );

  const { user } = renderComponent();

  // Wait for initial load
  await screen.findByText('ip-10-1-1-100.ec2.internal');

  // Switch to advanced search mode
  const advancedToggle = screen.getByRole('checkbox', {
    name: /advanced/i,
  });
  await user.click(advancedToggle);

  // Type in a predicate query
  const searchInput = screen.getByPlaceholderText(/search/i);
  await user.clear(searchInput);
  await user.type(searchInput, 'name == "teleport-auth-01"{Enter}');

  await waitFor(() => {
    expect(lastRequestUrl).toContain('query=');
  });
  {
    const url = new URL(lastRequestUrl);
    expect(url.searchParams.get('query')).toBe('name == "teleport-auth-01"');
  }

  // Select a version filter
  const versionButton = screen.getByRole('button', { name: /Version/i });
  await user.click(versionButton);

  const upToDateOption = screen.getByText('Up-to-date');
  await user.click(upToDateOption);

  const applyButton = screen.getByRole('button', { name: /Apply Filters/i });
  await user.click(applyButton);

  // Verify that the request made combines both predicates
  await waitFor(() => {
    expect(lastRequestUrl).toContain('version');
  });
  {
    const url = new URL(lastRequestUrl);
    expect(url.searchParams.get('query')).toBe(
      '(name == "teleport-auth-01") && (version == "18.2.4")'
    );
  }
}, 15000);

function renderComponent(options?: {
  customAcl?: ReturnType<typeof makeAcl>;
  initialUrl?: string;
}) {
  const user = userEvent.setup();
  const history = createMemoryHistory({
    initialEntries: [options?.initialUrl || cfg.routes.instances],
  });

  return {
    ...render(<Instances />, {
      wrapper: makeWrapper({
        customAcl: options?.customAcl,
        history,
      }),
    }),
    user,
  };
}

function makeWrapper(options: {
  history: ReturnType<typeof createMemoryHistory>;
  customAcl?: ReturnType<typeof makeAcl>;
}) {
  const {
    history,
    customAcl = makeAcl({
      instances: {
        ...defaultAccess,
        list: true,
        read: true,
      },
      botInstances: {
        ...defaultAccess,
        list: true,
        read: true,
      },
    }),
  } = options;

  return ({ children }: PropsWithChildren) => {
    const ctx = createTeleportContext({
      customAcl,
    });

    ctx.storeUser.state.cluster.authVersion = '18.2.4';

    return (
      <MemoryRouter>
        <QueryClientProvider client={testQueryClient}>
          <ConfiguredThemeProvider theme={darkTheme}>
            <ContextProvider ctx={ctx}>
              <Router history={history}>
                <Route path={cfg.routes.instances}>{children}</Route>
              </Router>
            </ContextProvider>
          </ConfiguredThemeProvider>
        </QueryClientProvider>
      </MemoryRouter>
    );
  };
}
