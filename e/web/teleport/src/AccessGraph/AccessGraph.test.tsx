import { MemoryRouter } from 'react-router';

import { act, render, screen } from 'design/utils/testing';

import { loadAccessGraph } from 'e-teleport/AccessGraph/loader';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';
import { allAccessAcl, noAccess } from 'teleport/mocks/contexts';
import { storageService } from 'teleport/services/storageService';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';

import { AccessGraph } from './AccessGraph';

jest.mock('teleport/services/storageService', () => ({
  storageService: {
    getAccessGraphEnabled: jest.fn(),
    getUseNewRoleEditor: jest.fn(),
  },
}));

jest.mock('e-teleport/AccessGraph/loader', () => ({
  loadAccessGraph: jest.fn(),
  ACCESS_GRAPH_JS_FILE: 'access-graph-react-19.umd.js',
}));

let originalAccessGraphEntitlement: (typeof cfg.oss.entitlements)['AccessGraph'];
let originalAccessGraphConfigSet: boolean;

beforeEach(() => {
  originalAccessGraphEntitlement = cfg.oss.entitlements.AccessGraph;
  originalAccessGraphConfigSet = cfg.oss.identitySecurity.accessGraphConfigSet;

  jest.mocked(storageService.getAccessGraphEnabled).mockReturnValue(false);
});

afterEach(() => {
  cfg.oss.entitlements.AccessGraph = originalAccessGraphEntitlement;
  cfg.oss.identitySecurity.accessGraphConfigSet = originalAccessGraphConfigSet;
});

function renderAccessGraph(hasAccessGraphAcl = false) {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = createTeleportContextE({
    customAcl: hasAccessGraphAcl
      ? allAccessAcl
      : { ...allAccessAcl, accessGraph: noAccess },
  });

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AccessGraph />
      </ContextProvider>
    </MemoryRouter>
  );
}

test('renders the empty state if user has no permissions', () => {
  renderAccessGraph();

  expect(screen.getByTestId('tag-empty-state')).toBeInTheDocument();
});

test('renders setup error when licensed but not configured', () => {
  cfg.oss.entitlements.AccessGraph = { enabled: true, limit: 0 };
  cfg.oss.identitySecurity.accessGraphConfigSet = false;

  renderAccessGraph();

  expect(screen.getByText(/has not been configured yet/)).toBeInTheDocument();
});

test('renders setup error when licensed and configured but service unreachable', () => {
  cfg.oss.entitlements.AccessGraph = { enabled: true, limit: 0 };
  cfg.oss.identitySecurity.accessGraphConfigSet = true;

  renderAccessGraph();

  expect(
    screen.getByText(/Access Graph service cannot be contacted/)
  ).toBeInTheDocument();
});

test('renders incompatible error when access graph bundle fails to load (can be removed in v17 reaches EOL support)', async () => {
  jest.spyOn(console, 'error').mockImplementation();
  jest.mocked(storageService.getAccessGraphEnabled).mockReturnValue(true);
  jest.mocked(loadAccessGraph).mockRejectedValue(new Error('Failed to load'));

  renderAccessGraph(true);

  // Wait for AccessGraph to process the rejected promise from loadAccessGraph.
  await act(() => Promise.resolve());

  expect(
    screen.getByText(
      /Identity Security may be incompatible with this version of Teleport/
    )
  ).toBeInTheDocument();
});
