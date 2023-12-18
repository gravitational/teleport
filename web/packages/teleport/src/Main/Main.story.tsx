/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React from 'react';
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';
import { Flex } from 'design';

import { ContextProvider, Context } from 'teleport';
import { getOSSFeatures } from 'teleport/features';

import { clusters } from 'teleport/Clusters/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';
import { events } from 'teleport/Audit/fixtures';
import { sessions } from 'teleport/Sessions/fixtures';
import { apps } from 'teleport/Apps/fixtures';
import { databases } from 'teleport/Databases/fixtures';

import { kubes } from 'teleport/Kubes/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';

import { userContext } from './fixtures';
import { Main } from './Main';

function createTeleportContext() {
  const ctx = new Context();

  // mock services
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

  return ctx;
}

export function OSS() {
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/one/nodes'],
  });
  const ctx = createTeleportContext();

  return (
    <Flex my={-3} mx={-4}>
      <ContextProvider ctx={ctx}>
        <Router history={history}>
          <Main features={getOSSFeatures()} />
        </Router>
      </ContextProvider>
    </Flex>
  );
}

OSS.storyName = 'Main';

export default {
  title: 'Teleport/Main',
};
