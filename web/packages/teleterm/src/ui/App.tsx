/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { DndProvider } from 'react-dnd';
import { HTML5Backend } from 'react-dnd-html5-backend';
import styled from 'styled-components';

import { Failed } from 'design/CardError';

import { AppInitializer } from 'teleterm/ui/AppInitializer';
import { NotificationsHost } from 'teleterm/ui/components/Notifcations';

import CatchError from './components/CatchError';
import ModalsHost from './ModalsHost';
import AppContextProvider from './appContextProvider';
import AppContext from './appContext';
import ThemeProvider from './ThemeProvider';
import { LayoutManager } from './LayoutManager';

export const App: React.FC<{ ctx: AppContext }> = ({ ctx }) => {
  const { appearance } = ctx.mainProcessClient.configService.get();
  return (
    <StyledApp>
      <CatchError>
        <DndProvider backend={HTML5Backend}>
          <AppContextProvider value={ctx}>
            <ThemeProvider appearanceConfig={appearance}>
              <AppInitializer>
                <LayoutManager />
                <ModalsHost />
                <NotificationsHost />
              </AppInitializer>
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
      <Failed alignSelf={'baseline'} message={props.message} />
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
