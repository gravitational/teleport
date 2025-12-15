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

import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { generatePath, MemoryRouter } from 'react-router';

import {
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import type { RecordingType } from 'teleport/services/recordings';
import { KeysEnum } from 'teleport/services/storageService';

import {
  createMockSessionEndEvent,
  getThumbnail,
  MOCK_EVENTS,
  MOCK_THUMBNAIL,
} from './mock';
import { RecordingsList } from './RecordingsList';
import type { RecordingsListState } from './state';
import { Density, ViewMode } from './ViewSwitcher';

const server = setupServer();

beforeAll(() => server.listen());
beforeEach(() => {
  server.use(
    getThumbnail(MOCK_THUMBNAIL),
    http.get(cfg.api.clustersPath, () => {
      return HttpResponse.json([
        {
          name: 'teleport',
          lastConnected: '2025-08-14T14:36:07.976470934Z',
          status: 'online',
          publicURL: '',
          authVersion: '',
          proxyVersion: '',
        },
      ]);
    })
  );
});
afterEach(async () => {
  server.resetHandlers();

  testQueryClient.clear();
});
afterAll(() => server.close());

const defaultState: RecordingsListState = {
  filters: {
    hideNonInteractive: false,
    resources: [],
    types: [],
    users: [],
  },
  page: 0,
  range: {
    from: new Date('2025-01-15T00:00:00Z'),
    to: new Date('2025-01-16T00:00:00Z'),
    isCustom: false,
  },
  search: '',
  sortKey: 'date',
  sortDirection: 'DESC',
};

const mockHandlers = {
  onFilterChange: jest.fn(),
  onPageChange: jest.fn(),
  onSearchChange: jest.fn(),
  onSortChange: jest.fn(),
};

const listRecordingsUrl = generatePath(cfg.api.clusterEventsRecordingsPath, {
  clusterId: 'localhost',
});

function withListRecordings(events = MOCK_EVENTS) {
  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json({
        events,
        startKey: '',
      });
    })
  );
}

function setupTest(state: RecordingsListState = defaultState) {
  const ctx = createTeleportContext();

  mockHandlers.onFilterChange.mockClear();
  mockHandlers.onPageChange.mockClear();
  mockHandlers.onSearchChange.mockClear();
  mockHandlers.onSortChange.mockClear();

  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <RecordingsList
          state={state}
          onFilterChange={mockHandlers.onFilterChange}
          onPageChange={mockHandlers.onPageChange}
          onSearchChange={mockHandlers.onSearchChange}
          onSortChange={mockHandlers.onSortChange}
        />
      </ContextProvider>
    </MemoryRouter>
  );
}

describe('rendering', () => {
  it('renders recordings list with all items', async () => {
    withListRecordings();
    setupTest();

    expect(await screen.findByText('server-01')).toBeInTheDocument();

    expect(screen.getByText('desktop-02')).toBeInTheDocument();
    expect(screen.getByText('database-01')).toBeInTheDocument();
    expect(screen.getByText('k8s-cluster//')).toBeInTheDocument();
    expect(screen.getByText('server-03')).toBeInTheDocument();
  });

  it('renders empty state when no recordings', async () => {
    withListRecordings([]);
    setupTest(defaultState);

    expect(await screen.findByText('No Recordings Found')).toBeInTheDocument();
  });

  it('displays cluster dropdown when there is more than 1 cluster', async () => {
    server.use(
      http.get(cfg.api.clustersPath, () => {
        return HttpResponse.json([
          {
            name: 'teleport',
            lastConnected: '2025-08-14T14:36:07.976470934Z',
            status: 'online',
            publicURL: '',
            authVersion: '',
            proxyVersion: '',
          },
          {
            name: 'other-cluster',
            lastConnected: '2025-08-14T14:36:07.976470934Z',
            status: 'online',
            publicURL: '',
            authVersion: '',
            proxyVersion: '',
          },
        ]);
      })
    );

    withListRecordings();
    setupTest();

    expect(await screen.findByText('Cluster: localhost')).toBeInTheDocument();
  });

  it('displays sort menu with correct default', async () => {
    withListRecordings();
    setupTest();

    const sortButton = await screen.findByRole('button', { name: 'Sort by' });

    expect(sortButton).toBeInTheDocument();
  });
});

