import { PropsWithChildren, createContext, useContext, useState } from 'react';

import api from 'teleport/services/api';
import { MfaChallengeResponse } from 'teleport/services/mfa';

export interface MFAContextValue {
  get(url: string, abortSignal?: AbortSignal): Promise<any>;
  fetchWithMFARetry(
    url: string,
    customOptions: RequestInit,
    mfaResponse?: MfaChallengeResponse
  ): Promise<any>;
}

export const MFAContext = createContext<MFAContextValue>(null);

export const useMfaContext = () => {
  return useContext(MFAContext);
};

export const MFAContextProvider = ({ children }: PropsWithChildren) => {
    const [requested, setRequested] = useState({})
  const [challengeResponse, setChallengeResponse] = useState({})

  const get = (url: string, abortSignal?: AbortSignal) => {
    return fetchWithMFARetry(url, {})
  };

  const fetchWithMFARetry = (
    url: string,
    customOptions: RequestInit,
    mfaResponse?: MfaChallengeResponse
  ) => {

    const response = await api.fetch(url, customOptions)

    if (response === "bad") {
        const challenge = await api.getChallenge()
        setRequested(challenge)
    }
    // fetch the call
    // if it passes, we are done
    // else
    // fetch the challenge
    // show UI for which challenge they want to use
  }

  return (
    <MFAContext.Provider value={{ get }}>
      <button onClick={() => setRequested(true)}>request</button>
      <div>
        which challenge?
        <div onClick={() => }></div>
      </div>
      {(requested.totp || requested.sso) && <div>hiiii</div>}
      {children}
    </MFAContext.Provider>
  );
};
