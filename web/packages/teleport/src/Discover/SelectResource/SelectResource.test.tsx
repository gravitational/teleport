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

import { SelectResource } from 'teleport/Discover/SelectResource/SelectResource';
import { Access, Acl, makeUserContext } from 'teleport/services/user';

import type { AgentKind } from '../useDiscover';

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

describe('select resource', () => {
  function create(resource: AgentKind, userAcl: Acl) {
    const userContext = makeUserContext({
      ...userContextJson,
      userAcl,
    });

    return render(
      <MemoryRouter>
        <SelectResource
          userContext={userContext}
          isEnterprise={false}
          nextStep={() => null}
          selectedResource={resource}
          onSelectResource={() => null}
        />
      </MemoryRouter>
    );
  }

  describe('server', () => {
    test('shows permissions error when lacking tokens.create', () => {
      create('server', {
        ...fullAcl,
        tokens: {
          ...fullAccess,
          create: false,
        },
      });

      expect(
        screen.getByText(/You are not able to add new Servers/)
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /Your Teleport Enterprise license does not include Server Access/
        )
      ).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    });

    test('shows permissions error when lacking nodes.list', () => {
      create('server', {
        ...fullAcl,
        nodes: {
          ...fullAccess,
          list: false,
        },
      });

      expect(
        screen.getByText(/You are not able to add new Servers/)
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /Your Teleport Enterprise license does not include Server Access/
        )
      ).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    });

    test('shows the teleport versions when having correct permissions', () => {
      create('server', fullAcl);

      expect(
        screen.getByText(
          /Teleport officially supports the following operating systems/
        )
      ).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
    });
  });

  describe('database', () => {
    test('shows permissions error when lacking tokens.create', () => {
      create('database', {
        ...fullAcl,
        tokens: {
          ...fullAccess,
          create: false,
        },
      });

      expect(
        screen.getByText(/You are not able to add new Databases/)
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /Your Teleport Enterprise license does not include Database Access/
        )
      ).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    });

    test('shows permissions error when lacking dbServers.read', () => {
      create('database', {
        ...fullAcl,
        dbServers: {
          ...fullAccess,
          read: false,
        },
      });

      expect(
        screen.getByText(/You are not able to add new Databases/)
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /Your Teleport Enterprise license does not include Database Access/
        )
      ).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    });

    test('has the proceed button enabled when having correct permissions', () => {
      create('database', fullAcl);

      expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
    });
  });

  describe('kubernetes', () => {
    test('shows permissions error when lacking tokens.create', () => {
      create('kubernetes', {
        ...fullAcl,
        tokens: {
          ...fullAccess,
          create: false,
        },
      });

      expect(
        screen.getByText(/You are not able to add new Kubernetes resources/)
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /Your Teleport Enterprise license does not include Kubernetes Access/
        )
      ).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    });

    test('shows permissions error when lacking kubeServers.read', () => {
      create('kubernetes', {
        ...fullAcl,
        kubeServers: {
          ...fullAccess,
          read: false,
        },
      });

      expect(
        screen.getByText(/You are not able to add new Kubernetes resources/)
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /Your Teleport Enterprise license does not include Kubernetes Access/
        )
      ).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    });

    test('has the proceed button enabled when having correct permissions', () => {
      create('kubernetes', fullAcl);

      expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
    });
  });

  describe('application', () => {
    test('shows permissions error when lacking tokens.create', () => {
      create('application', {
        ...fullAcl,
        tokens: {
          ...fullAccess,
          create: false,
        },
      });

      expect(
        screen.getByText(/You are not able to add new Applications/)
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /Your Teleport Enterprise license does not include Application Access/
        )
      ).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    });

    test('shows permissions error when lacking appServers.read', () => {
      create('application', {
        ...fullAcl,
        appServers: {
          ...fullAccess,
          read: false,
        },
      });

      expect(
        screen.getByText(/You are not able to add new Applications/)
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /Your Teleport Enterprise license does not include Application Access/
        )
      ).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    });

    test('has the proceed button enabled when having correct permissions', () => {
      create('application', fullAcl);

      expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
    });
  });

  describe('desktop', () => {
    test('shows permissions error when lacking tokens.create', () => {
      create('desktop', {
        ...fullAcl,
        tokens: {
          ...fullAccess,
          create: false,
        },
      });

      expect(
        screen.getByText(/You are not able to add new Desktops/)
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /Your Teleport Enterprise license does not include Desktop Access/
        )
      ).toBeInTheDocument();
    });

    test('shows permissions error when lacking desktops.read', () => {
      create('desktop', {
        ...fullAcl,
        desktops: {
          ...fullAccess,
          read: false,
        },
      });

      expect(
        screen.getByText(/You are not able to add new Desktops/)
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /Your Teleport Enterprise license does not include Desktop Access/
        )
      ).toBeInTheDocument();
    });

    test('has the view documentation button visible', () => {
      create('desktop', fullAcl);

      expect(
        screen.getByRole('link', { name: 'View Documentation' })
      ).toBeInTheDocument();
    });
  });
});
