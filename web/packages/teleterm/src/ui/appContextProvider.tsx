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

import React, { PropsWithChildren } from 'react';

import { IAppContext } from 'teleterm/ui/types';

export const AppReactContext = React.createContext<IAppContext>(null);

const AppContextProvider: React.FC<PropsWithChildren<Props>> = props => {
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
