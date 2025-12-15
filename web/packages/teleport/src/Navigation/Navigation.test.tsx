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

import { render, screen } from 'design/utils/testing';

import cfg from 'teleport/config';
import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import { NavTitle } from 'teleport/types';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';

import { Navigation } from '.';

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
