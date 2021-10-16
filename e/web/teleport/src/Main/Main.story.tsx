import React from 'react';
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';
import { Flex } from 'design';
import { ContextProvider } from 'teleport';
import TeleportContextE from 'e-teleport/teleportContextE';
import getFeatures from 'e-teleport/features';
import { Main } from 'teleport/Main/Main';
import { clusters } from 'teleport/Clusters/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';
import { events } from 'teleport/Audit/fixtures';
import { sessions } from 'teleport/Sessions/fixtures';
import { apps } from 'teleport/Apps/fixtures';
import { kubes } from 'teleport/Kubes/fixtures';
import { userContext } from 'teleport/Main/fixtures';
import { databases } from 'teleport/Databases/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import {
  MockedWorkflowService,
  MockedStoreAccessRequests,
} from 'e-teleport/Workflow/fixtures';
import { MockedCloudService } from 'e-teleport/Billing/fixtures';

export default {
  title: 'TeleportE/Main',
};

export function Enterprise() {
  const state = useMainStory();
  return (
    <Flex my={-3} mx={-4}>
      <ContextProvider ctx={state.ctx}>
        <Router history={state.history}>
          <Main {...state} />
        </Router>
      </ContextProvider>
    </Flex>
  );
}

Enterprise.storyName = 'Main';

function useMainStory() {
  const [history] = React.useState(() => {
    return createMemoryHistory({
      initialEntries: ['/web/cluster/one/nodes'],
    });
  });

  const [ctx] = React.useState(() => {
    const ctx = new TeleportContextE();
    // mock services
    ctx.isEnterprise = true;
    ctx.auditService.fetchEvents = () =>
      Promise.resolve({ startKey: '', events });
    ctx.clusterService.fetchClusters = () => Promise.resolve(clusters);
    ctx.nodeService.fetchNodes = () => Promise.resolve(nodes);
    ctx.sshService.fetchSessions = () => Promise.resolve(sessions);
    ctx.appService.fetchApps = () => Promise.resolve(apps);
    ctx.kubeService.fetchKubernetes = () => Promise.resolve(kubes);
    ctx.databaseService.fetchDatabases = () => Promise.resolve(databases);
    ctx.desktopService.fetchDesktops = () => Promise.resolve(desktops);
    ctx.storeUser.setState(userContext);
    ctx.storeAccessRequests = new MockedStoreAccessRequests();
    ctx.workflowService = new MockedWorkflowService();
    ctx.cloudService = new MockedCloudService();

    getFeatures().forEach(f => f.register(ctx));

    return ctx;
  });

  const status = 'success' as const;
  const statusText = '';

  return {
    history,
    ctx,
    status,
    statusText,
  };
}
