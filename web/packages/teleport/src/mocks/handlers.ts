/**
 * Copyright 2022 Gravitational, Inc.
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
