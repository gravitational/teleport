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

import TeleportContext from './teleportContext';

// ReactContext is an instance of React context that is used to
// access TeleportContext instance from within the virtual DOM
const ReactContext = React.createContext<TeleportContext>(null);

const TeleportContextProvider: React.FC<PropsWithChildren<Props>> = props => {
  return <ReactContext.Provider value={props.ctx} children={props.children} />;
};

export default TeleportContextProvider;

export { ReactContext };

type Props = {
  ctx: TeleportContext;
};
