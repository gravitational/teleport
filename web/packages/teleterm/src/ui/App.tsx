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
import styled from 'styled-components';

import { Failed } from 'design/CardError';

import { AppInitializer } from 'teleterm/ui/AppInitializer';

import CatchError from './components/CatchError';
import AppContextProvider from './appContextProvider';
import AppContext from './appContext';
import { StaticThemeProvider, ThemeProvider } from './ThemeProvider';
import { darkTheme } from './ThemeProvider/theme';

export const App: React.FC<{ ctx: AppContext }> = ({ ctx }) => {
  return (
    <StyledApp>
      <CatchError>
        <DndProvider backend={HTML5Backend}>
          <AppContextProvider value={ctx}>
            <ThemeProvider>
              <AppInitializer />
            </ThemeProvider>
          </AppContextProvider>
        </DndProvider>
      </CatchError>
    </StyledApp>
  );
};

export const FailedApp = (props: { message: string }) => {
  return (
    <StyledApp>
      <StaticThemeProvider theme={darkTheme}>
        <Failed alignSelf={'baseline'} message={props.message} />
      </StaticThemeProvider>
    </StyledApp>
  );
};

const StyledApp = styled.div`
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  position: absolute;
  display: flex;
  flex-direction: column;
`;
