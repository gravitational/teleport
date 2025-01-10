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

import { createMemoryHistory } from 'history';

import { Flex } from 'design';

import { Route, Router } from 'teleport/components/Router';

import { Player } from './Player';

export default {
  title: 'Teleport/Player',
};

export const SSH = () => {
  const history = createMemoryHistory({
    initialEntries: [
      '/web/cluster/localhost/session/123?recordingType=ssh&durationMs=1234',
    ],
    initialIndex: 0,
  });

  return (
    <Router history={history}>
      <Flex m={-3}>
        <Route path="/web/cluster/:clusterId/session/:sid">
          <Player />
        </Route>
      </Flex>
    </Router>
  );
};

// SSH player attempts to write to a web socket, and currently, there's no
// official support for web sockets in MSW (see
// https://github.com/mswjs/msw/issues/156).
SSH.tags = ['skip-test'];

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
          <Player />
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
          <Player />
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
          <Player />
        </Route>
      </Flex>
    </Router>
  );
};
