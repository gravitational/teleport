/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';
import { Flex } from 'design';
import { ContextProvider, Context } from 'teleport';
import getFeatures from 'teleport/features';
import { Main } from './Main';
import { clusters } from 'teleport/Clusters/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';
import { events } from 'teleport/Audit/fixtures';
import { sessions } from 'teleport/Sessions/fixtures';
import { apps } from 'teleport/Apps/fixtures';
import { databases } from 'teleport/Databases/fixtures';
import { userContext } from './fixtures';
import { kubes } from 'teleport/Kubes/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';

export function OSS() {
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

OSS.storyName = 'Main';

export default {
  title: 'Teleport/Main',
};

function useMainStory() {
  const [history] = React.useState(() => {
    return createMemoryHistory({
      initialEntries: ['/web/cluster/one/nodes'],
    });
  });

  const [ctx] = React.useState(() => {
    const ctx = new Context();
    // mock services
    ctx.isEnterprise = false;
    ctx.auditService.fetchEvents = () =>
      Promise.resolve({ events, startKey: '' });
    ctx.clusterService.fetchClusters = () => Promise.resolve(clusters);
    ctx.nodeService.fetchNodes = () => Promise.resolve({ nodes });
    ctx.sshService.fetchSessions = () => Promise.resolve(sessions);
    ctx.appService.fetchApps = () => Promise.resolve({ apps });
    ctx.kubeService.fetchKubernetes = () => Promise.resolve({ kubes });
    ctx.databaseService.fetchDatabases = () => Promise.resolve({ databases });
    ctx.desktopService.fetchDesktops = () => Promise.resolve({ desktops });
    ctx.storeUser.setState(userContext);
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
