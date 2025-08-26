/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import cfg from 'teleport/config';
import { ApiBot, EditBotRequest } from 'teleport/services/bot/types';
import { JsonObject } from 'teleport/types';

export const getBotSuccess = (overrides?: {
  name?: ApiBot['metadata']['name'];
  roles?: ApiBot['spec']['roles'];
  traits?: ApiBot['spec']['traits'];
  max_session_ttl?: ApiBot['spec']['max_session_ttl'];
}) => {
  const {
    name = 'test-bot-name',
    roles = ['admin', 'user'],
    traits = [
      {
        name: 'trait-1',
        values: ['value-1', 'value-2', 'value-3'],
      },
    ],
    max_session_ttl = {
      seconds: 43200,
    },
  } = overrides ?? {};

  return http.get(cfg.api.bot.read, () => {
    return HttpResponse.json({
      status: 'active',
      kind: 'bot',
      subKind: '',
      version: 'v1',
      metadata: {
        name,
        description: '',
        labels: new Map(),
        namespace: '',
        revision: '',
      },
      spec: {
        roles,
        traits,
        max_session_ttl,
      },
    });
  });
};

/**
 * `editBotSuccess` returns a handler that captures the request and uses its values
 * to construct a new bot object. `overrides` can be used to replace values in the request.
 *
 * @param overrides values to use instead of the values from the captured request
 * @returns http handler to use in SetupServerApi.use()
 */
export const editBotSuccess = (
  version: 1 | 2 = 2,
  overrides?: Partial<EditBotRequest>
) =>
  http.put<{ botName: string }>(
    version === 1 ? cfg.api.bot.update : cfg.api.bot.updateV2,
    async ({ request, params }) => {
      const req = (await request.clone().json()) as EditBotRequest;
      const {
        roles = req.roles,
        traits = req.traits,
        max_session_ttl = req.max_session_ttl,
      } = overrides ?? {};

      const maxSessionTtlSeconds =
        max_session_ttl === '12h30m' ? 43200 + 30 * 60 : 43200;

      return HttpResponse.json({
        status: 'active',
        kind: 'bot',
        subKind: '',
        version: 'v1',
        metadata: {
          name: params.botName,
          description: '',
          labels: new Map(),
          namespace: '',
          revision: '',
        },
        spec: {
          roles: roles ?? ['admin', 'user'],
          traits: traits ?? [
            {
              name: 'trait-1',
              values: ['value-1', 'value-2', 'value-3'],
            },
          ],
          max_session_ttl: {
            seconds: maxSessionTtlSeconds,
          },
        },
      });
    }
  );

export const deleteBotSuccess = () =>
  http.delete(cfg.api.bot.delete, () => {
    return HttpResponse.json({});
  });

export const deleteBotError = (status: number, error: string | null = null) =>
  http.delete(cfg.api.bot.delete, () => {
    return HttpResponse.json({ error: { message: error } }, { status });
  });

export const getBotError = (status: number, error: string | null = null) =>
  http.get(cfg.api.bot.read, () => {
    return HttpResponse.json({ error: { message: error } }, { status });
  });

export const editBotError = (
  status: number,
  error: string | null = null,
  fields: JsonObject = {}
) =>
  http.put(cfg.api.bot.updateV2, () => {
    return HttpResponse.json({ error: { message: error }, fields }, { status });
  });

export const getBotForever = () =>
  http.get(
    cfg.api.bot.read,
    () =>
      new Promise(() => {
        /* never resolved */
      })
  );

export const editBotForever = () =>
  http.put(
    cfg.api.bot.updateV2,
    () =>
      new Promise(() => {
        /* never resolved */
      })
  );
