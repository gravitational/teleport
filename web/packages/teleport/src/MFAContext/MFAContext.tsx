import { PropsWithChildren, createContext, useContext } from 'react';

import AuthnDialog from 'teleport/components/AuthnDialog';
import { MfaState, useMfa } from 'teleport/lib/useMfa';
import { MfaChallengeScope } from 'teleport/services/auth/auth';

export interface MFAContextValue {
  adminMfa: MfaState;
}

export const MFAContext = createContext<MFAContextValue>(null);

export const useMfaContext = () => {
  return useContext(MFAContext);
};

export const MFAContextProvider = ({ children }: PropsWithChildren) => {
  const adminMfa = useMfa({
    req: {
      scope: MfaChallengeScope.ADMIN_ACTION,
      isMfaRequiredRequest: {
        admin_action: {},
      },
    },
  });

  return (
    <MFAContext.Provider value={{ adminMfa }}>
      <AuthnDialog {...adminMfa}></AuthnDialog>
      {children}
    </MFAContext.Provider>
  );
};
