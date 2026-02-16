import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { MemoryRouter } from 'react-router';

import {
  createDeferredResponse,
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import { createQueryHook } from 'teleport/services/queryHelpers';

import { PaginatedList } from './PaginatedList';

interface TestItem {
  id: string;
  name: string;
}

interface TestResponse {
  items: TestItem[];
  nextKey: string;
}

interface TestVariables {
  startKey?: string;
}

const TEST_API_URL = '/v1/test/items';

async function listTestItems({
  startKey,
}: TestVariables): Promise<TestResponse> {
  const url = startKey ? `${TEST_API_URL}?startKey=${startKey}` : TEST_API_URL;
  const res = await fetch(url);

  if (!res.ok) {
    throw new Error('Server error');
  }

  return res.json();
}

const { useInfiniteQuery: useInfiniteTestItems } = createQueryHook(
  ['test', 'items'],
  listTestItems,
  (pageParam: string, variables: TestVariables) => ({
    ...variables,
    startKey: pageParam,
  })
);

function TestComponent() {
  const query = useInfiniteTestItems(
    {},
    {
      initialPageParam: '',
      getNextPageParam: lastPage => lastPage.nextKey || undefined,
    }
  );

  return (
    <PaginatedList
      query={query}
      rowRenderer={(item: TestItem) => (
        <div key={item.id} data-testid={`item-${item.id}`}>
          {item.name}
        </div>
      )}
    />
  );
}

const server = setupServer();

beforeAll(() => {
  server.listen();
});

afterEach(() => {
  server.resetHandlers();
  testQueryClient.clear();
});

afterAll(() => server.close());

test('renders loading state', async () => {
  const deferred = createDeferredResponse<TestResponse>({
    items: [],
    nextKey: '',
  });

  server.use(http.get(TEST_API_URL, deferred.handler));

  render(
    <MemoryRouter>
      <TestComponent />
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(screen.getByTestId('indicator')).toBeInTheDocument();
  });

  deferred.resolve();

  await waitFor(() => {
    expect(screen.queryByTestId('indicator')).not.toBeInTheDocument();
  });
});

test('renders error state', async () => {
  server.use(
    http.get(TEST_API_URL, () => {
      return HttpResponse.json(
        { error: { message: 'Server error' } },
        { status: 500 }
      );
    })
  );

  render(
    <MemoryRouter>
      <TestComponent />
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(screen.getByText('Server error')).toBeInTheDocument();
  });
});

test('renders empty state when no items', async () => {
  server.use(
    http.get(TEST_API_URL, () => {
      return HttpResponse.json({ items: [], nextKey: '' });
    })
  );

  render(
    <MemoryRouter>
      <TestComponent />
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(screen.getByText('No items found.')).toBeInTheDocument();
  });
});

test('renders items from first page', async () => {
  server.use(
    http.get(TEST_API_URL, () => {
      return HttpResponse.json({
        items: [
          { id: '1', name: 'Item 1' },
          { id: '2', name: 'Item 2' },
        ],
        nextKey: '',
      });
    })
  );

  render(
    <MemoryRouter>
      <TestComponent />
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(screen.getByTestId('item-1')).toBeInTheDocument();
  });

  expect(screen.getByTestId('item-2')).toBeInTheDocument();
  expect(screen.getByText('Item 1')).toBeInTheDocument();
  expect(screen.getByText('Item 2')).toBeInTheDocument();
});

test('displays current page number', async () => {
  server.use(
    http.get(TEST_API_URL, () => {
      return HttpResponse.json({
        items: [{ id: '1', name: 'Item 1' }],
        nextKey: '',
      });
    })
  );

  render(
    <MemoryRouter>
      <TestComponent />
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(screen.getByText('Page 1')).toBeInTheDocument();
  });
});

test('previous button is disabled on first page', async () => {
  server.use(
    http.get(TEST_API_URL, () => {
      return HttpResponse.json({
        items: [{ id: '1', name: 'Item 1' }],
        nextKey: '',
      });
    })
  );

  render(
    <MemoryRouter>
      <TestComponent />
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(screen.getByText('Item 1')).toBeInTheDocument();
  });

  const prevButton = screen.getByRole('button', { name: 'Previous page' });

  expect(prevButton).toBeDisabled();
});

test('next button is disabled when no next page available', async () => {
  server.use(
    http.get(TEST_API_URL, () => {
      return HttpResponse.json({
        items: [{ id: '1', name: 'Item 1' }],
        nextKey: '',
      });
    })
  );

  render(
    <MemoryRouter>
      <TestComponent />
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(screen.getByText('Item 1')).toBeInTheDocument();
  });

  const nextButton = screen.getByRole('button', { name: 'Next page' });

  expect(nextButton).toBeDisabled();
});

test('fetches next page when clicking next', async () => {
  let requestCount = 0;

  server.use(
    http.get(TEST_API_URL, ({ request }) => {
      requestCount++;

      return handlePaginatedRequest(new URL(request.url));
    })
  );

  render(
    <MemoryRouter>
      <TestComponent />
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(screen.getByText('Page 1 Item')).toBeInTheDocument();
  });

  expect(screen.getByText('Page 1')).toBeInTheDocument();

  const nextButton = screen.getByRole('button', { name: 'Next page' });

  expect(nextButton).toBeEnabled();

  await userEvent.click(nextButton);

  await waitFor(() => {
    expect(screen.getByText('Page 2 Item')).toBeInTheDocument();
  });

  expect(screen.getByText('Page 2')).toBeInTheDocument();
  expect(requestCount).toBe(2);
});

test('navigates back to previous page without refetching', async () => {
  let requestCount = 0;

  server.use(
    http.get(TEST_API_URL, ({ request }) => {
      requestCount++;

      return handlePaginatedRequest(new URL(request.url));
    })
  );

  render(
    <MemoryRouter>
      <TestComponent />
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(screen.getByText('Page 1 Item')).toBeInTheDocument();
  });

  await userEvent.click(screen.getByRole('button', { name: 'Next page' }));

  await waitFor(() => {
    expect(screen.getByText('Page 2 Item')).toBeInTheDocument();
  });

  const requestCountAfterPage2 = requestCount;

  await userEvent.click(screen.getByRole('button', { name: 'Previous page' }));

  await waitFor(() => {
    expect(screen.getByText('Page 1 Item')).toBeInTheDocument();
  });

  expect(screen.getByText('Page 1')).toBeInTheDocument();

  expect(requestCount).toBe(requestCountAfterPage2);
});

function handlePaginatedRequest(url: URL) {
  const startKey = url.searchParams.get('startKey');

  if (startKey === 'page2') {
    return HttpResponse.json({
      items: [{ id: '2', name: 'Page 2 Item' }],
      nextKey: '',
    });
  }

  return HttpResponse.json({
    items: [{ id: '1', name: 'Page 1 Item' }],
    nextKey: 'page2',
  });
}
