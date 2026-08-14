import { act, within } from '@testing-library/react';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { useEffect } from 'react';

import {
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import ResourceService from 'teleport/services/resources';

import { fetchUnifiedResources, makeHandlers } from '../TestHelper/mocks';
import { ProviderWithQuery } from '../TestHelper/ProviderWithQuery';
import { standardRoleEmptyAccess } from '../TestHelper/roles';
import { DefineAccess } from './DefineAccess';

const mio = mockIntersectionObserver();

enableMswServer();

// Predicate produced when the user clicks the env: test label
// on AppTestRow (used by the "Matched Applications" tab).
const matchedEnvTestQuery =
  '((labels["env"] == "test")) && labels["teleport.dev/origin"] != "aws-identity-center"';

let spiedUnifiedResource;
beforeEach(() => {
  server.use(...makeHandlers([fetchUnifiedResources('get', null)]));
  spiedUnifiedResource = jest.spyOn(
    ResourceService.prototype,
    'fetchUnifiedResources'
  );
});

afterEach(async () => {
  await testQueryClient.resetQueries();
  jest.clearAllMocks();
});

function getTab(name: string) {
  return screen.getByText(name).closest('[data-tab-id]') as HTMLElement;
}

function calledWithQuery(query: string) {
  return spiedUnifiedResource.mock.calls.some(call => call[1]?.query === query);
}

function DefineAccessEditingRoleWithEmptyAllow() {
  const { guideEditor } = useAccessListManagementContext();

  useEffect(() => {
    guideEditor.standardRoleState.initRoleEditState({
      ...standardRoleEmptyAccess,
      spec: {
        ...standardRoleEmptyAccess.spec,
        allow: {},
      },
    });
  }, []);

  return <DefineAccess />;
}

test('tab default behavior (no access) and tab behavior after label is clicked', async () => {
  const user = userEvent.setup({ delay: null });

  render(
    <ProviderWithQuery>
      <DefineAccess />
    </ProviderWithQuery>
  );

  await screen.findByText(/define application access/i);
  act(mio.enterAll);
  await screen.findByText(/no access defined/i);

  // Default, matched tab is disabled.
  expect(getTab('All Applications')).toBeInTheDocument();
  expect(getTab('Matched Applications')).toHaveAttribute('disabled');
  await user.hover(getTab('Matched Applications'));
  await screen.findByText(
    /define access by typing or clicking on labels to see matched applications/i
  );

  // Default, only the unfiltered (all resources tab) fetch fires on mount.
  expect(
    calledWithQuery('labels["teleport.dev/origin"] != "aws-identity-center"')
  ).toBe(true);
  expect(calledWithQuery(matchedEnvTestQuery)).toBe(false);

  spiedUnifiedResource.mockClear();

  // Test clicking on label, auto switches to the matched tab.
  const targetRow = screen.getByTestId('AppTestRow');
  await user.click(within(targetRow).getByTitle(/env: test/i));
  act(mio.enterAll);

  // Wait for the matched resources fetch to fire.
  await screen.findByText(/AppTestRow2/i);
  expect(calledWithQuery(matchedEnvTestQuery)).toBe(true);

  expect(getTab('Matched Applications')).toBeEnabled();

  // Removing the only label, brings the default view back where
  // empty state is rendered and matched tab is disabled again.
  const inputWrapper = screen.getByTestId('resource-label-input');
  await user.click(
    within(inputWrapper).getByRole('button', { name: 'Remove env: test' })
  );
  act(mio.enterAll);

  await screen.findByText(/no access defined/i);
  await user.hover(getTab('Matched Applications'));
  await screen.findByText(
    /define access by typing or clicking on labels to see matched applications/i
  );
});

test('tab labels match the selected resource kind', async () => {
  const user = userEvent.setup({ delay: null });

  render(
    <ProviderWithQuery>
      <DefineAccess />
    </ProviderWithQuery>
  );

  await screen.findByText(/define application access/i);
  expect(getTab('All Applications')).toBeInTheDocument();
  expect(getTab('Matched Applications')).toBeInTheDocument();

  await user.click(screen.getByTestId('db_labels'));
  await screen.findByText(/define database access/i);

  expect(getTab('All Databases')).toBeInTheDocument();
  expect(getTab('Matched Databases')).toBeInTheDocument();

  await user.click(screen.getByTestId('kubernetes_labels'));
  await screen.findByText(/define kubernetes cluster access/i);

  expect(getTab('All Kubernetes')).toBeInTheDocument();
  expect(getTab('Matched Kubernetes')).toBeInTheDocument();

  await user.click(screen.getByTestId('windows_desktop_labels'));
  await screen.findByText(/define windows desktop access/i);

  expect(getTab('All Desktops')).toBeInTheDocument();
  expect(getTab('Matched Desktops')).toBeInTheDocument();
});

test('clicking a label works when the existing standard role has empty allow', async () => {
  const user = userEvent.setup({ delay: null });

  render(
    <ProviderWithQuery>
      <DefineAccessEditingRoleWithEmptyAllow />
    </ProviderWithQuery>
  );

  await screen.findByText(/define application access/i);
  act(mio.enterAll);
  await screen.findByText(/no access defined/i);
  spiedUnifiedResource.mockClear();

  const targetRow = screen.getByTestId('AppTestRow');
  await user.click(within(targetRow).getByTitle(/env: test/i));
  act(mio.enterAll);

  await screen.findByText(/AppTestRow2/i);
  expect(calledWithQuery(matchedEnvTestQuery)).toBe(true);
});

test('manually switching tabs uses each tabs fetch', async () => {
  const user = userEvent.setup({ delay: null });

  render(
    <ProviderWithQuery>
      <DefineAccess />
    </ProviderWithQuery>
  );

  await screen.findByText(/define application access/i);
  act(mio.enterAll);
  await screen.findByText(/no access defined/i);

  // Defining an access auto switches to "matched" tab.
  const targetRow = screen.getByTestId('AppTestRow');
  await user.click(within(targetRow).getByTitle(/env: test/i));
  act(mio.enterAll);
  await screen.findByText(/AppTestRow2/i);

  spiedUnifiedResource.mockClear();

  // Click back to "all" tab — uses cache
  await user.click(getTab('All Applications'));
  act(mio.enterAll);
  await screen.findByText(/AppTestRow3/i);
  expect(calledWithQuery(matchedEnvTestQuery)).toBe(false);

  // Click back to "matched" tab — uses cache
  spiedUnifiedResource.mockClear();
  await user.click(getTab('Matched Applications'));
  act(mio.enterAll);
  await screen.findByText(/AppTestRow2/i);
  expect(spiedUnifiedResource).not.toHaveBeenCalled();
});
