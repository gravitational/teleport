import React, { createContext, useContext, useCallback } from 'react';
import { Attempt, useAsync } from 'shared/hooks/useAsync';

import useTeleportE from 'e-teleport/useTeleportE';

import type { SamlIdpMetadataResponse } from 'e-teleport/services/idp/types';

/**
 * SamlApplication defines type for Saml application create action.
 */
export interface SamlApplication {
  runFetchMetadataValues: () => Promise<[SamlIdpMetadataResponse, Error]>;
  fetchMetadataValuesAttempt: Attempt<SamlIdpMetadataResponse>;
}

export const SamlApplicationContext = createContext<SamlApplication>(null);

export function useSamlApplication() {
  const context = useContext(SamlApplicationContext);
  if (!context) {
    throw new Error(
      'SamlApplicationContext must be used within a SamlApplicationProvider'
    );
  }
  return context;
}

/**
 * SamlApplicationProvider provides context for SamlApplication create action.
 * For the edit and delete action which is based on unified resource view,
 * use SamlAppActionProvider (useSamlAppActionsE.tsx).
 */
export function SamlApplicationProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const { idpService } = useTeleportE();
  const [fetchMetadataValuesAttempt, runFetchMetadataValues] = useAsync(
    useCallback(() => idpService.getIdPMetadataValues(), [idpService])
  );

  const value: SamlApplication = {
    runFetchMetadataValues,
    fetchMetadataValuesAttempt,
  };

  return (
    <SamlApplicationContext.Provider value={value}>
      {children}
    </SamlApplicationContext.Provider>
  );
}
