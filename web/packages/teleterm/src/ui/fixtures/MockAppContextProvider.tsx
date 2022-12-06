import React from 'react';

import { HTML5Backend } from 'react-dnd-html5-backend';

import { DndProvider } from 'react-dnd';

import AppContextProvider from 'teleterm/ui/appContextProvider';
import { IAppContext } from 'teleterm/ui/types';

import { MockAppContext } from './mocks';

export const MockAppContextProvider: React.FC<{
  appContext?: IAppContext;
}> = props => {
  const appContext = new MockAppContext();
  return (
    <DndProvider backend={HTML5Backend}>
      <AppContextProvider value={props.appContext || appContext}>
        {props.children}
      </AppContextProvider>
    </DndProvider>
  );
};
