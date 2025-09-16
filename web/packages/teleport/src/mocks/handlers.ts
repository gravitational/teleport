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

import { rest } from 'msw';

import cfg from 'teleport/config';
import { INTERNAL_RESOURCE_ID_LABEL_KEY } from 'teleport/services/joinToken';

// handlersTeleport defines default positive (200) response values.
export const handlersTeleport = [
  rest.post(cfg.api.joinTokenPath, (req, res, ctx) => {
    return res(
      ctx.json({
        id: 'token-id',
        suggestedLabels: [
          { name: INTERNAL_RESOURCE_ID_LABEL_KEY, value: 'resource-id' },
        ],
      })
    );
  }),
  rest.post(cfg.api.captureUserEventPath, (req, res, ctx) => {
    return res(ctx.status(200));
  }),
  rest.get(cfg.api.thumbprintPath, (req, res, ctx) => {
    return res(ctx.json('examplevaluehere'));
  }),
  rest.post(cfg.getIntegrationsUrl(), (req, res, ctx) => {
    return res(ctx.json({}));
  }),
];
