/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { useState, useEffect, useRef } from 'react';
import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { assertUnreachable } from 'teleterm/ui/utils';
import { RootClusterUri } from 'teleterm/ui/uri';
import { cloneAbortSignal } from 'teleterm/services/tshd/cloneableClient';

import type * as types from 'teleterm/ui/services/clusters/types';

export default function useClusterLogin(props: Props) {
  const { onSuccess, clusterUri } = props;
  const { clustersService } = useAppContext();
  const cluster = clustersService.findCluster(clusterUri);
  const refAbortCtrl = useRef<AbortController>(null);
  const loggedInUserName =
    props.prefill.username || cluster.loggedInUser?.name || null;
  const [shouldPromptSsoStatus, promptSsoStatus] = useState(false);
  const [webauthnLogin, setWebauthnLogin] = useState<WebauthnLogin>();

  const [initAttempt, init] = useAsync(async () => {
    const authSettings = await clustersService.getAuthSettings(clusterUri);

    if (authSettings.preferredMfa === 'u2f') {
      throw new Error(`the U2F API for hardware keys is deprecated, \
        please notify your system administrator to update cluster \
        settings to use WebAuthn as the second factor protocol.`);
    }

    return authSettings;
  });

  const [loginAttempt, login, setAttempt] = useAsync(
    (params: types.LoginParams) => {
      refAbortCtrl.current = new AbortController();
      switch (params.kind) {
        case 'local':
          return clustersService.loginLocal(
            params,
            cloneAbortSignal(refAbortCtrl.current.signal)
          );
        case 'passwordless':
          return clustersService.loginPasswordless(
            params,
            cloneAbortSignal(refAbortCtrl.current.signal)
          );
        case 'sso':
          return clustersService.loginSso(
            params,
            cloneAbortSignal(refAbortCtrl.current.signal)
          );
        default:
          assertUnreachable(params);
      }
    }
  );

  const onLoginWithLocal = (
    username: string,
    password: string,
    token: string,
    secondFactor?: types.Auth2faType
  ) => {
    if (secondFactor === 'webauthn') {
      setWebauthnLogin({ prompt: 'tap' });
    }

    login({
      kind: 'local',
      clusterUri,
      username,
      password,
      token,
    });
  };

  const onLoginWithPasswordless = () => {
    login({
      kind: 'passwordless',
      clusterUri,
      onPromptCallback: (prompt: types.WebauthnLoginPrompt) => {
        const newLogin: WebauthnLogin = {
          prompt: prompt.type,
          processing: false,
        };

        if (prompt.type === 'pin') {
          newLogin.onUserResponse = (pin: string) => {
            setWebauthnLogin({
              ...newLogin,
              // prevent user from clicking on submit buttons more than once
              processing: true,
            });
            prompt.onUserResponse(pin);
          };
        }

        if (prompt.type === 'credential') {
          newLogin.loginUsernames = prompt.data.credentials.map(
            c => c.username
          );
          newLogin.onUserResponse = (index: number) => {
            setWebauthnLogin({
              ...newLogin,
              // prevent user from clicking on multiple usernames
              processing: true,
            });
            prompt.onUserResponse(index);
          };
        }

        setWebauthnLogin(newLogin);
      },
    });
  };

  const onLoginWithSso = (provider: types.AuthProvider) => {
    promptSsoStatus(true);
    login({
      kind: 'sso',
      clusterUri,
      providerName: provider.name,
      providerType: provider.type,
    });
  };

  const onAbort = () => {
    refAbortCtrl.current?.abort();
  };

  const onCloseDialog = () => {
    onAbort();
    props.onCancel();
  };

  // Since the login form can have two views (primary and secondary)
  // we need to clear any rendered error dialogs before switching.
  const clearLoginAttempt = () => {
    setAttempt({ status: '', statusText: '', data: null });
  };

  useEffect(() => {
    init();
  }, []);

  useEffect(() => {
    if (loginAttempt.status !== 'processing') {
      setWebauthnLogin(null);
      promptSsoStatus(false);
    }

    if (loginAttempt.status === 'success') {
      onSuccess?.();
    }
  }, [loginAttempt.status]);

  return {
    shouldPromptSsoStatus,
    webauthnLogin,
    title: cluster?.name,
    loggedInUserName,
    onLoginWithLocal,
    onLoginWithPasswordless,
    onLoginWithSso,
    onCloseDialog,
    onAbort,
    loginAttempt,
    initAttempt,
    clearLoginAttempt,
  };
}

export type State = ReturnType<typeof useClusterLogin>;

export type Props = {
  clusterUri: RootClusterUri;
  onCancel(): void;
  onSuccess?(): void;
  prefill: { username: string };
};

export type WebauthnLogin = {
  prompt: types.WebauthnLoginPrompt['type'];
  // The below fields are only ever used for passwordless login flow.
  processing?: boolean;
  loginUsernames?: string[];
  onUserResponse?(val: number | string): void;
};
