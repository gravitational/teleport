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
import { StoreParties, StoreDocs, StoreDialogs } from './stores';
import { useStore } from 'shared/libs/stores';
import cfg from 'teleport/config';
import service, { SessionStateEnum } from 'teleport/services/termsessions';

import TtyPlayer from 'teleport/lib/term/ttyPlayer';

export class SessionAuditContext {
  init(url) {
    this.tty = new TtyPlayer({ url });
  }
}

export const ConsoleReactContext = React.createContext(
  new SessionAuditContext()
);

export default function useSessionAuditContext() {
  const value = React.useContext(ConsoleReactContext);
  window.sessionAudit = value;
  return value;
}

export { SessionStateEnum };
