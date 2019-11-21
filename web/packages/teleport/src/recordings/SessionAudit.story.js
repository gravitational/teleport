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
import { Router, Route } from 'shared/components/Router';
import { storiesOf } from '@storybook/react';
import SessionAudit from './SessionAudit';
import { createMemoryHistory } from 'history';

storiesOf('TeleportAudit', module).add('Audit', () => {
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/localhost/session/123'],
    initialIndex: 0,
  });

  return (
    <Router history={history}>
      <Route path="/web/cluster/:clusterId/session/:sid">
        <SessionAudit sid="123" />
      </Route>
    </Router>
  );
});
