import { act, within } from '@testing-library/react';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import selectEvent from 'react-select-event';

import {
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import { definableResourceAccessFields } from '../role/listaccess';
import {
  fetchUnifiedResources,
  makeHandlers,
  withCustomError,
} from '../TestHelper/mocks';
import { ProviderWithQuery } from '../TestHelper/ProviderWithQuery';
import { DefineAccess } from './DefineAccess';

const mio = mockIntersectionObserver();

enableMswServer();

beforeEach(() => {
  server.use(...makeHandlers([fetchUnifiedResources('get', null)]));
});

afterEach(async () => {
  await testQueryClient.resetQueries();
  jest.clearAllMocks();
});

test(`clicking labels for all label based resources`, async () => {
  async function clickLabel(id: string) {
    let targetRow = screen.getByTestId(id);
    await user.click(within(targetRow).getByTitle(/env: test/i));
    act(mio.enterAll);
    await screen.findByText(id);
  }

  const user = userEvent.setup();

  render(
    <ProviderWithQuery>
      <DefineAccess />
    </ProviderWithQuery>
  );

  // resource application is the default tab
  await screen.findByText(/define application access/i);
  act(mio.enterAll); // trigger the isIntersecting of IntersectionObserver

  await screen.findByText(/no access defined/i);

  await withCustomError('app_labels', () => clickLabel('AppTestRow'));

  // Perform same test for other tabs
  for (let i = 0; i < definableResourceAccessFields.length; i++) {
    const field = definableResourceAccessFields[i];
    // skip testing as it was just tested above.
    if (field === 'app_labels') {
      continue;
    }
    // these fields are not label based resources
    if (field === 'awsIc' || field === 'github_permissions') {
      continue;
    }

    // Go to tab.
    await user.click(screen.getByTestId(field));

    await withCustomError(field, async () => {
      expect(await screen.findByText(/define .* access/i)).toBeInTheDocument();
      act(mio.enterAll);
      expect(await screen.findByText(/no access defined/i)).toBeInTheDocument();
    });

    switch (field) {
      case 'db_labels':
        await withCustomError('db_labels', () => clickLabel('DbTestRow'));
        break;

      case 'kubernetes_labels':
        await withCustomError('kubernetes_labels', () =>
          clickLabel('KubeClusterTestRow')
        );
        break;

      case 'node_labels':
        await withCustomError('node_labels', () => clickLabel('NodeTestRow'));
        break;

      case 'windows_desktop_labels':
        await clickLabel('WindowsTestRow');
        await withCustomError('windows_desktop_labels', () =>
          clickLabel('WindowsTestRow')
        );
        break;

      case 'linux_desktop_labels':
        await clickLabel('LinuxTestRow');
        await withCustomError('linux_desktop_labels', () =>
          clickLabel('LinuxTestRow')
        );
        break;

      default:
        field satisfies never;
    }
  }
});

test(`clicking a label then clicking the same label deselects it`, async () => {
  const user = userEvent.setup();

  render(
    <ProviderWithQuery>
      <DefineAccess />
    </ProviderWithQuery>
  );

  await screen.findByText(/define application access/i);
  act(mio.enterAll);
  await screen.findByText(/no access defined/i);

  // Click a label to select it.
  await user.click(
    within(screen.getByTestId('AppTestRow')).getByTitle(/env: test/i)
  );
  act(mio.enterAll);
  await screen.findByText(/AppTestRow3/i);

  const inputWrapper = screen.getByTestId('resource-label-input');
  expect(within(inputWrapper).getByText(/env: test/i)).toBeInTheDocument();

  // Click the same label again — should deselect it.
  await user.click(
    within(screen.getByTestId('AppTestRow')).getByTitle(/env: test/i)
  );
  act(mio.enterAll);
  await screen.findByText(/no access defined/i);

  expect(
    within(inputWrapper).queryByText(/env: test/i)
  ).not.toBeInTheDocument();
});

test(`clicking a label then typing the same label deselects it`, async () => {
  const user = userEvent.setup();

  render(
    <ProviderWithQuery>
      <DefineAccess />
    </ProviderWithQuery>
  );

  await screen.findByText(/define application access/i);
  act(mio.enterAll);
  await screen.findByText(/no access defined/i);

  // Click a label to select it.
  await user.click(
    within(screen.getByTestId('AppTestRow')).getByTitle(/env: test/i)
  );
  act(mio.enterAll);
  await screen.findByText(/AppTestRow3/i);

  const inputWrapper = screen.getByTestId('resource-label-input');
  expect(within(inputWrapper).getByText(/env: test/i)).toBeInTheDocument();

  // Type the same label — should deselect it.
  await user.type(inputWrapper, 'env: test');
  await selectEvent.select(inputWrapper, 'env: test');
  act(mio.enterAll);
  await screen.findByText(/no access defined/i);

  expect(
    within(inputWrapper).queryByText(/env: test/i)
  ).not.toBeInTheDocument();
});