describe('filtering', () => {
  it('filters by recording type', async () => {
    const stateWithTypeFilter: RecordingsListState = {
      ...defaultState,
      filters: {
        ...defaultState.filters,
        types: ['ssh' as RecordingType],
      },
    };

    withListRecordings();
    setupTest(stateWithTypeFilter);

    expect(await screen.findByText('server-01')).toBeInTheDocument();

    expect(screen.getByText('server-03')).toBeInTheDocument();
    expect(screen.queryByText('desktop-02')).not.toBeInTheDocument();
    expect(screen.queryByText('database-01')).not.toBeInTheDocument();
    expect(screen.queryByText('k8s-cluster')).not.toBeInTheDocument();
  });

  it('filters by user', async () => {
    const stateWithUserFilter: RecordingsListState = {
      ...defaultState,
      filters: {
        ...defaultState.filters,
        users: ['alice'],
      },
    };

    withListRecordings();
    setupTest(stateWithUserFilter);

    expect(await screen.findByText('server-01')).toBeInTheDocument();

    expect(screen.getByText('k8s-cluster//')).toBeInTheDocument();
    expect(screen.queryByText('desktop-02')).not.toBeInTheDocument();
    expect(screen.queryByText('database-01')).not.toBeInTheDocument();
    expect(screen.queryByText('server-03')).not.toBeInTheDocument();
  });

  it('filters by resource/hostname', async () => {
    const stateWithResourceFilter: RecordingsListState = {
      ...defaultState,
      filters: {
        ...defaultState.filters,
        resources: ['server-01', 'server-03'],
      },
    };

    withListRecordings();
    setupTest(stateWithResourceFilter);

    expect(await screen.findByText('server-01')).toBeInTheDocument();

    expect(screen.getByText('server-03')).toBeInTheDocument();
    expect(screen.queryByText('desktop-02')).not.toBeInTheDocument();
    expect(screen.queryByText('database-01')).not.toBeInTheDocument();
    expect(screen.queryByText('k8s-cluster')).not.toBeInTheDocument();
  });

  it('applies multiple filters simultaneously', async () => {
    const stateWithMultipleFilters: RecordingsListState = {
      ...defaultState,
      filters: {
        hideNonInteractive: false,
        types: ['ssh' as RecordingType],
        users: ['alice'],
        resources: [],
      },
    };

    withListRecordings();
    setupTest(stateWithMultipleFilters);

    expect(await screen.findByText('server-01')).toBeInTheDocument();

    expect(screen.queryByText('desktop-02')).not.toBeInTheDocument();
    expect(screen.queryByText('database-01')).not.toBeInTheDocument();
    expect(screen.queryByText('k8s-cluster')).not.toBeInTheDocument();
    expect(screen.queryByText('server-03')).not.toBeInTheDocument();
  });

  it('calls onFilterChange when filter is changed', async () => {
    withListRecordings();
    setupTest();

    const typesButton = await screen.findByRole('button', { name: /Types/i });
    await userEvent.click(typesButton);

    const sshOption = await screen.findByText('SSH');
    await userEvent.click(sshOption);

    const applyFiltersButton = screen.getByRole('button', {
      name: /Apply filters/i,
    });
    await userEvent.click(applyFiltersButton);

    expect(mockHandlers.onFilterChange).toHaveBeenCalledWith('types', ['ssh']);
  });
});

