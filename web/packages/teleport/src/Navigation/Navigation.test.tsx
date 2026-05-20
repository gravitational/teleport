/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Beams } from 'design/Icon';
import { render, screen } from 'design/utils/testing';
import { SideNavDrawerMode } from 'gen-proto-ts/teleport/userpreferences/v1/sidenav_preferences_pb';

import cfg from 'teleport/config';
import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { storageService } from 'teleport/services/storageService';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import { NavTitle, type TeleportFeature } from 'teleport/types';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';

import { Navigation } from '.';
import { NavigationCategory } from './categories';

const beamsFeature: TeleportFeature = {
  category: NavigationCategory.Beams,
  hasAccess: () => true,
  route: {
    title: 'Quickstart',
    path: '/web/beams/get-started',
    exact: true,
    component: () => null,
  },
  navigationItem: {
    title: NavTitle.BeamsQuickstart,
    icon: Beams,
    exact: true,
    getLink: () => '/web/beams/get-started',
  },
};

test('show all dashboard navigation items', async () => {
  const expectedItems = [
    NavTitle.Roles,
    NavTitle.Users,
    NavTitle.AuthConnectorsShortened,
    NavTitle.ManageClustersShortened,
    NavTitle.Downloads,
  ];
  cfg.isDashboard = true;
  const defaultPref = makeDefaultUserPreferences();
  mockUserContextProviderWith(
    makeTestUserContext({ preferences: defaultPref })
  );
  const ctx = createTeleportContext();
  const features = getOSSFeatures();
  const dashboardItems = features.filter(feature =>
    expectedItems.includes(feature.navigationItem?.title)
  );
  const nonDashboardItems = features.filter(
    feature => !expectedItems.includes(feature.navigationItem?.title)
  );

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <FeaturesContextProvider value={features}>
          <Navigation />
        </FeaturesContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );

  dashboardItems.forEach(item => {
    expect(screen.getByText(item.navigationItem.title)).toBeInTheDocument();
  });

  nonDashboardItems.forEach(item => {
    if (!item.navigationItem) {
      return;
    }
    expect(
      screen.queryByText(item.navigationItem.title)
    ).not.toBeInTheDocument();
  });
});

describe('Beams nav category', () => {
  const originalIsDashboard = cfg.isDashboard;
  const originalBeamsUi = cfg.beamsUi;

  beforeEach(() => {
    cfg.isDashboard = false;
    localStorage.clear();
  });

  afterEach(() => {
    cfg.isDashboard = originalIsDashboard;
    cfg.beamsUi = originalBeamsUi;
    localStorage.clear();
  });

  function renderNav(initialPath = '/') {
    mockUserContextProviderWith(
      makeTestUserContext({ preferences: makeDefaultUserPreferences() })
    );
    const ctx = createTeleportContext();
    const features = [beamsFeature];

    return render(
      <MemoryRouter initialEntries={[initialPath]}>
        <ContextProvider ctx={ctx}>
          <FeaturesContextProvider value={features}>
            <Navigation />
          </FeaturesContextProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  }

  test('renders Beams above Resources when cfg.beamsUi is true', () => {
    cfg.beamsUi = true;

    renderNav();

    const beamsButton = screen.getByRole('button', { name: 'Beams' });
    const resourcesButton = screen.getByRole('button', { name: 'Resources' });
    const buttons = screen.getAllByRole('button');

    expect(buttons.indexOf(beamsButton)).toBeLessThan(
      buttons.indexOf(resourcesButton)
    );
  });

  test('renders Beams below Resources when cfg.beamsUi is false', () => {
    cfg.beamsUi = false;

    renderNav();

    const beamsButton = screen.getByRole('button', { name: 'Beams' });
    const resourcesButton = screen.getByRole('button', { name: 'Resources' });
    const buttons = screen.getAllByRole('button');

    expect(buttons.indexOf(resourcesButton)).toBeLessThan(
      buttons.indexOf(beamsButton)
    );
  });
});

describe('Beams first-visit auto-expand', () => {
  const originalIsDashboard = cfg.isDashboard;
  const originalBeamsUi = cfg.beamsUi;

  beforeEach(() => {
    cfg.isDashboard = false;
    cfg.beamsUi = true;
    localStorage.clear();
  });

  afterEach(() => {
    cfg.isDashboard = originalIsDashboard;
    cfg.beamsUi = originalBeamsUi;
    localStorage.clear();
  });

  function mountNav({
    initialPath,
    updatePreferences,
  }: {
    initialPath: string;
    updatePreferences: jest.Mock;
  }) {
    mockUserContextProviderWith(
      makeTestUserContext({
        preferences: makeDefaultUserPreferences(),
        updatePreferences,
      })
    );
    const ctx = createTeleportContext();
    const features = [beamsFeature];

    render(
      <MemoryRouter initialEntries={[initialPath]}>
        <ContextProvider ctx={ctx}>
          <FeaturesContextProvider value={features}>
            <Navigation />
          </FeaturesContextProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  }

  test('flips sideNavDrawerMode to STICKY and sets the flag on first Beams page visit', () => {
    const updatePreferences = jest.fn();
    expect(storageService.getBeamsFirstVisitExpanded()).toBe(false);

    mountNav({
      initialPath: '/web/beams/get-started',
      updatePreferences,
    });

    expect(storageService.getBeamsFirstVisitExpanded()).toBe(true);
    expect(updatePreferences).toHaveBeenCalledWith({
      sideNavDrawerMode: SideNavDrawerMode.STICKY,
    });
  });

  test('does not run again once the flag is set', () => {
    const updatePreferences = jest.fn();
    storageService.setBeamsFirstVisitExpanded();

    mountNav({
      initialPath: '/web/beams/get-started',
      updatePreferences,
    });

    expect(updatePreferences).not.toHaveBeenCalled();
  });

  test('does not fire when landing on a non-Beams page', () => {
    const updatePreferences = jest.fn();

    mountNav({
      initialPath: '/web/cluster/x/resources',
      updatePreferences,
    });

    expect(storageService.getBeamsFirstVisitExpanded()).toBe(false);
    expect(updatePreferences).not.toHaveBeenCalled();
  });
});
