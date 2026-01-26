import { act, within } from '@testing-library/react';
import { UserEvent } from '@testing-library/user-event';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { setupServer } from 'msw/node';
import selectEvent from 'react-select-event';

import {
  render,
  screen,
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

const server = setupServer();
const mio = mockIntersectionObserver();

beforeAll(() => {
  server.listen();
});

beforeEach(() => {
  server.use(...makeHandlers([fetchUnifiedResources('get', null)]));
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();

  jest.clearAllMocks();
});

afterAll(() => {
  server.close();
});

// Only label based resource can result in no resources since user can
// input any labels. Other resources like git_server and aws ic selection is
// based on already existing resources.
test(`prevent going next if typed labels do not result in a list`, async () => {
  async function testRendersDialog(user: UserEvent) {
    await screen.findByText(
      /try a different selection or remove the selections/i
    );
    expect(
      screen.getByText(/no .* were found for the selected labels/i)
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /ok/i }));
  }

  async function typeLabel() {
    server.use(...makeHandlers([fetchUnifiedResources('get', [])]));
    const reactSelectInput = screen.getByRole('combobox');
    await user.type(reactSelectInput, 'random: label');
    await selectEvent.select(reactSelectInput, /random: label/i);
  }

  async function removeLabel() {
    server.use(...makeHandlers([fetchUnifiedResources('get', null)]));
    const inputWrapper = screen.getByTestId('resource-label-input');
    await user.click(
      within(inputWrapper).getByRole('button', {
        name: 'Remove random: label',
      })
    );
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

  await typeLabel();
  act(mio.enterAll);
  await screen.findByText(/no applications matched the labels/i);

  // Close preview warning.
  expect(screen.getByText(/previewing resources/i)).toBeInTheDocument();
  expect(screen.queryByText(/show warning/i)).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Dismiss' }));
  expect(screen.queryByText(/previewing resources/i)).not.toBeInTheDocument();
  expect(screen.getByText(/show warning/i)).toBeInTheDocument();

  // Prevent going next step.
  await user.click(screen.getByRole('button', { name: /next/i }));
  await testRendersDialog(user);

  // Prevent going to other tabs.
  await user.click(screen.getByTestId('github_permissions'));
  await testRendersDialog(user);

  // Removing the offending label allows going to other tabs.
  await removeLabel();
  act(mio.enterAll);
  await screen.findByText(/no access defined/i);

  // Perform same test for other tabs
  for (let i = 0; i < definableResourceAccessFields.length; i++) {
    const field = definableResourceAccessFields[i];
    // skip testing as it was just tested above.
    if (field === 'app_labels') {
      continue;
    }
    // no custom user input is allowed for these fields and
    // selections for these fields is based on existing resources.
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

    await typeLabel();
    act(mio.enterAll);

    switch (field) {
      case 'db_labels':
        await screen.findByText(/no databases matched the labels/i);
        break;

      case 'kubernetes_labels':
        await screen.findByText(/no kubernetes clusters matched the labels/i);
        break;

      case 'node_labels':
        await screen.findByText(/no servers matched the labels/i);
        break;

      case 'windows_desktop_labels':
        await screen.findByText(/no windows desktops matched the labels/i);
        break;

      default:
        field satisfies never;
    }

    // Prevent going next step.
    await user.click(screen.getByRole('button', { name: /next/i }));
    await testRendersDialog(user);

    // Prevent going to other tabs.
    await user.click(screen.getByTestId('app_labels'));
    await testRendersDialog(user);

    // Removing the offending label allows going to other tabs.
    await removeLabel();
    act(mio.enterAll);
    await screen.findByText(/no access defined/i);
  }
}, 15000); // increase test time to 15 seconds
