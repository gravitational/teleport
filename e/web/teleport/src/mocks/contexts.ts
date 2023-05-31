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

import makeUserContext from 'teleport/services/user/makeUserContext';
import { baseContext } from 'teleport/mocks/contexts';

import TeleportContextE from 'e-teleport/teleportContextE';

import type { Acl } from 'teleport/services/user/types';

export function createTeleportContextE(cfg?: { customAcl?: Acl }) {
  cfg = cfg || {};
  const ctx = new TeleportContextE();
  const userCtx = makeUserContext(baseContext);

  if (cfg.customAcl) {
    userCtx.acl = cfg.customAcl;
  }

  ctx.storeUser.setState(userCtx);

  return ctx;
}
