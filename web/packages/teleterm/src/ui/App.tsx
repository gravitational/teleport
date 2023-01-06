import React from 'react';
import { DndProvider } from 'react-dnd';
import { HTML5Backend } from 'react-dnd-html5-backend';
import styled from 'styled-components';

import { Failed } from 'design/CardError';

import { AppInitializer } from 'teleterm/ui/AppInitializer';

import CatchError from './components/CatchError';
import ModalsHost from './ModalsHost';
import AppContextProvider from './appContextProvider';
import AppContext from './appContext';
import ThemeProvider from './ThemeProvider';
import { LayoutManager } from './LayoutManager';

export const App: React.FC<{ ctx: AppContext }> = ({ ctx }) => {
  return (
    <StyledApp>
      <CatchError>
        <DndProvider backend={HTML5Backend}>
          <AppContextProvider value={ctx}>
            <ThemeProvider
              fonts={{
                mono: ctx.mainProcessClient.configService.get(
                  'fonts.monoFamily'
                ).value,
                sansSerif: ctx.mainProcessClient.configService.get(
                  'fonts.sansSerifFamily'
                ).value,
              }}
            >
              <AppInitializer>
                <LayoutManager />
                <ModalsHost />
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
