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
import { act, render, screen, tick } from 'design/utils/testing';
import { SideNavDrawerMode } from 'gen-proto-ts/teleport/userpreferences/v1/sidenav_preferences_pb';

import cfg from 'teleport/config';
import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
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

describe('Beams default sticky drawer', () => {
  const originalIsDashboard = cfg.isDashboard;
  const originalBeamsUi = cfg.beamsUi;

  // The drawer panel has no dedicated open/closed attribute: it slides into
  // view (translateX(0)) when open and off-screen (translateX(-100%)) when
  // closed. Assert on that slide transform to verify the expanded state.
  function expectBeamsDrawerOpen(open: boolean) {
    expect(document.getElementById('panel-Beams')).toHaveStyle({
      transform: open ? 'translateX(0)' : 'translateX(-100%)',
    });
  }

  beforeEach(() => {
    cfg.isDashboard = false;
    cfg.beamsUi = true;
  });

  afterEach(() => {
    cfg.isDashboard = originalIsDashboard;
    cfg.beamsUi = originalBeamsUi;
  });

  async function mountNav({
    drawerMode,
    updatePreferences = jest.fn(),
    initialPath = '/web/beams/get-started',
  }: {
    drawerMode: SideNavDrawerMode;
    updatePreferences?: jest.Mock;
    initialPath?: string;
  }) {
    const preferences = makeDefaultUserPreferences();
    preferences.sideNavDrawerMode = drawerMode;
    mockUserContextProviderWith(
      makeTestUserContext({ preferences, updatePreferences })
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
    await act(tick);
    return { updatePreferences };
  }

  test('defaults to a sticky, expanded drawer when the user has no prior preference', async () => {
    const { updatePreferences } = await mountNav({
      drawerMode: SideNavDrawerMode.UNSPECIFIED,
    });

    expectBeamsDrawerOpen(true);
    expect(updatePreferences).not.toHaveBeenCalled();
  });

  test('honors an explicit COLLAPSED preference', async () => {
    const { updatePreferences } = await mountNav({
      drawerMode: SideNavDrawerMode.COLLAPSED,
    });

    expectBeamsDrawerOpen(false);
    expect(updatePreferences).not.toHaveBeenCalled();
  });

  test('honors an explicit STICKY preference', async () => {
    await mountNav({ drawerMode: SideNavDrawerMode.STICKY });

    expectBeamsDrawerOpen(true);
  });

  test('does not default to sticky when cfg.beamsUi is false', async () => {
    cfg.beamsUi = false;
    const { updatePreferences } = await mountNav({
      drawerMode: SideNavDrawerMode.UNSPECIFIED,
    });

    expectBeamsDrawerOpen(false);
    expect(updatePreferences).not.toHaveBeenCalled();
  });
});
