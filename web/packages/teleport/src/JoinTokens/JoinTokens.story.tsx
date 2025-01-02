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

import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { JoinToken } from 'teleport/services/joinToken';

import { JoinTokens } from './JoinTokens';

export default {
  title: 'Teleport/JoinTokens',
};

export const Loaded = () => (
  <Provider>
    <JoinTokens />
  </Provider>
);

Loaded.parameters = {
  msw: {
    handlers: [
      http.get(cfg.api.joinTokensPath, () => {
        return HttpResponse.json({ items: tokens });
      }),
      http.put(cfg.api.joinTokenYamlPath, () => {
        return HttpResponse.json(editedToken);
      }),
    ],
  },
};

const Provider = ({ children }) => {
  const ctx = createTeleportContext();

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>{children}</ContextProvider>
    </MemoryRouter>
  );
};

const tokens: JoinToken[] = [
  {
    id: 'token1',
    roles: ['Node'],
    isStatic: true,
    expiry: new Date('0001-01-01'),
    method: 'token',
    safeName: '******',
    allow: [],
    gcp: {
      allow: [],
    },
    content: '',
  },
  {
    id: 'iam-EDIT-ME-BUT-DONT-SAVE',
    roles: ['Node', 'App', 'Trusted_cluster'],
    isStatic: false,
    expiry: new Date('2023-06-01'),
    method: 'iam',
    safeName: 'iam-EDIT-ME-BUT-DONT-SAVE',
    allow: [],
    gcp: {
      allow: [],
    },
    content: `kind: token
        metadata:
        name: iam-EDIT-ME-BUT-DONT-SAVE
        revision: d018f5e3-774f-43b5-a349-4529147eab2c
        spec:
        gcp:
            allow:
            - project_ids:
            - "123123"
        join_method: iam
        roles:
        - Node
        version: v2
    `,
  },
];

const editedToken: JoinToken = {
  id: 'iam-I-SAID-DONT-SAVE',
  roles: ['Node', 'App', 'Trusted_cluster'],
  isStatic: false,
  expiry: new Date(),
  method: 'iam',
  safeName: 'iam-I-SAID-DONT-SAVE',
  content: `kind: token
            metadata:
            name: iam-I-SAID-DONT-SAVE
            revision: d018f5e3-774f-43b5-a349-4529147eab2c
            spec:
            gcp:
                allow:
                - project_ids:
                - "123123"
            join_method: iam
            roles:
            - Node
            version: v2
        `,
};
