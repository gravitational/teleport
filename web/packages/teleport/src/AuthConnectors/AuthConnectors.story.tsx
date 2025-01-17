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
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { AuthConnectors } from './AuthConnectors';
import { connectors } from './fixtures';

export default {
  title: 'Teleport/AuthConnectors',
};

export function Processing() {
  return (
    <MemoryRouter initialEntries={[cfg.routes.sso]}>
      <ContextWrapper>
        <AuthConnectors />
      </ContextWrapper>
    </MemoryRouter>
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

export function Loaded() {
  return (
    <MemoryRouter initialEntries={[cfg.routes.sso]}>
      <ContextWrapper>
        <AuthConnectors />
      </ContextWrapper>
    </MemoryRouter>
  );
}
Loaded.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getGithubConnectorsUrl(), () =>
        HttpResponse.json([connectors[0], connectors[1]])
      ),
    ],
  },
};

export function Empty() {
  return (
    <MemoryRouter initialEntries={[cfg.routes.sso]}>
      <ContextWrapper>
        <AuthConnectors />
      </ContextWrapper>
    </MemoryRouter>
  );
}
Empty.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getGithubConnectorsUrl(), () => HttpResponse.json([])),
    ],
  },
};

export function Failed() {
  return (
    <MemoryRouter initialEntries={[cfg.routes.sso]}>
      <ContextWrapper>
        <AuthConnectors />
      </ContextWrapper>
    </MemoryRouter>
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

function ContextWrapper({ children }: { children: JSX.Element }) {
  const ctx = createTeleportContext();
  return <ContextProvider ctx={ctx}>{children}</ContextProvider>;
}
