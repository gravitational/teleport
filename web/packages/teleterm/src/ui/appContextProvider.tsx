/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';

import { IAppContext } from 'teleterm/ui/types';

export const AppReactContext = React.createContext<IAppContext>(null);

const AppContextProvider: React.FC<Props> = props => {
  return <AppReactContext.Provider {...props} />;
};

export default AppContextProvider;

export function useAppContext() {
  const ctx = React.useContext(AppReactContext);

  // Attach the app context to the window for debugging and diagnostic purposes.
  // Do not do this in the packaged app as this exposes privileged APIs through the window object.
  if (process.env.NODE_ENV === 'development') {
    window['teleterm'] = ctx;
  }
  return ctx;
}

type Props = {
  value: IAppContext;
};
