import { AccessRequest } from 'shared/services/accessRequests';

import TeleportContextE from 'e-teleport/teleportContextE';

export function getBaseRequestFlags(
  request: AccessRequest,
  ctx: TeleportContextE
) {
  const ownRequest = request.user === ctx.storeUser.getUsername();
  const canAssume = ownRequest && request.state === 'APPROVED';
  const isAssumed =
    ownRequest && !!ctx.storeAccessRequests.isAssumed(request.id);

  const isPromoted = request.state === 'PROMOTED';

  return {
    // canAssume is a flag to show the assume btn.
    canAssume,
    // isAssumed is a flag if the assume btn should be disabled or not,
    // and determines the text that implies if user already has assumed or not.
    isAssumed,
    ownRequest,
    isPromoted,
  };
}
