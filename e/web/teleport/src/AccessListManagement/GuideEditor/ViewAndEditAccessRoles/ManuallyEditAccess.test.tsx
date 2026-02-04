import { QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { MemoryRouter } from 'react-router';

import {
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { AccessGraphDemoProvider } from 'e-teleport/Roles/AccessGraphDemoContext';
import { ContextProvider } from 'teleport/index';
import { allAccessAcl, noAccess } from 'teleport/mocks/contexts';
import { Acl } from 'teleport/services/user/types';
import { UserContextProvider } from 'teleport/User';

import { makeHandlers } from '../Preset/TestHelper/mocks';
import { ManuallyEditAccess } from './ManuallyEditAccess';

const server = setupServer();

beforeAll(() => {
  server.listen();
});

beforeEach(() => {
  server.use(...makeHandlers());
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();
  jest.clearAllMocks();
});

afterAll(() => server.close());

const testRoles = [
  'access-standard-acl-preset-ABCD',
  'access-awsic-acl-preset-ABCD',
];

test('render missing permission text when user lacks write role access', async () => {
  const customAcl: Acl = {
    ...allAccessAcl,
    roles: noAccess,
  };

  render(
    <TestProvider customAcl={customAcl}>
      <ManuallyEditAccess roles={testRoles} />
    </TestProvider>
  );

  expect(
    await screen.findByText(/manually edit member access/i)
  ).toBeInTheDocument();
  expect(
    screen.getByText(/do not have permission to edit roles/i)
  ).toBeInTheDocument();
});

test('render table with role names and edit buttons when user has write role access', async () => {
  render(
    <TestProvider>
      <ManuallyEditAccess roles={testRoles} />
    </TestProvider>
  );

  await waitFor(() => {
    expect(
      screen.getByText(/manually edit member access/i)
    ).toBeInTheDocument();
  });
  expect(
    screen.getByText('access-standard-acl-preset-ABCD')
  ).toBeInTheDocument();
  expect(screen.getByText('access-awsic-acl-preset-ABCD')).toBeInTheDocument();
  expect(screen.getAllByRole('button', { name: /edit/i })).toHaveLength(2);
});

test('render error when fetch role fails and retry attempt', async () => {
  const user = userEvent.setup();

  server.use(
    http.get('/v1/webapi/roles/:name', () => {
      return HttpResponse.json(
        { message: 'Failed to fetch role' },
        { status: 500 }
      );
    })
  );

  render(
    <TestProvider>
      <ManuallyEditAccess roles={testRoles} />
    </TestProvider>
  );

  await waitFor(() => {
    expect(
      screen.getByText(/manually edit member access/i)
    ).toBeInTheDocument();
  });

  const editButtons = screen.getAllByRole('button', { name: /edit/i });
  await user.click(editButtons[0]);

  await waitFor(() => {
    expect(screen.getByText(/failed to fetch role/i)).toBeInTheDocument();
  });

  // Test retrying
  server.use(...makeHandlers());

  const retryButton = screen.getByRole('button', { name: /retry/i });
  await user.click(retryButton);

  await waitFor(() => {
    expect(screen.queryByText(/failed to fetch role/i)).not.toBeInTheDocument();
  });
});

test('opens role editor dialog when edit button is clicked', async () => {
  const user = userEvent.setup();

  render(
    <TestProvider>
      <ManuallyEditAccess roles={testRoles} />
    </TestProvider>
  );

  await waitFor(() => {
    expect(
      screen.getByText(/manually edit member access/i)
    ).toBeInTheDocument();
  });

  const editButtons = screen.getAllByRole('button', { name: /edit/i });
  await user.click(editButtons[0]);

  // Test role editor renders
  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: /save changes/i })
    ).toBeInTheDocument();
  });
});

test('renders empty table when no roles provided', async () => {
  render(
    <TestProvider>
      <ManuallyEditAccess roles={[]} />
    </TestProvider>
  );

  await waitFor(() => {
    expect(
      screen.getByText(/manually edit member access/i)
    ).toBeInTheDocument();
  });
  expect(
    screen.queryByRole('button', { name: /edit/i })
  ).not.toBeInTheDocument();
});

function TestProvider({
  children,
  customAcl,
}: {
  children: React.ReactNode;
  customAcl?: Acl;
}) {
  const ctx = createTeleportContextE({ customAcl });

  return (
    <QueryClientProvider client={testQueryClient}>
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <UserContextProvider>
            <AccessGraphDemoProvider>
              <ContextProvider ctx={ctx}>{children}</ContextProvider>
            </AccessGraphDemoProvider>
          </UserContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    </QueryClientProvider>
  );
}
