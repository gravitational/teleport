/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { PropsWithChildren } from 'react';
import { MemoryRouter } from 'react-router';

import { Box } from 'design';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { ContextProvider } from 'teleport/index';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import { TopBar as Component } from 'teleport/TopBar';
import { UserContext } from 'teleport/User/UserContext';

export const TopBar: React.FC<PropsWithChildren> = ({ children }) => {
  const ctx = createTeleportContext();
  const updatePreferences = () => Promise.resolve();
  const getClusterPinnedResources = () => Promise.resolve([]);
  const updateClusterPinnedResources = () => Promise.resolve();
  const updateDiscoverResourcePreferences = () => Promise.resolve();
  const preferences = makeDefaultUserPreferences();

  return (
    <MemoryRouter>
      <UserContext.Provider
        value={{
          preferences,
          updatePreferences,
          getClusterPinnedResources,
          updateClusterPinnedResources,
          updateDiscoverResourcePreferences,
        }}
      >
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <FeaturesContextProvider value={getOSSFeatures()}>
              <LayoutContextProvider>
                <Box
                  css={`
                    position: absolute;
                    top: 0;
                    right: 0;
                    left: 0;
                  `}
                >
                  <Component />
                  {children}
                </Box>
              </LayoutContextProvider>
            </FeaturesContextProvider>
          </ContextProvider>
        </InfoGuidePanelProvider>
      </UserContext.Provider>
    </MemoryRouter>
  );
};
