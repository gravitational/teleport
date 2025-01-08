import { createContext, PropsWithChildren, useCallback, useState } from 'react';

import AuthnDialog from 'teleport/components/AuthnDialog';
import { useMfa } from 'teleport/lib/useMfa';
import api from 'teleport/services/api/api';
import { CreateAuthenticateChallengeRequest } from 'teleport/services/auth';
import auth from 'teleport/services/auth/auth';
import { MfaChallengeResponse } from 'teleport/services/mfa';

export interface MfaContextValue {
  getMfaChallengeResponse(
    req: CreateAuthenticateChallengeRequest
  ): Promise<MfaChallengeResponse>;
}

export const MfaContext = createContext<MfaContextValue>(null);

/**
 * Provides a global MFA context to handle MFA prompts for methods outside
 * of the React scope, such as admin action API calls in auth.ts or api.ts.
 * This is intended as a workaround for such cases, and should not be used
 * for methods with access to the React scope. Use useMfa directly instead.
 */
export const MfaContextProvider = ({ children }: PropsWithChildren) => {
  const adminMfa = useMfa({});

  const getMfaChallengeResponse = useCallback(
    async (req: CreateAuthenticateChallengeRequest) => {
      const chal = await auth.getMfaChallenge(req);

      const res = await adminMfa.getChallengeResponse(chal);
      if (!res) {
        return {}; // return an empty challenge to prevent mfa retry.
      }

      return res;
    },
    [adminMfa]
  );

  const [mfaCtx] = useState<MfaContextValue>(() => {
    const mfaCtx = { getMfaChallengeResponse };
    auth.setMfaContext(mfaCtx);
    api.setMfaContext(mfaCtx);
    return mfaCtx;
  });

  return (
    <MfaContext.Provider value={mfaCtx}>
      <AuthnDialog mfaState={adminMfa}></AuthnDialog>
      {children}
    </MfaContext.Provider>
  );
};
