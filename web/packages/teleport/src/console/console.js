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
import { StoreSession, StoreScp } from './stores';
import { useStore } from 'shared/libs/stores';
import cfg from 'teleport/config';

class Console {
  storeSession = new StoreSession();
  storeScp = new StoreScp();

  init({ clusterId }) {
    cfg.setClusterId(clusterId);
    return Promise.resolve();
  }
}

const ConsoleContext = React.createContext(new Console());

export default ConsoleContext;

export function useConsole() {
  const value = React.useContext(ConsoleContext);

  if (!value) {
    throw new Error('ConsoleContext is missing a value');
  }

  window.consoleContext = value;
  return value;
}

export function useStoreSession() {
  const console = React.useContext(ConsoleContext);
  return useStore(console.storeSession);
}

export function useStoreScp() {
  const console = React.useContext(ConsoleContext);
  return useStore(console.storeScp);
}
