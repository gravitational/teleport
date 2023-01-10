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
import { Acl, makeUserContext } from 'teleport/services/user';
import TeleportContext from 'teleport/teleportContext';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { ResourceKind } from 'teleport/Discover/Shared';
import { DiscoverProvider } from 'teleport/Discover/useDiscover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { fullAccess, fullAcl } from 'teleport/mocks/contexts';

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
  function create(kind: ResourceKind, userAcl: Acl) {
    const ctx = new TeleportContext();

    ctx.storeUser.state = makeUserContext({
      ...userContextJson,
      userAcl,
    });

    return render(
      <MemoryRouter>
        <TeleportContextProvider ctx={ctx}>
          <FeaturesContextProvider>
            <DiscoverProvider>
              <SelectResource
                selectedResourceKind={kind}
                onSelect={() => null}
                onNext={() => null}
                resourceState={null}
              />
            </DiscoverProvider>
          </FeaturesContextProvider>
        </TeleportContextProvider>
      </MemoryRouter>
    );
  }

  describe('server', () => {
    test('shows permissions error when lacking tokens.create', () => {
      create(ResourceKind.Server, {
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
      create(ResourceKind.Server, {
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
      create(ResourceKind.Server, fullAcl);

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
      create(ResourceKind.Database, {
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
      create(ResourceKind.Database, {
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

    test('has the proceed button disabled without a selection', () => {
      create(ResourceKind.Database, fullAcl);

      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    });
  });

  describe('kubernetes', () => {
    test('shows permissions error when lacking tokens.create', () => {
      create(ResourceKind.Kubernetes, {
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
      create(ResourceKind.Kubernetes, {
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
      create(ResourceKind.Kubernetes, fullAcl);

      expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
    });
  });

  describe('application', () => {
    test('shows permissions error when lacking tokens.create', () => {
      create(ResourceKind.Application, {
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
      create(ResourceKind.Application, {
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
      create(ResourceKind.Application, fullAcl);

      expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
    });
  });

  describe('desktop', () => {
    test('shows permissions error when lacking tokens.create', () => {
      create(ResourceKind.Desktop, {
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
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    });

    test('shows permissions error when lacking desktops.read', () => {
      create(ResourceKind.Desktop, {
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
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    });

    test('has the proceed button enabled when having correct permissions', () => {
      create(ResourceKind.Desktop, fullAcl);

      expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
    });

    test('has the Active Directory note visible when having correct permissions', () => {
      create(ResourceKind.Desktop, fullAcl);

      expect(
        screen.getByText(
          /Teleport Desktop Access currently only supports Windows Desktops managed by Active Directory \(AD\)/
        )
      ).toBeInTheDocument();
    });
  });
});
