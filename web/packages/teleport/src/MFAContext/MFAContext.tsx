import { createContext, PropsWithChildren, useCallback, useState } from 'react';

import AuthnDialog from 'teleport/components/AuthnDialog';
import { useMfa } from 'teleport/lib/useMfa';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import { MfaChallengeResponse } from 'teleport/services/mfa';

export interface MfaContextValue {
  getAdminActionMfaResponse(reusable?: boolean): Promise<MfaChallengeResponse>;
}

export const MfaContext = createContext<MfaContextValue>(null);

export const MfaContextProvider = ({ children }: PropsWithChildren) => {
  const adminMfa = useMfa({});

  const getAdminActionMfaResponse = useCallback(
    async (reusable: boolean = false) => {
      const chal = await auth.getMfaChallenge({
        scope: MfaChallengeScope.ADMIN_ACTION,
        allowReuse: reusable,
        isMfaRequiredRequest: {
          admin_action: {},
        },
      });

      const res = await adminMfa.getChallengeResponse(chal);
      if (!res) {
        return {}; // return an empty challenge to prevent mfa retry.
      }

      return res;
    },
    [adminMfa]
  );

  const [mfaCtx, setMfaCtx] = useState<MfaContextValue>();

  if (!mfaCtx) {
    const mfaCtx = { getAdminActionMfaResponse };
    setMfaCtx(mfaCtx);
    auth.setMfaContext(mfaCtx);
  }

  return (
    <MfaContext.Provider value={mfaCtx}>
      <AuthnDialog mfaState={adminMfa}></AuthnDialog>
      {children}
    </MfaContext.Provider>
  );
};
