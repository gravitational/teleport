import React from 'react';
import { MockAppContext } from './mocks';
import AppContextProvider from 'teleterm/ui/appContextProvider';
import AppContext from 'teleterm/ui/appContext';

export const MockAppContextProvider: React.FC<{ appContext?: AppContext }> = props => {
  const appContext = new MockAppContext();
  return (
    <AppContextProvider value={props.appContext || appContext}>
      {props.children}
    </AppContextProvider>
  );
};
