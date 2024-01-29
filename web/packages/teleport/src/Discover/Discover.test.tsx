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

import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';

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

import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';

import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

import { ResourceKind } from './Shared';

beforeEach(() => {
  jest.restoreAllMocks();
});

type createProps = {
  initialEntry?: string;
  preferredResource?: Resource;
};

const create = ({ initialEntry = '', preferredResource }: createProps) => {
  jest.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue('Macintosh');

  const defaultPref = makeDefaultUserPreferences();
  defaultPref.onboard.preferredResources = preferredResource
    ? [preferredResource]
    : [];

  mockUserContextProviderWith(
    makeTestUserContext({ preferences: defaultPref })
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

  expect(
    screen
      .getAllByTestId(ResourceKind.Server)
      .concat(screen.getAllByTestId(ResourceKind.ConnectMyComputer))
  ).toHaveLength(SERVERS.length);
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
    preferredResource: Resource.WEB_APPLICATIONS,
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

    expect(
      screen
        .getAllByTestId(ResourceKind.Server)
        .concat(screen.getAllByTestId(ResourceKind.ConnectMyComputer))
    ).toHaveLength(SERVERS.length);

    // we assert three databases for servers because the naming convention includes "server"
    expect(screen.queryAllByTestId(ResourceKind.Database)).toHaveLength(4);

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
