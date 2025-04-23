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

import { delay, http, HttpResponse } from 'msw';

import cfg from 'teleport/config';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import { AuthConnectors } from './AuthConnectors';
import { connectors } from './fixtures';

export default {
  title: 'Teleport/AuthConnectors',
};

export function Loaded() {
  return (
    <TeleportProviderBasic initialEntries={[cfg.routes.sso]}>
      <AuthConnectors />
    </TeleportProviderBasic>
  );
}
Loaded.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getGithubConnectorsUrl(), () =>
        HttpResponse.json({ connectors: [connectors[0], connectors[1]] })
      ),
    ],
  },
};

export function Processing() {
  return (
    <TeleportProviderBasic initialEntries={[cfg.routes.sso]}>
      <AuthConnectors />
    </TeleportProviderBasic>
  );
}
Processing.parameters = {
  msw: {
    handlers: [
      http.get(
        cfg.getGithubConnectorsUrl(),
        async () => await delay('infinite')
      ),
    ],
  },
};

export function Empty() {
  return (
    <TeleportProviderBasic initialEntries={[cfg.routes.sso]}>
      <AuthConnectors />
    </TeleportProviderBasic>
  );
}
Empty.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getGithubConnectorsUrl(), () => HttpResponse.json({})),
    ],
  },
};

export function Failed() {
  return (
    <TeleportProviderBasic initialEntries={[cfg.routes.sso]}>
      <AuthConnectors />
    </TeleportProviderBasic>
  );
}
Failed.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getGithubConnectorsUrl(), () =>
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
