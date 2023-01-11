/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
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

import { Access, Acl, makeUserContext } from 'teleport/services/user';
import TeleportContext from 'teleport/teleportContext';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { Discover } from 'teleport/Discover/Discover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';

const fullAccess: Access = {
  list: true,
  read: true,
  edit: true,
  create: true,
  remove: true,
};

const fullAcl: Acl = {
  windowsLogins: ['Administrator'],
  tokens: fullAccess,
  appServers: fullAccess,
  kubeServers: fullAccess,
  recordedSessions: fullAccess,
  activeSessions: fullAccess,
  authConnectors: fullAccess,
  roles: fullAccess,
  users: fullAccess,
  trustedClusters: fullAccess,
  events: fullAccess,
  accessRequests: fullAccess,
  billing: fullAccess,
  dbServers: fullAccess,
  db: fullAccess,
  desktops: fullAccess,
  nodes: fullAccess,
  clipboardSharingEnabled: true,
  desktopSessionRecordingEnabled: true,
  directorySharingEnabled: true,
  connectionDiagnostic: fullAccess,
};

const userContextJson = {
  authType: 'sso',
  userName: 'Sam',
  accessCapabilities: {
    suggestedReviewers: ['george_washington@gmail.com', 'chad'],
    requestableRoles: ['dev-a', 'dev-b', 'dev-c', 'dev-d'],
  },
  cluster: {
    name: 'aws',
    lastConnected: '2020-09-26T17:30:23.512876876Z',
    status: 'online',
    nodeCount: 1,
    publicURL: 'localhost',
    authVersion: '4.4.0-dev',
    proxyVersion: '4.4.0-dev',
  },
};

describe('discover', () => {
  function create(initialEntry: string, userAcl: Acl) {
    const ctx = new TeleportContext();

    ctx.storeUser.state = makeUserContext({
      ...userContextJson,
      userAcl,
    });

    return render(
      <MemoryRouter initialEntries={[{ state: { entity: initialEntry } }]}>
        <TeleportContextProvider ctx={ctx}>
          <FeaturesContextProvider>
            <Discover />
          </FeaturesContextProvider>
        </TeleportContextProvider>
      </MemoryRouter>
    );
  }

  describe('server', () => {
    test('shows the server view when the location state is server', () => {
      create('server', {
        ...fullAcl,
      });

      expect(
        screen.getByText(
          /Teleport officially supports the following operating systems/
        )
      ).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
    });
  });

  describe('desktop', () => {
    test('shows the desktop view when the location state is desktop', () => {
      create('desktop', {
        ...fullAcl,
      });

      expect(
        screen.getByText(
          /Teleport Desktop Access currently only supports Windows Desktops managed by Active Directory/
        )
      ).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
    });
  });
});
