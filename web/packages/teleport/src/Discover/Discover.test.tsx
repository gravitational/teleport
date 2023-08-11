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

import { Acl } from 'teleport/services/user';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { Discover } from 'teleport/Discover/Discover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { getAcl, createTeleportContext } from 'teleport/mocks/contexts';
import { getOSSFeatures } from 'teleport/features';
import cfg from 'teleport/config';
import {
  SERVERS,
  APPLICATIONS,
  KUBERNETES,
  WINDOWS_DESKTOPS,
} from 'teleport/Discover/SelectResource/resources';
import {
  DATABASES,
  DATABASES_UNGUIDED,
  DATABASES_UNGUIDED_DOC,
} from 'teleport/Discover/SelectResource/databases';

import { ResourceKind } from './Shared';

describe('discover', () => {
  function create(initialEntry: string, userAcl: Acl) {
    const ctx = createTeleportContext({ customAcl: userAcl });

    return render(
      <MemoryRouter
        initialEntries={[
          { pathname: cfg.routes.discover, state: { entity: initialEntry } },
        ]}
      >
        <TeleportContextProvider ctx={ctx}>
          <FeaturesContextProvider value={getOSSFeatures()}>
            <Discover />
          </FeaturesContextProvider>
        </TeleportContextProvider>
      </MemoryRouter>
    );
  }

  describe('server', () => {
    test('shows all the servers when location state is server', () => {
      create('server', getAcl());

      expect(screen.getAllByTestId(ResourceKind.Server)).toHaveLength(
        SERVERS.length
      );
    });
  });

  describe('desktop', () => {
    test('shows the desktops when the location state is desktop', () => {
      create('desktop', getAcl());

      expect(screen.getAllByTestId(ResourceKind.Desktop)).toHaveLength(
        WINDOWS_DESKTOPS.length
      );
    });
  });

  describe('application', () => {
    test('shows the apps when the location state is application', () => {
      create('application', getAcl());

      expect(screen.getAllByTestId(ResourceKind.Application)).toHaveLength(
        APPLICATIONS.length
      );
    });
  });

  describe('database', () => {
    test('shows the database when the location state is database', () => {
      create('database', getAcl());

      expect(screen.getAllByTestId(ResourceKind.Database)).toHaveLength(
        DATABASES.length +
          DATABASES_UNGUIDED.length +
          DATABASES_UNGUIDED_DOC.length
      );
    });
  });

  describe('kube', () => {
    test('shows the kubes when the location state is kubernetes', () => {
      create('kubernetes', getAcl());

      expect(screen.getAllByTestId(ResourceKind.Kubernetes)).toHaveLength(
        KUBERNETES.length
      );
    });
  });
});
