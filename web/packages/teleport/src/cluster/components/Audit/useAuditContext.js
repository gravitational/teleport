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
import { useStore } from 'shared/libs/stores';
import StoreEvents from 'teleport/stores/storeEvents';

export class AuditContext {
  storeEvents = new StoreEvents();
}

export const ReactAuditContext = React.createContext(new AuditContext());

export default function useAuditContext() {
  const value = React.useContext(ReactAuditContext);
  return value;
}

export function useStoreEvents() {
  const auditContext = useAuditContext();
  return useStore(auditContext.storeEvents);
}
