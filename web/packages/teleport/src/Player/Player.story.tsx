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

import { Meta } from '@storybook/react';
import { createMemoryHistory } from 'history';
import React from 'react';

import { Flex } from 'design';

import { ContextProvider } from 'teleport';
import { Route, Router } from 'teleport/components/Router';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { Player } from './Player';

const meta: Meta = {
  title: 'Teleport/Player',
};
export default meta;

export const SSH = () => {
  const ctx = createTeleportContext();
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/localhost/session/123?recordingType=ssh'],
    initialIndex: 0,
  });

  return (
    <ContextProvider ctx={ctx}>
      <Router history={history}>
        <Flex m={-3}>
          <Route path="/web/cluster/:clusterId/session/:sid">
            <Player />
          </Route>
        </Flex>
      </Router>
    </ContextProvider>
  );
};

export const Desktop = () => {
  const ctx = createTeleportContext();
  const history = createMemoryHistory({
    initialEntries: [
      '/web/cluster/localhost/session/123?recordingType=desktop&durationMs=1234',
    ],
    initialIndex: 0,
  });

  return (
    <ContextProvider ctx={ctx}>
      <Router history={history}>
        <Flex m={-3}>
          <Route path="/web/cluster/:clusterId/session/:sid">
            <Player />
          </Route>
        </Flex>
      </Router>
    </ContextProvider>
  );
};

export const RecordingTypeError = () => {
  const ctx = createTeleportContext();
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/localhost/session/123?recordingType=bla'],
    initialIndex: 0,
  });

  return (
    <ContextProvider ctx={ctx}>
      <Router history={history}>
        <Flex m={-3}>
          <Route path="/web/cluster/:clusterId/session/:sid">
            <Player />
          </Route>
        </Flex>
      </Router>
    </ContextProvider>
  );
};

export const DurationMsError = () => {
  const ctx = createTeleportContext();
  const history = createMemoryHistory({
    initialEntries: [
      '/web/cluster/localhost/session/123?recordingType=desktop&durationMs=blabla',
    ],
    initialIndex: 0,
  });

  return (
    <ContextProvider ctx={ctx}>
      <Router history={history}>
        <Flex m={-3}>
          <Route path="/web/cluster/:clusterId/session/:sid">
            <Player />
          </Route>
        </Flex>
      </Router>
    </ContextProvider>
  );
};
