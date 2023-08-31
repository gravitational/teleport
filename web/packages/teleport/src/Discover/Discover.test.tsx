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

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { Discover } from 'teleport/Discover/Discover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { createTeleportContext, getAcl } from 'teleport/mocks/contexts';
import { getOSSFeatures } from 'teleport/features';
import cfg from 'teleport/config';
import {
  APPLICATIONS,
  KUBERNETES,
  SERVERS,
  WINDOWS_DESKTOPS,
} from 'teleport/Discover/SelectResource/resources';
import {
  DATABASES,
  DATABASES_UNGUIDED,
  DATABASES_UNGUIDED_DOC,
} from 'teleport/Discover/SelectResource/databases';

import { ClusterResource } from 'teleport/services/userPreferences/types';

import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';

import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

import { ResourceKind } from './Shared';

type createProps = {
  initialEntry?: string;
  preferredResource?: ClusterResource;
};

const create = ({ initialEntry = '', preferredResource }: createProps) => {
  const defaultPref = makeDefaultUserPreferences();
  mockUserContextProviderWith(
    makeTestUserContext({
      preferences: {
        ...defaultPref,
        onboard: {
          preferredResources: preferredResource ? [preferredResource] : [],
        },
      },
    })
  );

  const userAcl = getAcl();
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
};

test('displays all resources by default', () => {
  create({});

  expect(screen.getAllByTestId(ResourceKind.Server)).toHaveLength(
    SERVERS.length
  );
  expect(screen.getAllByTestId(ResourceKind.Desktop)).toHaveLength(
    WINDOWS_DESKTOPS.length
  );
  expect(screen.getAllByTestId(ResourceKind.Database)).toHaveLength(
    DATABASES.length + DATABASES_UNGUIDED.length + DATABASES_UNGUIDED_DOC.length
  );
  expect(screen.getAllByTestId(ResourceKind.Application)).toHaveLength(
    APPLICATIONS.length
  );
  expect(screen.getAllByTestId(ResourceKind.Kubernetes)).toHaveLength(
    KUBERNETES.length
  );
});

test('location state applies filter/search', () => {
  create({
    initialEntry: 'desktop',
    preferredResource: ClusterResource.RESOURCE_WEB_APPLICATIONS,
  });

  expect(screen.getAllByTestId(ResourceKind.Desktop)).toHaveLength(
    WINDOWS_DESKTOPS.length
  );

  expect(
    screen.queryByTestId(ResourceKind.Application)
  ).not.toBeInTheDocument();
  expect(screen.queryByTestId(ResourceKind.Server)).not.toBeInTheDocument();
  expect(screen.queryByTestId(ResourceKind.Database)).not.toBeInTheDocument();
  expect(screen.queryByTestId(ResourceKind.Kubernetes)).not.toBeInTheDocument();
});

describe('location state', () => {
  test('displays servers when the location state is server', () => {
    create({ initialEntry: 'server' });

    expect(screen.getAllByTestId(ResourceKind.Server)).toHaveLength(
      SERVERS.length
    );

    // we assert three databases for servers because the naming convention includes "server"
    expect(screen.queryAllByTestId(ResourceKind.Database)).toHaveLength(3);

    expect(screen.queryByTestId(ResourceKind.Desktop)).not.toBeInTheDocument();
    expect(
      screen.queryByTestId(ResourceKind.Application)
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId(ResourceKind.Kubernetes)
    ).not.toBeInTheDocument();
  });

  test('displays desktops when the location state is desktop', () => {
    create({ initialEntry: 'desktop' });

    expect(screen.getAllByTestId(ResourceKind.Desktop)).toHaveLength(
      WINDOWS_DESKTOPS.length
    );

    expect(screen.queryByTestId(ResourceKind.Server)).not.toBeInTheDocument();
    expect(screen.queryByTestId(ResourceKind.Database)).not.toBeInTheDocument();
    expect(
      screen.queryByTestId(ResourceKind.Application)
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId(ResourceKind.Kubernetes)
    ).not.toBeInTheDocument();
  });

  test('displays apps when the location state is application', () => {
    create({ initialEntry: 'application' });

    expect(screen.getAllByTestId(ResourceKind.Application)).toHaveLength(
      APPLICATIONS.length
    );

    expect(screen.queryByTestId(ResourceKind.Server)).not.toBeInTheDocument();
    expect(screen.queryByTestId(ResourceKind.Desktop)).not.toBeInTheDocument();
    expect(screen.queryByTestId(ResourceKind.Database)).not.toBeInTheDocument();
    expect(
      screen.queryByTestId(ResourceKind.Kubernetes)
    ).not.toBeInTheDocument();
  });

  test('displays databases when the location state is database', () => {
    create({ initialEntry: 'database' });

    expect(screen.getAllByTestId(ResourceKind.Database)).toHaveLength(
      DATABASES.length +
        DATABASES_UNGUIDED.length +
        DATABASES_UNGUIDED_DOC.length
    );

    expect(screen.queryByTestId(ResourceKind.Server)).not.toBeInTheDocument();
    expect(screen.queryByTestId(ResourceKind.Desktop)).not.toBeInTheDocument();
    expect(
      screen.queryByTestId(ResourceKind.Application)
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId(ResourceKind.Kubernetes)
    ).not.toBeInTheDocument();
  });

  test('displays kube resources when the location state is kubernetes', () => {
    create({ initialEntry: 'kubernetes' });

    expect(screen.getAllByTestId(ResourceKind.Kubernetes)).toHaveLength(
      KUBERNETES.length
    );

    expect(screen.queryByTestId(ResourceKind.Server)).not.toBeInTheDocument();
    expect(screen.queryByTestId(ResourceKind.Desktop)).not.toBeInTheDocument();
    expect(screen.queryByTestId(ResourceKind.Database)).not.toBeInTheDocument();
    expect(
      screen.queryByTestId(ResourceKind.Application)
    ).not.toBeInTheDocument();
  });
});
