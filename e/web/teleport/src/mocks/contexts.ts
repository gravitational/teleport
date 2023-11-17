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
