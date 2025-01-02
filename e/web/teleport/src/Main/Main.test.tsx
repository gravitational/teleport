import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import cfg from 'e-teleport/config';
import { surveyService } from 'e-teleport/services/survey';
import TeleportContextE from 'e-teleport/teleportContextE';
import {
  MockedStoreAccessRequests,
  MockedWorkflowService,
} from 'e-teleport/Workflow/fixtures';
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
import { KeysEnum } from 'teleport/services/storageService';
import { sessions } from 'teleport/Sessions/fixtures';
import TeleportContext from 'teleport/teleportContext';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';

import { MainE } from './Main';

const setupContext = (): TeleportContext => {
  const ctx = new TeleportContextE();
  ctx.isEnterprise = true;
  ctx.auditService.fetchEvents = () =>
    Promise.resolve({ startKey: '', events });
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
  ctx.storeAccessRequests = new MockedStoreAccessRequests();
  ctx.workflowService = new MockedWorkflowService();

  return ctx;
};

jest.mock('shared/hooks', () => ({
  useAttempt: () => {
    return {
      attempt: { status: 'success', statusText: 'Success Text' },
      setAttempt: jest.fn(),
      run: (fn?: any) => Promise.resolve(fn()),
    };
  },
}));

cfg.oss.isStripeManaged = true;
cfg.oss.hasQuestionnaire = true;

cfg.oss.entitlements.MobileDeviceManagement = { enabled: false, limit: 0 };
cfg.oss.entitlements.DeviceTrust = { enabled: false, limit: 0 };

test('displays questionnaire if unanswered in both survey and preferences', async () => {
  mockUserContextProviderWith(makeTestUserContext());

  jest
    .spyOn(surveyService, 'getSurveyCompanyResults')
    .mockImplementation(() =>
      Promise.resolve({ companyName: '', employeeCount: '' })
    );

  const ctx = setupContext();
  localStorage.clear();
  localStorage.setItem(KeysEnum.ONBOARD_SURVEY, '{"clusterResources": []}');
  localStorage.setItem(
    KeysEnum.USER_PREFERENCES,
    '{"onboard": {"preferredResources": []}}'
  );

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <LayoutContextProvider>
          <MainE />
        </LayoutContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );

  await screen.findByText('Tell us about yourself');
  expect(screen.getByText('Company Name')).toBeInTheDocument();
});

test('does not display questionnaire if answered via survey', () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();
  localStorage.setItem(KeysEnum.ONBOARD_SURVEY, '{"clusterResources": [1]}');

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <LayoutContextProvider>
          <MainE />
        </LayoutContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );

  expect(screen.queryByText('Tell us about yourself')).not.toBeInTheDocument();
});

test('does not display questionnaire if answered via preferences', () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();
  localStorage.setItem(
    KeysEnum.USER_PREFERENCES,
    '{"onboard": {"preferredResources": [1]}}'
  );

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <LayoutContextProvider>
          <MainE />
        </LayoutContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );

  expect(screen.queryByText('Tell us about yourself')).not.toBeInTheDocument();
});
