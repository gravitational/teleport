/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { enableMswServer, server } from 'design/utils/testing';

import { botsApiResponseFixture } from 'teleport/Bots/fixtures';
import { listBotsError, listBotsSuccess } from 'teleport/test/helpers/bots';

import { fetchAllBotsV1, listBots } from './bot';

enableMswServer();

test('v1 fallback pages by position when bot names repeat across scopes', async () => {
  const [bot] = botsApiResponseFixture.items;
  const bots = ['r0', 'r1', 'r2'].map(role => ({
    ...bot,
    metadata: { ...bot.metadata, name: 'robot' },
    spec: { ...bot.spec, roles: [role] },
  }));
  server.use(
    listBotsError(404, 'path not found', {
      proxyVersion: {
        major: 18,
        minor: 0,
        patch: 0,
        preRelease: '',
        string: '18.0.0',
      },
    }),
    listBotsSuccess({ items: bots }, 'v1')
  );

  const roles: string[] = [];
  let pageToken = '';
  do {
    const page = await listBots({ pageToken, pageSize: 1 }, undefined, () =>
      fetchAllBotsV1()
    );
    roles.push(...page.items.flatMap(b => b.roles));
    pageToken = page.next_page_token ?? '';
  } while (pageToken && roles.length < 10);

  expect(roles).toEqual(['r0', 'r1', 'r2']);
});
