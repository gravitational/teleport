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

import { Flex } from 'design';

import { createMemoryHistory } from 'history';

import { Router, Route } from 'teleport/components/Router';

import PlayerComponent from './Player';

export default {
  title: 'Teleport/Player',
};

export const SSH = () => {
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/localhost/session/123?recordingType=ssh'],
    initialIndex: 0,
  });

  return (
    <Router history={history}>
      <Flex m={-3}>
        <Route path="/web/cluster/:clusterId/session/:sid">
          <PlayerComponent />
        </Route>
      </Flex>
    </Router>
  );
};

export const Desktop = () => {
  const history = createMemoryHistory({
    initialEntries: [
      '/web/cluster/localhost/session/123?recordingType=desktop&durationMs=1234',
    ],
    initialIndex: 0,
  });

  return (
    <Router history={history}>
      <Flex m={-3}>
        <Route path="/web/cluster/:clusterId/session/:sid">
          <PlayerComponent />
        </Route>
      </Flex>
    </Router>
  );
};

export const RecordingTypeError = () => {
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/localhost/session/123?recordingType=bla'],
    initialIndex: 0,
  });

  return (
    <Router history={history}>
      <Flex m={-3}>
        <Route path="/web/cluster/:clusterId/session/:sid">
          <PlayerComponent />
        </Route>
      </Flex>
    </Router>
  );
};

export const DurationMsError = () => {
  const history = createMemoryHistory({
    initialEntries: [
      '/web/cluster/localhost/session/123?recordingType=desktop&durationMs=blabla',
    ],
    initialIndex: 0,
  });

  return (
    <Router history={history}>
      <Flex m={-3}>
        <Route path="/web/cluster/:clusterId/session/:sid">
          <PlayerComponent />
        </Route>
      </Flex>
    </Router>
  );
};
