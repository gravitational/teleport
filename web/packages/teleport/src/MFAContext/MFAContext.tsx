import {
  PropsWithChildren,
  createContext,
  useCallback,
  useContext,
  useRef,
} from 'react';

import AuthnDialog from 'teleport/components/AuthnDialog';
import { useMfa } from 'teleport/lib/useMfa';
import { MfaChallengeScope } from 'teleport/services/auth/auth';
import { MfaChallengeResponse } from 'teleport/services/mfa';

export interface MFAContextValue {
  getAdminActionMfaResponse(reusable?: boolean): Promise<MfaChallengeResponse>;
}

export const MFAContext = createContext<MFAContextValue>(null);

export const useMfaContext = () => {
  return useContext(MFAContext);
};

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
      return adminMfa.getChallengeResponse();
    },
    [adminMfa, allowReuse]
  );

  return (
    <MFAContext.Provider value={{ getAdminActionMfaResponse }}>
      <AuthnDialog {...adminMfa}></AuthnDialog>
      {children}
    </MFAContext.Provider>
  );
};
