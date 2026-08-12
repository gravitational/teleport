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

import { act, renderHook, waitFor } from '@testing-library/react';

import cfg from 'teleport/config';
import auth from 'teleport/services/auth';
import history from 'teleport/services/history';
import { storageService } from 'teleport/services/storageService';
import session from 'teleport/services/websession';

import useLogin from './useLogin';

beforeEach(() => {
  jest.restoreAllMocks();
  jest.spyOn(session, 'isValid').mockImplementation(() => true);
  jest.spyOn(history, 'push').mockImplementation();
  jest.spyOn(history, 'replace').mockImplementation();
  jest.mock('shared/hooks', () => ({
    useAttempt: () => {
      return [
        { status: 'success', statusText: 'Success Text' },
        {
          clear: jest.fn(),
        },
      ];
    },
  }));
});

afterEach(() => {
  jest.resetAllMocks();
});

it('redirect to root on path not matching "/enterprise/saml-idp/sso"', () => {
  jest.spyOn(history, 'getRedirectParam').mockReturnValue('http://localhost');
  renderHook(() => useLogin());
  expect(history.replace).toHaveBeenCalledWith('/web');

  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue('http://localhost/web/cluster/name/resources');
  renderHook(() => useLogin());
  expect(history.replace).toHaveBeenCalledWith('/web');
});

it('redirect to SAML SSO path on matching "/enterprise/saml-idp/sso"', () => {
  const samlIdpPath = new URL('http://localhost' + cfg.routes.samlIdpSso);
  cfg.baseUrl = 'http://localhost';
  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue(samlIdpPath.toString());
  renderHook(() => useLogin());
  expect(history.push).toHaveBeenCalledWith(samlIdpPath.toString(), true);
});

it('non-base domain redirects with base domain for a matching "/enterprise/saml-idp/sso"', async () => {
  const samlIdpPath = new URL('http://different-base' + cfg.routes.samlIdpSso);
  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue(samlIdpPath.toString());
  renderHook(() => useLogin());
  const expectedPath = new URL('http://localhost' + cfg.routes.samlIdpSso);
  expect(history.push).toHaveBeenCalledWith(expectedPath.toString(), true);
});

it('base domain with different path is redirected to root', async () => {
  const nonSamlIdpPath = new URL('http://localhost/web/cluster/name/resources');
  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue(nonSamlIdpPath.toString());
  renderHook(() => useLogin());
  expect(history.replace).toHaveBeenCalledWith('/web');
});

it('invalid session does nothing', async () => {
  const samlIdpPathWithDifferentBase = new URL(
    'http://different-base' + cfg.routes.samlIdpSso
  );
  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue(samlIdpPathWithDifferentBase.toString());
  jest.spyOn(session, 'isValid').mockImplementation(() => false);
  renderHook(() => useLogin());
  expect(history.replace).not.toHaveBeenCalled();
  expect(history.push).not.toHaveBeenCalled();
});

