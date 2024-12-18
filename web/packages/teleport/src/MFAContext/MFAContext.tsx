import { PropsWithChildren, createContext, useCallback, useRef } from 'react';
import AuthnDialog from 'teleport/components/AuthnDialog';
import { useMfa } from 'teleport/lib/useMfa';
import { MfaChallengeScope } from 'teleport/services/auth/auth';
import { MfaChallengeResponse } from 'teleport/services/mfa';

import { useTeleport } from '..';

export interface MFAContextValue {
  getAdminActionMfaResponse(reusable?: boolean): Promise<MfaChallengeResponse>;
}

export const MFAContext = createContext<MFAContextValue>(null);

export const MFAContextProvider = ({ children }: PropsWithChildren) => {
  const allowReuse = useRef(false);
  const adminMfa = useMfa({
    req: {
      scope: MfaChallengeScope.ADMIN_ACTION,
      allowReuse: allowReuse.current,
      isMfaRequiredRequest: {
        admin_action: {},
      },
    },
  });

  const getAdminActionMfaResponse = useCallback(
    async (reusable: boolean = false) => {
      allowReuse.current = reusable;
      return (await adminMfa.getChallengeResponse()) || {}; // return an empty challenge to prevent mfa retry.
    },
    [adminMfa, allowReuse]
  );

  const mfaCtx = { getAdminActionMfaResponse };

  const ctx = useTeleport();
  ctx.joinTokenService.setMfaContext(mfaCtx);

  return (
    <MFAContext.Provider value={mfaCtx}>
      <AuthnDialog {...adminMfa}></AuthnDialog>
      {children}
    </MFAContext.Provider>
  );
};