describe('sorting', () => {
  it('sorts by date descending by default', async () => {
    withListRecordings();
    setupTest();

    expect(await screen.findByText('server-01')).toBeInTheDocument();

    const firstItemInGrid = screen.getAllByTestId('recording-item')[0];

    expect(firstItemInGrid).toHaveTextContent('server-01');
  });

  it('sorts by type when specified', async () => {
    const stateWithTypeSort: RecordingsListState = {
      ...defaultState,
      sortKey: 'type',
      sortDirection: 'ASC',
    };

    withListRecordings();
    setupTest(stateWithTypeSort);

    expect(await screen.findByText('server-01')).toBeInTheDocument();

    const firstItemInGrid = screen.getAllByTestId('recording-item')[0];

    expect(firstItemInGrid).toHaveTextContent('database-01');
  });

  it('calls onSortChange when sort is changed', async () => {
    withListRecordings();
    setupTest();

    const sortButton = await screen.findByRole('button', { name: 'Sort by' });
    await userEvent.click(sortButton);

    const typeOption = await screen.findByText('Type');
    await userEvent.click(typeOption);

    expect(mockHandlers.onSortChange).toHaveBeenCalledWith('type', 'DESC');

    const sortDirectionButton = screen.getByRole('button', {
      name: 'Sort direction',
    });
    await userEvent.click(sortDirectionButton);

    // The sort direction is still the default (Date, DESC), so we expect
    // the direction change to be called with Date, ASC.
    expect(mockHandlers.onSortChange).toHaveBeenCalledWith('date', 'ASC');
  });
});

