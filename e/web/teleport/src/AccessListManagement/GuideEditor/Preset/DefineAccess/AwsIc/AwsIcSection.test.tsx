import { setupServer } from 'msw/node';

import {
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import {
  appsWithAllMatchingPermissionSet,
  appsWithoutPermissionSets,
  fetchUnifiedResources,
  makeHandlers,
} from '../../TestHelper/mocks';
import { ProviderWithQuery } from '../../TestHelper/ProviderWithQuery';
import { AwsIcSection } from './AwsIcSection';

const server = setupServer();

beforeAll(() => {
  server.listen();
});

beforeEach(() => {
  server.use(...makeHandlers([fetchUnifiedResources('get', [])]));
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();

  jest.clearAllMocks();
});

afterAll(() => server.close());

describe('AwsIcSection', () => {
  test('empty state when there is no applications in cluster', async () => {
    render(
      <ProviderWithQuery>
        <AwsIcSection />
      </ProviderWithQuery>
    );

    await screen.findByText(/no aws identity center found/i);
    expect(screen.queryByText(/make a new selection/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/no access defined/i)).not.toBeInTheDocument();
  });

  test('empty state when there are no selections yet', async () => {
    server.use(fetchUnifiedResources('get', appsWithAllMatchingPermissionSet));

    render(
      <ProviderWithQuery>
        <AwsIcSection />
      </ProviderWithQuery>
    );

    await screen.findByText(/no access defined/i);

    expect(screen.getByText(/make a new selection/i)).toBeInTheDocument();
    expect(screen.queryByText(/showing/i)).not.toBeInTheDocument(); // no table
  });

  test('making wildcard selection and afterwwards removing a account from the table removes the row', async () => {
    const user = userEvent.setup();
    server.use(fetchUnifiedResources('get', appsWithAllMatchingPermissionSet));

    render(
      <ProviderWithQuery>
        <AwsIcSection />
      </ProviderWithQuery>
    );

    await screen.findByText(/no access defined/i);

    // Open selection dialog.
    await user.click(
      screen.getByRole('button', { name: /make a new selection/i })
    );

    expect(screen.getByText(/1. select aws accounts/i)).toBeInTheDocument();
    expect(screen.getByText(/2. select permission sets/i)).toBeInTheDocument();
    expect(
      screen.getByText(/select an account on the left/i)
    ).toBeInTheDocument();

    // Add a non wildcard selection.
    await user.click(
      screen.getByRole('checkbox', { name: /app-friendly-name-1/i })
    );
    await user.click(screen.getByRole('checkbox', { name: /ps-name-1/i }));
    await user.click(
      screen.getByRole('checkbox', { name: /app-friendly-name-2/i })
    );
    await user.click(screen.getByRole('button', { name: /add selection/i }));

    // Test selection is rendered on table.
    expect(screen.getAllByTestId(/row-*/i)).toHaveLength(2);
    expect(screen.getByText(/app-friendly-name-1/i)).toBeInTheDocument();
    expect(screen.getByText(/app-friendly-name-2/i)).toBeInTheDocument();
    expect(screen.getAllByText(/ps-name-1/i)).toHaveLength(2);

    // Test wildcard selection
    await user.click(
      screen.getByRole('button', { name: /make a new selection/i })
    );
    await user.click(screen.getByRole('checkbox', { name: /select all/i }));
    await user.click(screen.getByRole('checkbox', { name: /ps-name-1/i }));
    await user.click(screen.getByRole('checkbox', { name: /ps-name-2/i }));

    await user.click(screen.getByRole('button', { name: /add selection/i }));

    // Test adding wildcard replaced all previous selections.
    expect(screen.getAllByTestId(/row-*/i)).toHaveLength(1);
    expect(screen.getByText(/any account/i)).toBeInTheDocument();
    expect(screen.getByText(/ps-name-1/i)).toBeInTheDocument();
    expect(screen.getByText(/ps-name-2/i)).toBeInTheDocument();

    expect(
      screen.getByRole('button', { name: /make a new selection/i })
    ).toBeDisabled();

    // Test removing an arn from table.
    await user.click(screen.getByText(/ps-name-2/));
    expect(screen.queryByText(/ps-name-2/i)).not.toBeInTheDocument();
    expect(screen.getByText(/ps-name-1/i)).toBeInTheDocument();

    // Test removing last row from table.
    await user.click(screen.getByTestId('remove-*'));
    expect(screen.queryByText(/ps-name-2/i)).not.toBeInTheDocument();

    expect(screen.getByText(/no access defined/i)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /make a new selection/i })
    ).toBeEnabled();
  });

  test('making separate selections (non wildcard) and afterwards removing all arns for an account, removes that row', async () => {
    const user = userEvent.setup();
    server.use(fetchUnifiedResources('get', appsWithAllMatchingPermissionSet));

    render(
      <ProviderWithQuery>
        <AwsIcSection />
      </ProviderWithQuery>
    );

    await screen.findByText(/no access defined/i);

    // Open selection dialog.
    await user.click(
      screen.getByRole('button', { name: /make a new selection/i })
    );

    expect(screen.getByText(/1. select aws accounts/i)).toBeInTheDocument();
    expect(screen.getByText(/2. select permission sets/i)).toBeInTheDocument();
    expect(
      screen.getByText(/select an account on the left/i)
    ).toBeInTheDocument();

    // Add a selection.
    await user.click(
      screen.getByRole('checkbox', { name: /app-friendly-name-1/i })
    );
    await user.click(screen.getByRole('checkbox', { name: /ps-name-1/i }));
    await user.click(screen.getByRole('button', { name: /add selection/i }));

    // Test adding renders table
    expect(screen.getAllByTestId(/row-*/i)).toHaveLength(1);
    expect(screen.getByText(/app-friendly-name-1/i)).toBeInTheDocument();
    expect(screen.getByText(/app-name-1/i)).toBeInTheDocument();
    expect(screen.queryAllByText(/ps-name-1/i)).toHaveLength(1);
    expect(screen.queryByText(/ps-name-2/i)).not.toBeInTheDocument();

    // Testing adding another arn to an existing account seletion
    await user.click(
      screen.getByRole('button', { name: /make a new selection/i })
    );
    await user.click(
      screen.getByRole('checkbox', { name: /app-friendly-name-1/i })
    );
    await user.click(screen.getByRole('checkbox', { name: /ps-name-2/i }));
    await user.click(screen.getByRole('button', { name: /add selection/i }));

    expect(screen.getAllByTestId(/row-*/i)).toHaveLength(1);
    expect(screen.getByText(/app-friendly-name-1/i)).toBeInTheDocument();
    expect(screen.queryAllByText(/ps-name-1/i)).toHaveLength(1);
    expect(screen.queryAllByText(/ps-name-2/i)).toHaveLength(1);

    // Selecting a different account, adds new account to table.
    await user.click(
      screen.getByRole('button', { name: /make a new selection/i })
    );
    await user.click(
      screen.getByRole('checkbox', { name: /app-friendly-name-3/i })
    );
    await user.click(screen.getByRole('checkbox', { name: /ps-name-2/i }));
    await user.click(screen.getByRole('checkbox', { name: /ps-name-3/i }));
    await user.click(screen.getByRole('button', { name: /add selection/i }));

    expect(screen.getAllByTestId(/row-*/i)).toHaveLength(2);
    expect(screen.getByText(/app-friendly-name-1/i)).toBeInTheDocument();
    expect(screen.getByText(/app-friendly-name-3/i)).toBeInTheDocument();
    expect(screen.queryAllByText(/ps-name-1/i)).toHaveLength(1);
    expect(screen.queryAllByText(/ps-name-2/i)).toHaveLength(2);
    expect(screen.queryAllByText(/ps-name-3/i)).toHaveLength(1);

    // Test removing all arns from a row, removes the row
    await user.click(screen.queryAllByText(/ps-name-2/)[0]); // The latest account selection should be the first in the row.
    await user.click(screen.getByText(/ps-name-3/));
    expect(screen.queryByText(/app-friendly-name-3/i)).not.toBeInTheDocument();
    expect(screen.getByText(/app-friendly-name-1/i)).toBeInTheDocument();
  });

  test('selecting multi accounts from one selection renders all on table', async () => {
    const user = userEvent.setup();
    server.use(fetchUnifiedResources('get', appsWithAllMatchingPermissionSet));

    render(
      <ProviderWithQuery>
        <AwsIcSection />
      </ProviderWithQuery>
    );

    await screen.findByText(/no access defined/i);

    // Open selection dialog.
    await user.click(
      screen.getByRole('button', { name: /make a new selection/i })
    );

    // Add multi account selections.
    await user.click(
      screen.getByRole('checkbox', { name: /app-friendly-name-1/i })
    );
    await user.click(
      screen.getByRole('checkbox', { name: /app-friendly-name-3/i })
    );

    expect(screen.queryAllByText(/ps-name/i)).toHaveLength(2);
    await user.click(screen.getByRole('checkbox', { name: /ps-name-1/i }));
    await user.click(screen.getByRole('checkbox', { name: /ps-name-2/i }));
    await user.click(screen.getByRole('button', { name: /add selection/i }));

    // Test adding renders all accounts on table.
    expect(screen.getAllByTestId(/row-*/i)).toHaveLength(2);
    expect(screen.getByText(/app-friendly-name-1/i)).toBeInTheDocument();
    expect(screen.getByText(/app-friendly-name-3/i)).toBeInTheDocument();
    expect(screen.queryAllByText(/ps-name-1/i)).toHaveLength(2);
    expect(screen.queryAllByText(/ps-name-2/i)).toHaveLength(2);
  });

  test('when there are no shared permission sets between wildcards or multi selected accounts', async () => {
    const user = userEvent.setup();

    // work with top 3 elements
    server.use(
      fetchUnifiedResources('get', appsWithoutPermissionSets.slice(0, 3))
    );

    render(
      <ProviderWithQuery>
        <AwsIcSection />
      </ProviderWithQuery>
    );

    await screen.findByText(/no access defined/i);

    // Open selection dialog.
    await user.click(
      screen.getByRole('button', { name: /make a new selection/i })
    );

    expect(screen.getByText(/1. select aws accounts/i)).toBeInTheDocument();
    expect(screen.getByText(/2. select permission sets/i)).toBeInTheDocument();
    expect(
      screen.getByText(/select an account on the left/i)
    ).toBeInTheDocument();

    // Select wildcard
    await user.click(screen.getByRole('checkbox', { name: /select all/i }));
    expect(screen.getByText(/there are no/i)).toBeInTheDocument();
    expect(screen.getByText(/shared/i)).toBeInTheDocument(); // shared is wrapped in a Mark component
    expect(
      screen.getByText(/permission sets available for any account/i)
    ).toBeInTheDocument();

    // Select one account
    await user.click(screen.getByRole('checkbox', { name: /select all/i })); // deselect wildcard
    await user.click(
      screen.getByRole('checkbox', { name: /app-friendly-name-1/i })
    );
    expect(
      screen.getByText(/no permission sets available for the selected account/i)
    ).toBeInTheDocument();

    // Select multi account
    await user.click(
      screen.getByRole('checkbox', { name: /app-friendly-name-2/i })
    );
    expect(screen.getByText(/there are no/i)).toBeInTheDocument();
    expect(screen.getByText(/shared/i)).toBeInTheDocument(); // shared is wrapped in a Mark component
    expect(
      screen.getByText(/permission sets between the selected accounts/i)
    ).toBeInTheDocument();
    expect(screen.getByText(/try a different selection/i)).toBeInTheDocument();
  });
});
