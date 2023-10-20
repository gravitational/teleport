/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { MemoryRouter } from 'react-router';
import { setupServer } from 'msw/node';
import { rest } from 'msw';
import { render, fireEvent, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';
import cfg from 'teleport/config';

import { Locks } from './Locks';

test('lock search', async () => {
  const server = setupServer(
    rest.get(cfg.getLocksUrl(), (req, res, ctx) => {
      return res(
        ctx.json([
          {
            name: 'lock-name-1',
            targets: {
              user: 'lock-user',
            },
          },
          {
            name: 'lock-name-2',
            targets: {
              role: 'lock-role-1',
            },
          },
          {
            name: 'lock-name-3',
            targets: {
              role: 'lock-role-2',
            },
          },
        ])
      );
    })
  );

  server.listen();

  const ctx = createTeleportContext();

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Locks />
      </ContextProvider>
    </MemoryRouter>
  );

  const rows = await screen.findAllByText(/lock-/i);
  expect(rows).toHaveLength(3);

  // Test searching.
  fireEvent.change(screen.getByPlaceholderText(/search/i), {
    target: { value: 'lock-role' },
  });

  expect(screen.queryAllByText(/lock-role/i)).toHaveLength(2);
  expect(screen.queryByText(/lock-user/i)).not.toBeInTheDocument();

  server.close();
});
