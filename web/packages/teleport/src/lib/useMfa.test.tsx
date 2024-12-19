/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';

import { renderHook, waitFor } from '@testing-library/react';
import { useState } from 'react';

import { CreateAuthenticateChallengeRequest } from 'teleport/services/auth';
import {
  MFA_OPTION_WEBAUTHN,
  MfaAuthenticateChallenge,
  MfaChallengeResponse,
} from 'teleport/services/mfa';

import { useMfa } from './useMfa';

const mockChallenge: MfaAuthenticateChallenge = {
  webauthnPublicKey: {} as PublicKeyCredentialRequestOptions,
};

const mockResponse: MfaChallengeResponse = {
  webauthn_response: {
    id: 'cred-id',
    type: 'public-key',
    extensions: {
      appid: true,
    },
    rawId: 'rawId',
    response: {
      authenticatorData: 'authenticatorData',
      clientDataJSON: 'clientDataJSON',
      signature: 'signature',
      userHandle: 'userHandle',
    },
  },
};

const mockChallengeReq: CreateAuthenticateChallengeRequest = {
  scope: MfaChallengeScope.USER_SESSION,
  isMfaRequiredRequest: {
    node: {
      node_name: 'node',
      login: 'login',
    },
  },
};

describe('useMfa', () => {
  beforeEach(() => jest.spyOn(console, 'error').mockImplementation());

  test('mfa required', async () => {
    jest.spyOn(auth, 'getMfaChallenge').mockResolvedValueOnce(mockChallenge);
    jest
      .spyOn(auth, 'getMfaChallengeResponse')
      .mockResolvedValueOnce(mockResponse);
    const { result: mfa } = renderHook(() =>
      useMfa({
        req: mockChallengeReq,
      })
    );

    const respPromise = mfa.current.getChallengeResponse();
    await waitFor(() => {
      expect(auth.getMfaChallenge).toHaveBeenCalledWith(mockChallengeReq);
    });

    expect(mfa.current.options).toEqual([MFA_OPTION_WEBAUTHN]);
    expect(mfa.current.required).toEqual(true);
    expect(mfa.current.challenge).toEqual(mockChallenge);
    expect(mfa.current.attempt.status).toEqual('processing');

    await mfa.current.submit('webauthn');
    await waitFor(() => {
      expect(auth.getMfaChallengeResponse).toHaveBeenCalledWith(
        mockChallenge,
        'webauthn',
        undefined
      );
    });

    const resp = await respPromise;
    expect(resp).toEqual(mockResponse);
    expect(mfa.current.challenge).toEqual(null);
    expect(mfa.current.attempt.status).toEqual('success');
  });

  test('mfa not required', async () => {
    jest.spyOn(auth, 'getMfaChallenge').mockResolvedValue(null);

    const { result: mfa } = renderHook(() =>
      useMfa({
        req: mockChallengeReq,
      })
    );

    // If a challenge is not returned, an empty mfa response should be returned
    // early and the requirement changed to false for future calls.
    const resp = await mfa.current.getChallengeResponse();
    expect(auth.getMfaChallenge).toHaveBeenCalledWith(mockChallengeReq);
    expect(resp).toEqual(undefined);
    await waitFor(() => expect(mfa.current.required).toEqual(false));
  });

  test('adaptable mfa requirement state', async () => {
    jest.spyOn(auth, 'getMfaChallenge').mockResolvedValue(null);

    let isMfaRequired: boolean;
    let setMfaRequired: (b: boolean) => void;

    let req: CreateAuthenticateChallengeRequest;
    let setReq: (r: CreateAuthenticateChallengeRequest) => void;

    const { result: mfa } = renderHook(() => {
      [isMfaRequired, setMfaRequired] = useState<boolean>(null);
      [req, setReq] =
        useState<CreateAuthenticateChallengeRequest>(mockChallengeReq);

      return useMfa({
        req: req,
        isMfaRequired: isMfaRequired,
      });
    });

    // mfaRequired should change when the isMfaRequired arg changes, allowing
    // callers to propagate mfa required late (e.g. per-session MFA for file transfers)
    setMfaRequired(false);
    await waitFor(() => expect(mfa.current.required).toEqual(false));

    setMfaRequired(true);
    await waitFor(() => expect(mfa.current.required).toEqual(true));

    setMfaRequired(null);
    await waitFor(() => expect(mfa.current.required).toEqual(null));

    // If isMfaRequiredRequest changes, the mfaRequired value should be reset.
    setReq({
      ...mockChallengeReq,
      isMfaRequiredRequest: {
        admin_action: {},
      },
    });
    await waitFor(() => expect(mfa.current.required).toEqual(null));
  });

  test('mfa challenge error', async () => {
    const err = new Error('an error has occurred');
    jest.spyOn(auth, 'getMfaChallenge').mockImplementation(() => {
      throw err;
    });

    const { result: mfa } = renderHook(() => useMfa({}));

    await expect(mfa.current.getChallengeResponse).rejects.toThrow(err);
    await waitFor(() => {
      expect(mfa.current.attempt).toEqual({
        status: 'error',
        statusText: err.message,
        error: err,
        data: null,
      });
    });
  });

  test('mfa response error', async () => {
    const err = new Error('an error has occurred');
    jest.spyOn(auth, 'getMfaChallenge').mockResolvedValueOnce(mockChallenge);
    jest.spyOn(auth, 'getMfaChallengeResponse').mockImplementation(async () => {
      throw err;
    });

    const { result: mfa } = renderHook(() => useMfa({}));

    const respPromise = mfa.current.getChallengeResponse();
    await waitFor(() => {
      expect(auth.getMfaChallenge).toHaveBeenCalledWith(mockChallengeReq);
    });
    await mfa.current.submit('webauthn');

    await expect(respPromise).rejects.toThrow(err);
    await waitFor(() => {
      expect(mfa.current.attempt).toEqual({
        status: 'error',
        statusText: err.message,
        error: err,
        data: null,
      });
    });
  });
});