describe('passkey auto-prompt', () => {
  function setup({
    hasValidSession,
    passwordlessEnabled,
    hasLoggedInWithPasskey,
    autoPromptDisabled,
  }: {
    hasValidSession: boolean;
    passwordlessEnabled: boolean;
    hasLoggedInWithPasskey: boolean;
    autoPromptDisabled: boolean;
  }) {
    jest.spyOn(session, 'isValid').mockReturnValue(hasValidSession);
    jest.spyOn(history, 'getRedirectParam').mockReturnValue(undefined);
    jest
      .spyOn(cfg, 'isPasswordlessEnabled')
      .mockReturnValue(passwordlessEnabled);
    jest
      .spyOn(storageService, 'getHasLoggedInWithPasskey')
      .mockReturnValue(hasLoggedInWithPasskey);
    jest
      .spyOn(storageService, 'getPasskeyAutoPromptDisabled')
      .mockReturnValue(autoPromptDisabled);
    return jest
      .spyOn(auth, 'loginWithWebauthn')
      .mockResolvedValue(
        {} as Awaited<ReturnType<typeof auth.loginWithWebauthn>>
      );
  }

  it('fires once when passwordless is enabled, the browser has used a passkey, and it is not disabled', async () => {
    const loginSpy = setup({
      hasValidSession: false,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: true,
      autoPromptDisabled: false,
    });

    const { result } = renderHook(() => useLogin());
    // The ceremony resolves on its own; let it settle before asserting.
    await waitFor(() => expect(result.current.autoPromptPending).toBe(false));

    expect(loginSpy).toHaveBeenCalledTimes(1);
    expect(loginSpy).toHaveBeenCalledWith(undefined, expect.any(AbortSignal));
  });

  it('does not fire when the auto-prompt is disabled', () => {
    const loginSpy = setup({
      hasValidSession: false,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: true,
      autoPromptDisabled: true,
    });

    renderHook(() => useLogin());

    expect(loginSpy).not.toHaveBeenCalled();
  });

  it('does not fire when the browser has never used a passkey', () => {
    const loginSpy = setup({
      hasValidSession: false,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: false,
      autoPromptDisabled: false,
    });

    renderHook(() => useLogin());

    expect(loginSpy).not.toHaveBeenCalled();
  });

  it('does not fire when a valid session already exists', () => {
    const loginSpy = setup({
      hasValidSession: true,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: true,
      autoPromptDisabled: false,
    });

    renderHook(() => useLogin());

    expect(loginSpy).not.toHaveBeenCalled();
  });

  it('waits for the message of the day to be acknowledged', async () => {
    const loginSpy = setup({
      hasValidSession: false,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: true,
      autoPromptDisabled: false,
    });
    jest.spyOn(cfg, 'getMotd').mockReturnValue('read me first');

    const { result } = renderHook(() => useLogin());

    // Signing in behind the MOTD would authorize the user past a notice they never saw.
    expect(loginSpy).not.toHaveBeenCalled();

    act(() => result.current.acknowledgeMotd());
    expect(loginSpy).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(result.current.autoPromptPending).toBe(false));
  });

  it('does not fire while an unacknowledged community license is shown', () => {
    const loginSpy = setup({
      hasValidSession: false,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: true,
      autoPromptDisabled: false,
    });
    jest.spyOn(storageService, 'getLicenseAcknowledged').mockReturnValue(false);
    const edition = cfg.edition;
    cfg.edition = 'community';

    try {
      renderHook(() => useLogin());
    } finally {
      cfg.edition = edition;
    }

    expect(loginSpy).not.toHaveBeenCalled();
  });

  it('never claims the form attempt state', async () => {
    const loginSpy = setup({
      hasValidSession: false,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: true,
      autoPromptDisabled: false,
    });
    let rejectCeremony: (err: Error) => void;
    loginSpy.mockReturnValue(
      new Promise((_, reject) => {
        rejectCeremony = reject;
      }) as ReturnType<typeof auth.loginWithWebauthn>
    );

    const { result } = renderHook(() => useLogin());

    expect(loginSpy).toHaveBeenCalledTimes(1);
    // The form stays usable while the dialog is open, since the user never asked for it.
    expect(result.current.attempt.isProcessing).toBe(false);

    // A user-initiated attempt is in flight by the time the auto-prompt is dismissed. It never settles
    // here, so the assertions below see exactly what the auto-prompt did to it.
    jest
      .spyOn(auth, 'login')
      .mockReturnValue(new Promise(() => {}) as ReturnType<typeof auth.login>);
    act(() => result.current.onLogin('llama', 'secret', ''));
    expect(result.current.attempt.isProcessing).toBe(true);

    await act(async () => {
      rejectCeremony(new Error('The operation was not allowed'));
    });

    // The rejected auto-prompt must not report a failure or discard the login in flight.
    expect(result.current.attempt.isFailed).toBe(false);
    expect(result.current.attempt.isProcessing).toBe(true);
  });

  it('holds back only the passkey button while the ceremony is outstanding', async () => {
    const loginSpy = setup({
      hasValidSession: false,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: true,
      autoPromptDisabled: false,
    });
    let settleCeremony: () => void;
    loginSpy.mockReturnValue(
      new Promise<never>((_, reject) => {
        settleCeremony = () => reject(new Error('dismissed'));
      }) as ReturnType<typeof auth.loginWithWebauthn>
    );

    const { result } = renderHook(() => useLogin());

    expect(result.current.autoPromptPending).toBe(true);
    // Only the passkey button is held back; the password form is left alone.
    expect(result.current.attempt.isProcessing).toBe(false);

    await act(async () => settleCeremony());
    expect(result.current.autoPromptPending).toBe(false);
  });

  it.each([
    {
      method: 'a password',
      start: (r: ReturnType<typeof useLogin>) =>
        r.onLogin('llama', 'secret', ''),
    },
    {
      method: 'a passkey',
      start: (r: ReturnType<typeof useLogin>) => r.onLoginWithWebauthn(),
    },
  ])(
    'aborts the outstanding ceremony when the user signs in with $method',
    async ({ start }) => {
      const loginSpy = setup({
        hasValidSession: false,
        passwordlessEnabled: true,
        hasLoggedInWithPasskey: true,
        autoPromptDisabled: false,
      });
      // Never settles: the ceremony is outstanding until it gets aborted.
      loginSpy.mockReturnValue(
        new Promise(() => {}) as ReturnType<typeof auth.loginWithWebauthn>
      );
      jest
        .spyOn(auth, 'login')
        .mockReturnValue(
          new Promise(() => {}) as ReturnType<typeof auth.login>
        );

      const { result } = renderHook(() => useLogin());
      const signal = loginSpy.mock.calls[0][1];
      expect(signal.aborted).toBe(false);

      act(() => start(result.current));

      // A browser rejects whichever ceremony comes second, so the automatic one has to stand down.
      expect(signal.aborted).toBe(true);
      expect(result.current.autoPromptPending).toBe(false);
    }
  );

  it('aborts the outstanding ceremony when the page goes away', () => {
    const loginSpy = setup({
      hasValidSession: false,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: true,
      autoPromptDisabled: false,
    });
    loginSpy.mockReturnValue(
      new Promise(() => {}) as ReturnType<typeof auth.loginWithWebauthn>
    );

    const { unmount } = renderHook(() => useLogin());
    const signal = loginSpy.mock.calls[0][1];

    unmount();
    expect(signal.aborted).toBe(true);
  });

  it('does not clear the login time when the ceremony is rejected', async () => {
    const loginSpy = setup({
      hasValidSession: false,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: true,
      autoPromptDisabled: false,
    });
    loginSpy.mockRejectedValue(new Error('The operation was not allowed'));
    const clearLoginTime = jest
      .spyOn(storageService, 'clearLoginTime')
      .mockImplementation();

    renderHook(() => useLogin());
    await waitFor(() => expect(loginSpy).toHaveBeenCalledTimes(1));

    expect(clearLoginTime).not.toHaveBeenCalled();
  });

  it('clears the login time when the ceremony succeeds', async () => {
    setup({
      hasValidSession: false,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: true,
      autoPromptDisabled: false,
    });
    const clearLoginTime = jest
      .spyOn(storageService, 'clearLoginTime')
      .mockImplementation();

    renderHook(() => useLogin());
    await waitFor(() => expect(clearLoginTime).toHaveBeenCalledTimes(1));
  });

  it('still surfaces the error when the user starts the ceremony', async () => {
    const loginSpy = setup({
      hasValidSession: false,
      passwordlessEnabled: true,
      hasLoggedInWithPasskey: false,
      autoPromptDisabled: false,
    });
    loginSpy.mockRejectedValue(new Error('The operation was not allowed'));
    // useAttempt logs through the design logger on error.
    jest.spyOn(console, 'error').mockImplementation();

    const { result } = renderHook(() => useLogin());
    await act(async () => {
      result.current.onLoginWithWebauthn();
    });

    expect(result.current.attempt.isFailed).toBe(true);
    expect(result.current.attempt.message).toBe(
      'The operation was not allowed'
    );
  });
});

