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
import { Router, Route } from 'teleport/components/Router';
import PlayerComponent from './Player';
import { createMemoryHistory } from 'history';

export default {
  title: 'TeleportPlayerEnhanced',
};

export const Player = () => {
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/localhost/session/123'],
    initialIndex: 0,
  });

  return (
    <Router history={history}>
      <Route path="/web/cluster/:clusterId/session/:sid">
        <PlayerComponent />
      </Route>
    </Router>
  );
};
