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

import { useEffect, useRef, useState } from 'react';

import { useAsync } from 'shared/hooks/useAsync';

import { cloneAbortSignal } from 'teleterm/services/tshd/cloneableClient';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import type * as types from 'teleterm/ui/services/clusters/types';
import { RootClusterUri } from 'teleterm/ui/uri';
import { assertUnreachable } from 'teleterm/ui/utils';

export default function useClusterLogin(props: Props) {
  const { onSuccess, clusterUri } = props;
  const { clustersService, tshd } = useAppContext();
  const cluster = clustersService.findCluster(clusterUri);
  const refAbortCtrl = useRef<AbortController>(null);
  const loggedInUserName =
    props.prefill.username || cluster.loggedInUser?.name || null;
  const [shouldPromptSsoStatus, promptSsoStatus] = useState(false);
  const [passwordlessLoginState, setPasswordlessLoginState] =
    useState<PasswordlessLoginState>();

  const [initAttempt, init] = useAsync(() =>
    tshd.getAuthSettings({ clusterUri }).then(({ response }) => response)
  );

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

  const onLoginWithLocal = (username: string, password: string) => {
    login({
      kind: 'local',
      clusterUri,
      username,
      password,
    });
  };

  const onLoginWithPasswordless = () => {
    login({
      kind: 'passwordless',
      clusterUri,
      onPromptCallback: (prompt: types.PasswordlessLoginPrompt) => {
        const newState: PasswordlessLoginState = {
          prompt: prompt.type,
          processing: false,
        };

        if (prompt.type === 'pin') {
          newState.onUserResponse = (pin: string) => {
            setPasswordlessLoginState({
              ...newState,
              // prevent user from clicking on submit buttons more than once
              processing: true,
            });
            prompt.onUserResponse(pin);
          };
        }

        if (prompt.type === 'credential') {
          newState.loginUsernames = prompt.data.credentials.map(
            c => c.username
          );
          newState.onUserResponse = (index: number) => {
            setPasswordlessLoginState({
              ...newState,
              // prevent user from clicking on multiple usernames
              processing: true,
            });
            prompt.onUserResponse(index);
          };
        }

        setPasswordlessLoginState(newState);
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
      setPasswordlessLoginState(null);
      promptSsoStatus(false);
    }

    if (loginAttempt.status === 'success') {
      onSuccess?.();
    }
  }, [loginAttempt.status]);

  return {
    shouldPromptSsoStatus,
    passwordlessLoginState,
    title: cluster?.name,
    loggedInUserName,
    onLoginWithLocal,
    onLoginWithPasswordless,
    onLoginWithSso,
    onCloseDialog,
    onAbort,
    loginAttempt,
    initAttempt,
    init,
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

export type PasswordlessLoginState = {
  /**
   * prompt describes the current step, or prompt, shown to the user during the passwordless login.
   */
  prompt: types.PasswordlessLoginPrompt['type'];
  processing?: boolean;
  loginUsernames?: string[];
  onUserResponse?(val: number | string): void;
};
