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
import { http, HttpResponse } from 'msw';
import { useState } from 'react';
import { Route, Router } from 'react-router';

import { Flex } from 'design';

import cfg from 'teleport/config';

import Console from './Console';
import ConsoleContext from './consoleContext';
import ConsoleContextProvider from './consoleContextProvider';

export default {
  title: 'Teleport/Console',
};

export const ConsoleStory = () => {
  const ctx = new ConsoleContext();
  return (
    <TestLayout ctx={ctx}>
      <Console />
    </TestLayout>
  );
};
ConsoleStory.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getUserContextUrl(), () => {
        return HttpResponse.json({
          cluster: {
            name: 'aws',
            lastConnected: new Date('2020-09-26T17:30:23.512876876Z'),
            status: 'online',
            publicURL: 'localhost',
            authVersion: '4.4.0-dev',
            proxyVersion: '4.4.0-dev',
          },
        });
      }),
    ],
  },
};

export function TestLayout(props: PropType) {
  const [context] = useState((): ConsoleContext => {
    return props.ctx || new ConsoleContext();
  });

  const [history] = useState((): any => {
    const history =
      props.history ||
      createMemoryHistory({
        initialEntries: ['/clusterX'],
        initialIndex: 0,
      });

    return history;
  });

  return (
    <Router history={history}>
      <Route path="/:clusterId">
        <ConsoleContextProvider value={context}>
          <Flex
            m={-3}
            style={{ position: 'absolute' }}
            width="100%"
            height="100%"
          >
            {props.children}
          </Flex>
        </ConsoleContextProvider>
      </Route>
    </Router>
  );
}

type PropType = {
  children: any;
  ctx?: ConsoleContext;
  history?: History;
};
