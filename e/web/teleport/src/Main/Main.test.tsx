import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';
import { ToastNotificationProvider } from 'shared/components/ToastNotification';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportContextE from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import { apps } from 'teleport/Apps/fixtures';
import { events } from 'teleport/Audit/fixtures';
import { clusters } from 'teleport/Clusters/fixtures';
import { databases } from 'teleport/Databases/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import { kubes } from 'teleport/Kubes/fixtures';
import { userContext } from 'teleport/Main/fixtures';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import { nodes } from 'teleport/Nodes/fixtures';
import { storageService } from 'teleport/services/storageService';
import { sessions } from 'teleport/Sessions/fixtures';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';

import { Main } from '.';

jest.mock('teleport/services/storageService', () => ({
  storageService: {
    getCloudUserInvites: jest.fn(),
    getAccessGraphRoleTesterEnabled: jest.fn(),
    getAccessGraphEnabled: jest.fn(),
    getUseNewRoleEditor: jest.fn(),
    clearCloudUserInvites: jest.fn(),
    getOnboardDiscover: jest.fn(),
    getBearerToken: jest.fn(),
    getRecentHistory: () => [],
    getUseLoginScopePicker: () => false,
    getScopeSelected: () => false,
  },
}));

const setupContext = (): TeleportContextE => {
  const ctx = createTeleportContextE();
  ctx.isEnterprise = false;
  ctx.auditService.fetchEvents = () =>
    Promise.resolve({ events, startKey: '' });
  ctx.clusterService.fetchClusters = () => Promise.resolve(clusters);
  ctx.nodeService.fetchNodes = () => Promise.resolve({ agents: nodes });
  ctx.sshService.fetchSessions = () => Promise.resolve(sessions);
  ctx.appService.fetchApps = () => Promise.resolve({ agents: apps });
  ctx.kubeService.fetchKubernetes = () => Promise.resolve({ agents: kubes });
  ctx.databaseService.fetchDatabases = () =>
    Promise.resolve({ agents: databases });
  ctx.desktopService.fetchDesktops = () =>
    Promise.resolve({ agents: desktops });
  ctx.storeUser.setState(userContext);

  ctx.cloudService.sendTeleportInvite = () => Promise.resolve([]);

  return ctx;
};

test('render without user invites', async () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();

  (storageService.getCloudUserInvites as jest.Mock).mockReturnValue({
    recipients: ['user1', 'user2'],
    roles: ['role1', 'role2'],
  });

  renderComponent({ ctx });

  expect(screen.getByTestId('teleport-logo')).toBeInTheDocument();
  expect(screen.queryAllByTestId(/toast-note/i)).toHaveLength(0);
});

test('render toast notifications from successful single user invites', async () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();

  (storageService.getCloudUserInvites as jest.Mock).mockReturnValue({
    recipients: ['user1'],
    roles: ['role1', 'role2'],
  });

  renderComponent({ ctx });

  expect(screen.getByTestId('teleport-logo')).toBeInTheDocument();
  await screen.findByText(/pinned resources/i);

  await screen.findByText(/user1 was invited to your cluster/i);
  expect(screen.queryAllByTestId(/toast-note/i)).toHaveLength(1);
});

test('render toast notifications from successful multi user invites', async () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();

  (storageService.getCloudUserInvites as jest.Mock).mockReturnValue({
    recipients: ['user1', 'user2'],
    roles: ['role1', 'role2'],
  });

  renderComponent({ ctx });

  expect(screen.getByTestId('teleport-logo')).toBeInTheDocument();
  await screen.findByText(/pinned resources/i);

  await screen.findByText(/2 members were invited to your cluster/i);
  expect(screen.queryAllByTestId(/toast-note/i)).toHaveLength(1);
});

test('render toast notifications from failed user invites', async () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();

  ctx.cloudService.sendTeleportInvite = () =>
    Promise.reject(new Error('some error'));

  (storageService.getCloudUserInvites as jest.Mock).mockReturnValue({
    recipients: ['user1', 'user2'],
    roles: ['role1', 'role2'],
  });

  renderComponent({ ctx });

  expect(screen.getByTestId('teleport-logo')).toBeInTheDocument();
  await screen.findByText(/pinned resources/i);

  await screen.findByText(/could not invite users due to an error/i);
  expect(screen.queryAllByTestId(/toast-note/i)).toHaveLength(1);
});

function renderComponent({ ctx }: { ctx: TeleportContextE }) {
  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ToastNotificationProvider>
          <LayoutContextProvider>
            <Main />
          </LayoutContextProvider>
        </ToastNotificationProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}
