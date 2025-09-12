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
  waitFor,
  waitForElementToBeRemoved,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import { listWorkloadIdentities } from 'teleport/services/workloadIdentity/workloadIdentity';
import {
  listWorkloadIdentitiesError,
  listWorkloadIdentitiesSuccess,
} from 'teleport/test/helpers/workloadIdentities';

import { ContextProvider } from '..';
import { WorkloadIdentities } from './WorkloadIdentities';

jest.mock('teleport/services/workloadIdentity/workloadIdentity', () => {
  const actual = jest.requireActual(
    'teleport/services/workloadIdentity/workloadIdentity'
  );
  return {
    listWorkloadIdentities: jest.fn((...all) => {
      return actual.listWorkloadIdentities(...all);
    }),
  };
});

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

describe('WorkloadIdentities', () => {
  it('Shows an empty state', async () => {
    server.use(
      listWorkloadIdentitiesSuccess({
        items: [],
        next_page_token: '',
      })
    );

    render(<WorkloadIdentities />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('What is Workload Identity')).toBeInTheDocument();
  });

  it('Shows an error state', async () => {
    server.use(listWorkloadIdentitiesError(500, 'server error'));

    render(<WorkloadIdentities />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('server error')).toBeInTheDocument();
  });

  it('Shows an unsupported sort error state', async () => {
    server.use(listWorkloadIdentitiesSuccess());

    render(<WorkloadIdentities />, {
      wrapper: makeWrapper(),
    });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    const testErrorMessage =
      'unsupported sort, only name:asc is supported, but got "blah" (desc = true)';
    server.use(listWorkloadIdentitiesError(400, testErrorMessage));

    fireEvent.click(screen.getByText('SPIFFE ID'));

    await waitFor(() => {
      expect(screen.getByText(testErrorMessage)).toBeInTheDocument();
    });

    server.use(listWorkloadIdentitiesSuccess());

    const resetButton = screen.getByText('Reset sort');
    expect(resetButton).toBeInTheDocument();
    fireEvent.click(resetButton);

    await waitFor(() => {
      expect(screen.queryByText(testErrorMessage)).not.toBeInTheDocument();
    });
  });

  it('Shows an unauthorised error state', async () => {
    render(<WorkloadIdentities />, {
      wrapper: makeWrapper(
        makeAcl({
          workloadIdentity: {
            ...defaultAccess,
            list: false,
          },
        })
      ),
    });

    expect(
      screen.getByText(
        'You do not have permission to access Workload Identities. Missing role permissions:',
        { exact: false }
      )
    ).toBeInTheDocument();

    expect(screen.getByText('workload_identity.list')).toBeInTheDocument();
  });

  it('Shows a list', async () => {
    server.use(listWorkloadIdentitiesSuccess());

    render(<WorkloadIdentities />, {
      wrapper: makeWrapper(),
    });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('test-workload-identity-1')).toBeInTheDocument();
    expect(
      screen.getByText('/test/spiffe/abb53fc8-eba6-40a9-8801-221db41f3c21')
    ).toBeInTheDocument();
    expect(screen.getByText('test-label-1: test-value-1')).toBeInTheDocument();
    expect(screen.getByText('test-label-2: test-value-2')).toBeInTheDocument();
    expect(screen.getByText('test-label-3: test-value-3')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.'
      )
    ).toBeInTheDocument();
  });

  it('Allows paging', async () => {
    jest.mocked(listWorkloadIdentities).mockImplementation(
      ({ pageToken }) =>
        new Promise(resolve => {
          resolve({
            items: [
              {
                name: 'test-workload-identity-1',
                spiffe_id: '/test/spiffe/abb53fc8-eba6-40a9-8801-221db41f3c21',
                spiffe_hint:
                  'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
                labels: {
                  'test-label-1': 'test-value-1',
                  'test-label-2': 'test-value-2',
                  'test-label-3': 'test-value-3',
                },
              },
            ],
            next_page_token: pageToken + '.next',
          });
        })
    );

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(0);

    render(<WorkloadIdentities />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    const [nextButton] = screen.getAllByTitle('Next page');

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(1);
    expect(listWorkloadIdentities).toHaveBeenLastCalledWith({
      pageSize: 20,
      pageToken: '',
      searchTerm: '',
      sortField: 'name',
      sortDir: 'ASC',
    });

    await waitFor(() => expect(nextButton).toBeEnabled());
    fireEvent.click(nextButton);

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(2);
    expect(listWorkloadIdentities).toHaveBeenLastCalledWith({
      pageSize: 20,
      pageToken: '.next',
      searchTerm: '',
      sortField: 'name',
      sortDir: 'ASC',
    });

    await waitFor(() => expect(nextButton).toBeEnabled());
    fireEvent.click(nextButton);

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(3);
    expect(listWorkloadIdentities).toHaveBeenLastCalledWith({
      pageSize: 20,
      pageToken: '.next.next',
      searchTerm: '',
      sortField: 'name',
      sortDir: 'ASC',
    });

    const [prevButton] = screen.getAllByTitle('Previous page');

    await waitFor(() => expect(prevButton).toBeEnabled());
    fireEvent.click(prevButton);

    // This page's data will have been cached
    expect(listWorkloadIdentities).toHaveBeenCalledTimes(3);

    await waitFor(() => expect(prevButton).toBeEnabled());
    fireEvent.click(prevButton);

    // This page's data will have been cached
    expect(listWorkloadIdentities).toHaveBeenCalledTimes(3);
  });

  it('Allows filtering (search)', async () => {
    jest.mocked(listWorkloadIdentities).mockImplementation(
      ({ pageToken }) =>
        new Promise(resolve => {
          resolve({
            items: [
              {
                name: 'test-workload-identity-1',
                spiffe_id: '/test/spiffe/abb53fc8-eba6-40a9-8801-221db41f3c21',
                spiffe_hint:
                  'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
                labels: {
                  'test-label-1': 'test-value-1',
                  'test-label-2': 'test-value-2',
                  'test-label-3': 'test-value-3',
                },
              },
            ],
            next_page_token: pageToken + '.next',
          });
        })
    );

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(0);

    render(<WorkloadIdentities />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(1);
    expect(listWorkloadIdentities).toHaveBeenLastCalledWith({
      pageSize: 20,
      pageToken: '',
      searchTerm: '',
      sortField: 'name',
      sortDir: 'ASC',
    });

    const [nextButton] = screen.getAllByTitle('Next page');
    await waitFor(() => expect(nextButton).toBeEnabled());
    fireEvent.click(nextButton);

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(2);
    expect(listWorkloadIdentities).toHaveBeenLastCalledWith({
      pageSize: 20,
      pageToken: '.next',
      searchTerm: '',
      sortField: 'name',
      sortDir: 'ASC',
    });

    const search = screen.getByPlaceholderText('Search...');
    await waitFor(() => expect(search).toBeEnabled());
    await userEvent.type(search, 'test-search-term');
    await userEvent.type(search, '{enter}');

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(3);
    expect(listWorkloadIdentities).toHaveBeenLastCalledWith({
      pageSize: 20,
      pageToken: '', // Search should reset to the first page
      searchTerm: 'test-search-term',
      sortField: 'name',
      sortDir: 'ASC',
    });
  });

  it('Allows sorting', async () => {
    jest.mocked(listWorkloadIdentities).mockImplementation(
      ({ pageToken }) =>
        new Promise(resolve => {
          resolve({
            items: [
              {
                name: 'test-workload-identity-1',
                spiffe_id: '/test/spiffe/abb53fc8-eba6-40a9-8801-221db41f3c21',
                spiffe_hint:
                  'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
                labels: {
                  'test-label-1': 'test-value-1',
                  'test-label-2': 'test-value-2',
                  'test-label-3': 'test-value-3',
                },
              },
            ],
            next_page_token: pageToken,
          });
        })
    );

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(0);

    render(<WorkloadIdentities />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(1);
    expect(listWorkloadIdentities).toHaveBeenLastCalledWith({
      pageSize: 20,
      pageToken: '',
      searchTerm: '',
      sortField: 'name',
      sortDir: 'ASC',
    });

    fireEvent.click(screen.getByText('Name'));

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(2);
    expect(listWorkloadIdentities).toHaveBeenLastCalledWith({
      pageSize: 20,
      pageToken: '',
      searchTerm: '',
      sortField: 'name',
      sortDir: 'DESC',
    });

    fireEvent.click(screen.getByText('SPIFFE ID'));

    expect(listWorkloadIdentities).toHaveBeenCalledTimes(3);
    expect(listWorkloadIdentities).toHaveBeenLastCalledWith({
      pageSize: 20,
      pageToken: '',
      searchTerm: '',
      sortField: 'spiffe_id',
      sortDir: 'ASC',
    });
  });
});

function makeWrapper(
  customAcl: ReturnType<typeof makeAcl> = makeAcl({
    workloadIdentity: {
      list: true,
      create: true,
      edit: true,
      remove: true,
      read: true,
    },
  })
) {
  return ({ children }: PropsWithChildren) => {
    const ctx = createTeleportContext({
      customAcl,
    });
    return (
      <MemoryRouter>
        <QueryClientProvider client={testQueryClient}>
          <ConfiguredThemeProvider theme={darkTheme}>
            <InfoGuidePanelProvider data-testid="blah">
              <ContextProvider ctx={ctx}>{children}</ContextProvider>
            </InfoGuidePanelProvider>
          </ConfiguredThemeProvider>
        </QueryClientProvider>
      </MemoryRouter>
    );
  };
}
