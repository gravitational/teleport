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

import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import { Context, ContextProvider } from 'teleport';
import { apps } from 'teleport/Apps/fixtures';
import { events } from 'teleport/Audit/fixtures';
import { clusters } from 'teleport/Clusters/fixtures';
import { databases } from 'teleport/Databases/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import { getOSSFeatures } from 'teleport/features';
import { kubes } from 'teleport/Kubes/fixtures';
import { userContext } from 'teleport/Main/fixtures';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import { nodes } from 'teleport/Nodes/fixtures';
import { sessions } from 'teleport/Sessions/fixtures';
import TeleportContext from 'teleport/teleportContext';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';

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

test('renders', () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();

  const props: MainProps = {
    features: getOSSFeatures(),
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

  expect(screen.getByTestId('teleport-logo')).toBeInTheDocument();
});

test('displays invite collaborators feedback if present', () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();

  const props: MainProps = {
    features: getOSSFeatures(),
    inviteCollaboratorsFeedback: <div>Passed Component!</div>,
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

test('renders without invite collaborators feedback enabled', () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();

  const props: MainProps = {
    features: getOSSFeatures(),
  };
  expect(props.inviteCollaboratorsFeedback).toBeUndefined();

  render(
    <MemoryRouter>
      <LayoutContextProvider>
        <ContextProvider ctx={ctx}>
          <Main {...props} />
        </ContextProvider>
      </LayoutContextProvider>
    </MemoryRouter>
  );

  expect(screen.getByTestId('teleport-logo')).toBeInTheDocument();
});
