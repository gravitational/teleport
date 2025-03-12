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
import { DndProvider } from 'react-dnd';
import { HTML5Backend } from 'react-dnd-html5-backend';

import { AppInitializer } from 'teleterm/ui/AppInitializer';

import AppContext from './appContext';
import AppContextProvider from './appContextProvider';
import { StyledApp } from './components/App';
import { CatchError } from './components/CatchError';
import { ResourcesContextProvider } from './DocumentCluster/resourcesContext';
import { ThemeProvider } from './ThemeProvider';
import { ConnectionsContextProvider } from './TopBar/Connections/connectionsContext';
import { VnetContextProvider } from './Vnet/vnetContext';

export const App: React.FC<{ ctx: AppContext }> = ({ ctx }) => {
  return (
    <CatchError>
      <StyledApp>
        <DndProvider backend={HTML5Backend}>
          <AppContextProvider value={ctx}>
            <ResourcesContextProvider>
              <ConnectionsContextProvider>
                <VnetContextProvider>
                  <ThemeProvider>
                    <AppInitializer />
                  </ThemeProvider>
                </VnetContextProvider>
              </ConnectionsContextProvider>
            </ResourcesContextProvider>
          </AppContextProvider>
        </DndProvider>
      </StyledApp>
    </CatchError>
  );
};
