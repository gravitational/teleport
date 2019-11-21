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
import { storiesOf } from '@storybook/react';
import Audit from './Audit';
import { ReactAuditContext, AuditContext } from './useAuditContext';
import { createMemoryHistory } from 'history';

storiesOf('Teleport/Audit', module).add('Audit', () => {
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/localhost/audit/events'],
    initialIndex: 0,
  });

  const context = new AuditContext();
  context.storeEvents.fetchEvents = () => Promise.resolve([]);
  context.storeEvents.fetchLatest = () => Promise.resolve([]);

  return (
    <Router history={history}>
      <ReactAuditContext.Provider value={context}>
        <Audit />
      </ReactAuditContext.Provider>
    </Router>
  );
});
