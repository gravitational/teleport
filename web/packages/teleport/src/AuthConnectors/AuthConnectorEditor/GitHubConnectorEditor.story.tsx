/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { delay, http, HttpResponse } from 'msw';
import { Route } from 'react-router';

import cfg from 'teleport/config';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import { connectors } from '../fixtures';
import { GitHubConnectorEditor } from './GitHubConnectorEditor';

export default {
  title: 'Teleport/AuthConnectors/GitHubConnectorEditor',
};

export function Loaded() {
  return (
    <TeleportProviderBasic
      initialEntries={[
        cfg.getEditAuthConnectorRoute('github', 'github_connector'),
      ]}
    >
      <Route path={cfg.routes.ssoConnector.edit}>
        <GitHubConnectorEditor />
      </Route>
    </TeleportProviderBasic>
  );
}
Loaded.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getGithubConnectorUrl('github_connector'), () =>
        HttpResponse.json(connectors[0])
      ),
    ],
  },
};

export function Processing() {
  return (
    <TeleportProviderBasic
      initialEntries={[
        cfg.getEditAuthConnectorRoute('github', 'github_connector'),
      ]}
    >
      <Route path={cfg.routes.ssoConnector.edit}>
        <GitHubConnectorEditor />
      </Route>
    </TeleportProviderBasic>
  );
}
Processing.parameters = {
  msw: {
    handlers: [
      http.get(
        cfg.getGithubConnectorUrl('github_connector'),
        async () => await delay('infinite')
      ),
    ],
  },
};

export function Failed() {
  return (
    <TeleportProviderBasic
      initialEntries={[
        cfg.getEditAuthConnectorRoute('github', 'github_connector'),
      ]}
    >
      <Route path={cfg.routes.ssoConnector.edit}>
        <GitHubConnectorEditor />
      </Route>
    </TeleportProviderBasic>
  );
}
Failed.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getGithubConnectorUrl('github_connector'), () =>
        HttpResponse.json(
          { message: 'something went wrong' },
          {
            status: 500,
          }
        )
      ),
    ],
  },
};