describe('pagination', () => {
  it('displays pagination controls', async () => {
    const manyRecordings = Array.from({ length: 60 }, (_, i) =>
      createMockSessionEndEvent({
        sid: `session-${i.toString().padStart(3, '0')}`,
        server_hostname: `server-${i.toString().padStart(2, '0')}`,
      })
    );

    withListRecordings(manyRecordings);
    setupTest(defaultState);

    const paginationIndicator = await screen.findByTestId(
      'recordings-pagination-indicator'
    );

    expect(paginationIndicator).toHaveTextContent('Showing 1 - 50 of 60');
  });

  it('fetches more data when there are more results', async () => {
    server.use(
      http.get(listRecordingsUrl, ({ request }) => {
        const url = new URL(request.url);
        const startKey = url.searchParams.get('startKey');

        if (startKey === 'next-page-key') {
          const events = Array.from({ length: 10 }, (_, i) =>
            createMockSessionEndEvent({
              sid: `session-page2-${i.toString().padStart(2, '0')}`,
              server_hostname: `server-page2-${i.toString().padStart(2, '0')}`,
            })
          );

          return HttpResponse.json({
            events,
            startKey: '',
          });
        }

        const events = Array.from({ length: 20 }, (_, i) =>
          createMockSessionEndEvent({
            sid: `session-page1-${i.toString().padStart(2, '0')}`,
            server_hostname: `server-page1-${i.toString().padStart(2, '0')}`,
          })
        );

        return HttpResponse.json({
          events,
          startKey: 'next-page-key',
        });
      })
    );

    setupTest(defaultState);

    await screen.findByText('server-page1-00');

    const paginationIndicator = screen.getByTestId(
      'recordings-pagination-indicator'
    );

    expect(paginationIndicator).toHaveTextContent('Showing 1 - 20 of 20');

    const nextPageButton = screen.getByRole('button', {
      name: 'Next page',
    });

    expect(nextPageButton).toBeDisabled();

    const fetchMoreButton = await screen.findByRole('button', {
      name: 'Fetch More',
    });

    expect(fetchMoreButton).toBeInTheDocument();
    expect(fetchMoreButton).toBeEnabled();

    await userEvent.click(fetchMoreButton);

    expect(paginationIndicator).toHaveTextContent('Showing 1 - 30 of 30');

    expect(
      screen.queryByRole('button', { name: 'Fetch More' })
    ).not.toBeInTheDocument();
  });

  it('shows an error correctly if fetching more results fails', async () => {
    server.use(
      http.get(listRecordingsUrl, ({ request }) => {
        const url = new URL(request.url);
        const startKey = url.searchParams.get('startKey');

        if (startKey === 'next-page-key') {
          return HttpResponse.json(
            { error: { message: 'Failed to fetch more results' } },
            { status: 500 }
          );
        }

        return HttpResponse.json({
          events: MOCK_EVENTS,
          startKey: 'next-page-key',
        });
      })
    );

    setupTest(defaultState);

    await screen.findByText('server-01');

    const paginationIndicator = screen.getByTestId(
      'recordings-pagination-indicator'
    );

    expect(paginationIndicator).toHaveTextContent('Showing 1 - 5 of 5');

    const fetchMoreButton = await screen.findByRole('button', {
      name: 'Fetch More',
    });

    expect(fetchMoreButton).toBeInTheDocument();
    expect(fetchMoreButton).toBeEnabled();

    await userEvent.click(fetchMoreButton);

    expect(paginationIndicator).toHaveTextContent('An error occurred');

    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  });

  it('navigates to next page', async () => {
    const manyRecordings = Array.from({ length: 60 }, (_, i) =>
      createMockSessionEndEvent({
        sid: `session-${i.toString().padStart(3, '0')}`,
        server_hostname: `server-${i.toString().padStart(2, '0')}`,
      })
    );

    withListRecordings(manyRecordings);
    setupTest(defaultState);

    const nextButton = await screen.findByRole('button', {
      name: 'Next page',
    });
    await userEvent.click(nextButton);

    expect(mockHandlers.onPageChange).toHaveBeenCalledWith(1);
  });

  it('displays correct range on page 2', async () => {
    const manyRecordings = Array.from({ length: 60 }, (_, i) =>
      createMockSessionEndEvent({
        sid: `session-${i.toString().padStart(3, '0')}`,
        server_hostname: `server-${i.toString().padStart(2, '0')}`,
      })
    );

    const stateOnPage2: RecordingsListState = {
      ...defaultState,
      page: 1,
    };

    withListRecordings(manyRecordings);
    setupTest(stateOnPage2);

    await screen.findByTestId('recordings-pagination-indicator');

    const paginationIndicator = screen.getByTestId(
      'recordings-pagination-indicator'
    );

    expect(paginationIndicator).toHaveTextContent('Showing 51 - 60 of 60');
  });

  it('disables next button on last page', async () => {
    const stateOnLastPage: RecordingsListState = {
      ...defaultState,
      page: 0,
    };

    withListRecordings();
    setupTest(stateOnLastPage);

    const nextButton = await screen.findByRole('button', {
      name: 'Next page',
    });

    expect(nextButton).toBeDisabled();
  });

  it('disables previous button on first page', async () => {
    withListRecordings();
    setupTest();

    const prevButton = await screen.findByRole('button', {
      name: 'Previous page',
    });

    expect(prevButton).toBeDisabled();
  });
});

