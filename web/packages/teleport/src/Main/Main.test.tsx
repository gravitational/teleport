/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen } from 'design/utils/testing';

import { getOSSFeatures } from 'teleport/features';
import { Context, ContextProvider } from 'teleport';
import { events } from 'teleport/Audit/fixtures';
import { clusters } from 'teleport/Clusters/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';
import { sessions } from 'teleport/Sessions/fixtures';
import { apps } from 'teleport/Apps/fixtures';
import { kubes } from 'teleport/Kubes/fixtures';
import { databases } from 'teleport/Databases/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import { userContext } from 'teleport/Main/fixtures';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import TeleportContext from 'teleport/teleportContext';

import { Main, MainProps } from './Main';

const setupContext = (): TeleportContext => {
  const ctx = new Context();
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
};

test('displays questionnaire if present', () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();

  const props: MainProps = {
    features: getOSSFeatures(),
    Questionnaire: () => <div>Passed Component!</div>,
  };

  render(
    <MemoryRouter>
      <LayoutContextProvider>
        <ContextProvider ctx={ctx}>
          <Main {...props} />
        </ContextProvider>
      </LayoutContextProvider>
    </MemoryRouter>
  );

  expect(screen.getByText('Passed Component!')).toBeInTheDocument();
});

test('renders without questionnaire prop', () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();

  const props: MainProps = {
    features: getOSSFeatures(),
  };
  expect(props.Questionnaire).toBeUndefined();

  render(
    <MemoryRouter>
      <LayoutContextProvider>
        <ContextProvider ctx={ctx}>
          <Main {...props} />
        </ContextProvider>
      </LayoutContextProvider>
    </MemoryRouter>
  );

  expect(screen.getByTestId('title')).toBeInTheDocument();
});
