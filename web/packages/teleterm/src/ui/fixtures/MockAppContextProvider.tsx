import React from 'react';
import { MockAppContext } from './mocks';
import AppContextProvider from 'teleterm/ui/appContextProvider';
import AppContext from 'teleterm/ui/appContext';
import { HTML5Backend } from 'react-dnd-html5-backend';
import { DndProvider } from 'react-dnd';

export const MockAppContextProvider: React.FC<{
  appContext?: AppContext;
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