describe('search', () => {
  it('displays search input field', async () => {
    withListRecordings();
    setupTest();

    const searchInput = await screen.findByPlaceholderText('Search...');

    expect(searchInput).toBeInTheDocument();
  });

  it('filters recordings based on search', async () => {
    const stateWithSearch: RecordingsListState = {
      ...defaultState,
      search: 'server-01',
    };

    withListRecordings();
    setupTest(stateWithSearch);

    expect(await screen.findByText('server-01')).toBeInTheDocument();

    expect(screen.queryByText('desktop-02')).not.toBeInTheDocument();
    expect(screen.queryByText('database-01')).not.toBeInTheDocument();
    expect(screen.queryByText('k8s-cluster')).not.toBeInTheDocument();
    expect(screen.queryByText('server-03')).not.toBeInTheDocument();
  });

  it('calls onSearchChange when enter is pressed', async () => {
    withListRecordings();
    setupTest();

    const searchInput = await screen.findByPlaceholderText('Search...');

    await userEvent.clear(searchInput);
    await userEvent.type(searchInput, 'test search');
    await userEvent.keyboard('{Enter}');

    expect(mockHandlers.onSearchChange).toHaveBeenCalled();
    expect(mockHandlers.onSearchChange).toHaveBeenLastCalledWith('test search');
  });

  it('shows correct search value in input', async () => {
    const stateWithSearch: RecordingsListState = {
      ...defaultState,
      search: 'existing search',
    };

    withListRecordings();
    setupTest(stateWithSearch);

    const searchInput = await screen.findByPlaceholderText('Search...');

    expect(searchInput).toHaveValue('existing search');
  });

  it('applies search with other filters simultaneously', async () => {
    const stateWithSearchAndFilters: RecordingsListState = {
      ...defaultState,
      filters: {
        ...defaultState.filters,
        types: ['ssh' as RecordingType],
      },
      search: '03',
    };

    withListRecordings();
    setupTest(stateWithSearchAndFilters);

    expect(await screen.findByText('server-03')).toBeInTheDocument();

    expect(screen.queryByText('server-01')).not.toBeInTheDocument();
    expect(screen.queryByText('desktop-02')).not.toBeInTheDocument();
    expect(screen.queryByText('database-01')).not.toBeInTheDocument();
    expect(screen.queryByText('k8s-cluster')).not.toBeInTheDocument();
  });

  it('handles empty search value', async () => {
    const stateWithEmptySearch: RecordingsListState = {
      ...defaultState,
      search: '',
    };

    withListRecordings();
    setupTest(stateWithEmptySearch);

    expect(await screen.findByText('server-01')).toBeInTheDocument();

    expect(screen.getByText('desktop-02')).toBeInTheDocument();
    expect(screen.getByText('database-01')).toBeInTheDocument();
    expect(screen.getByText('k8s-cluster//')).toBeInTheDocument();
    expect(screen.getByText('server-03')).toBeInTheDocument();
  });

  it('clears search when user clears the input', async () => {
    const stateWithSearch: RecordingsListState = {
      ...defaultState,
      search: 'some search',
    };

    withListRecordings();
    setupTest(stateWithSearch);

    const searchInput = await screen.findByPlaceholderText('Search...');

    await userEvent.clear(searchInput);
    await userEvent.type(searchInput, '{Enter}');

    expect(mockHandlers.onSearchChange).toHaveBeenCalledWith('');
  });
});

describe('view modes', () => {
  it('switches between card and list view', async () => {
    withListRecordings();
    setupTest();

    expect(
      await screen.findByLabelText('View Mode Switch')
    ).toBeInTheDocument();

    const cardOption = screen.getByRole('radio', {
      name: 'Card View',
    });
    await userEvent.click(cardOption);

    expect(localStorage.getItem(KeysEnum.SESSION_RECORDINGS_VIEW_MODE)).toBe(
      JSON.stringify(ViewMode.Card)
    );

    const listOption = screen.getByRole('radio', {
      name: 'List View',
    });
    await userEvent.click(listOption);

    expect(localStorage.getItem(KeysEnum.SESSION_RECORDINGS_VIEW_MODE)).toBe(
      JSON.stringify(ViewMode.List)
    );
  });

  it('switches between comfortable and compact density', async () => {
    withListRecordings();
    setupTest();

    expect(await screen.findByLabelText('Density Switch')).toBeInTheDocument();

    const compactOption = screen.getByRole('radio', {
      name: 'Compact View',
    });
    await userEvent.click(compactOption);

    expect(localStorage.getItem(KeysEnum.SESSION_RECORDINGS_DENSITY)).toBe(
      JSON.stringify(Density.Compact)
    );

    const comfortableOption = screen.getByRole('radio', {
      name: 'Comfortable View',
    });
    await userEvent.click(comfortableOption);

    expect(localStorage.getItem(KeysEnum.SESSION_RECORDINGS_DENSITY)).toBe(
      JSON.stringify(Density.Comfortable)
    );
  });
});
