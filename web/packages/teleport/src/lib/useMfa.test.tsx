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

import { renderHook, waitFor } from '@testing-library/react';

import { CreateAuthenticateChallengeRequest } from 'teleport/services/auth';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import {
  MFA_OPTION_WEBAUTHN,
  MfaAuthenticateChallenge,
  MfaChallengeResponse,
} from 'teleport/services/mfa';

import { MfaCanceledError, useMfa } from './useMfa';

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
  beforeEach(() => {
    jest.spyOn(console, 'error').mockImplementation();
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

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
    expect(mfa.current.mfaRequired).toEqual(true);
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
    await waitFor(() => expect(mfa.current.mfaRequired).toEqual(false));
  });

  test('adaptable mfa requirement', async () => {
    jest.spyOn(auth, 'getMfaChallenge').mockResolvedValueOnce(mockChallenge);

    const { result: mfa } = renderHook(() =>
      useMfa({
        isMfaRequired: false,
        req: mockChallengeReq,
      })
    );

    // The mfa requirement can be initialized to a non-undefined value to skip
    // the mfa check when it isn't needed, e.g. the requirement was predetermined.
    expect(mfa.current.mfaRequired).toEqual(false);

    // The mfa required state can be changed directly, in case the requirement
    // need to be updated by the caller.
    mfa.current.setMfaRequired(true);
    await waitFor(() => {
      expect(mfa.current.mfaRequired).toEqual(true);
    });

    // The mfa required state can be changed back to undefined (default) or null to force
    // the next mfa attempt to re-check mfa required / attempt to get a challenge.
    mfa.current.setMfaRequired(null);
    await waitFor(() => {
      expect(mfa.current.mfaRequired).toEqual(null);
    });

    // mfa required will be updated to true as usual once the server returns an mfa challenge.
    mfa.current.getChallengeResponse();
    await waitFor(() => {
      expect(mfa.current.mfaRequired).toEqual(true);
    });
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

    const { result: mfa } = renderHook(() =>
      useMfa({
        req: mockChallengeReq,
      })
    );

    const respPromise = mfa.current.getChallengeResponse();
    await waitFor(() => {
      expect(auth.getMfaChallenge).toHaveBeenCalledWith(mockChallengeReq);
    });
    await mfa.current.submit('webauthn');

    await waitFor(() => {
      expect(mfa.current.attempt).toEqual({
        status: 'error',
        statusText: err.message,
        error: err,
        data: null,
      });
    });

    // After an error, the mfa response promise remains in an unresolved state,
    // allowing for retries.
    jest
      .spyOn(auth, 'getMfaChallengeResponse')
      .mockResolvedValueOnce(mockResponse);
    await mfa.current.submit('webauthn');
    expect(await respPromise).toEqual(mockResponse);
  });

  test('reset mfa attempt', async () => {
    jest.spyOn(auth, 'getMfaChallenge').mockResolvedValue(mockChallenge);
    const { result: mfa } = renderHook(() =>
      useMfa({
        req: mockChallengeReq,
      })
    );

    const respPromise = mfa.current.getChallengeResponse();
    await waitFor(() => {
      expect(auth.getMfaChallenge).toHaveBeenCalled();
    });

    mfa.current.cancelAttempt();

    await expect(respPromise).rejects.toThrow(new MfaCanceledError());

    await waitFor(() => {
      expect(mfa.current.attempt.status).toEqual('error');
    });
    expect(
      mfa.current.attempt.status === 'error' && mfa.current.attempt.error
    ).toEqual(new MfaCanceledError());
  });
});