describe('recording passkey usage on login', () => {
  function setup() {
    jest.spyOn(session, 'isValid').mockReturnValue(false);
    jest.spyOn(history, 'getRedirectParam').mockReturnValue(undefined);
    jest.spyOn(cfg, 'isPasswordlessEnabled').mockReturnValue(true);
    // Keep the auto-prompt from firing so only the explicit calls are exercised.
    jest
      .spyOn(storageService, 'getHasLoggedInWithPasskey')
      .mockReturnValue(false);
    jest
      .spyOn(storageService, 'getPasskeyAutoPromptDisabled')
      .mockReturnValue(false);
    jest
      .spyOn(auth, 'loginWithWebauthn')
      .mockResolvedValue(
        {} as Awaited<ReturnType<typeof auth.loginWithWebauthn>>
      );
    return jest
      .spyOn(storageService, 'setHasLoggedInWithPasskey')
      .mockImplementation();
  }

  it('records passkey usage on a passwordless login success', async () => {
    const setHasLogged = setup();

    const { result } = renderHook(() => useLogin());
    await act(async () => {
      result.current.onLoginWithWebauthn();
    });

    expect(setHasLogged).toHaveBeenCalledTimes(1);
  });

  it('does not record passkey usage on a username/password webauthn login', async () => {
    const setHasLogged = setup();

    const { result } = renderHook(() => useLogin());
    await act(async () => {
      result.current.onLoginWithWebauthn({
        username: 'llama',
        password: 'secret',
      });
    });

    expect(setHasLogged).not.toHaveBeenCalled();
  });
});
