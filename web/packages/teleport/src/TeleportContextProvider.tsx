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

import TeleportContext from './teleportContext';

// ReactContext is an instance of React context that is used to
// access TeleportContext instance from within the virtual DOM
const ReactContext = React.createContext<TeleportContext>(null);

const TeleportContextProvider: React.FC<Props> = props => {
  return <ReactContext.Provider value={props.ctx} children={props.children} />;
};

export default TeleportContextProvider;

export { ReactContext };

type Props = {
  ctx: TeleportContext;
};
