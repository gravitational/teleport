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
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import TeleportContextProvider from 'teleport/teleportContextProvider';
import TeleportContext from 'teleport/teleportContext';
import Audit from './Audit';

export default {
  title: 'Teleport/Audit',
};

export const AuditLog = () => {
  const ctx = new TeleportContext();
  ctx.auditService.fetchEvents = () =>
    Promise.resolve({
      overflow: false,
      events: [],
    });

  return render(ctx);
};

export const Overflow = () => {
  const ctx = new TeleportContext();
  ctx.auditService.fetchEvents = () =>
    Promise.resolve({
      overflow: true,
      events: [],
    });

  return render(ctx);
};

export const Processing = () => {
  const ctx = new TeleportContext();
  ctx.auditService.fetchEvents = () => new Promise(() => null);
  return render(ctx);
};

export const Failed = () => {
  const ctx = new TeleportContext();
  ctx.auditService.fetchEvents = () =>
    Promise.reject(new Error('server error'));
  return render(ctx);
};

function render(ctx) {
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/localhost/audit/events'],
    initialIndex: 0,
  });

  return (
    <TeleportContextProvider value={ctx}>
      <Router history={history}>
        <Audit />
      </Router>
    </TeleportContextProvider>
  );
}
