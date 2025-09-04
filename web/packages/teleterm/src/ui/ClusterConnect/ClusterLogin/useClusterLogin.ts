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

import { AuthProvider } from 'gen-proto-ts/teleport/lib/teleterm/v1/auth_settings_pb';
import {
  CredentialInfo,
  PasswordlessPrompt,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import { useAsync } from 'shared/hooks/useAsync';

import {
  CloneableAbortSignal,
  cloneAbortSignal,
} from 'teleterm/services/tshd/cloneableClient';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useAppUpdaterContext } from 'teleterm/ui/AppUpdater';
import { IAppContext } from 'teleterm/ui/types';
import * as uri from 'teleterm/ui/uri';
import { RootClusterUri } from 'teleterm/ui/uri';

export type SsoPrompt =
  /**
   * No prompt, SSO login not in use.
   */
  | 'no-prompt'
  /**
   * The user is asked to follow the steps in the browser (the browser automatically opens).
   */
  | 'follow-browser-steps'
  /**
   * After the user followed the steps in the browser and the login RPC returned but before the
   * cluster sync RPC started.
   */
  | 'wait-for-sync';

export function useClusterLogin(props: Props) {
  const { onSuccess, clusterUri } = props;
  const ctx = useAppContext();
  const { clustersService, tshd, configService, mainProcessClient } = ctx;
  const appUpdaterContext = useAppUpdaterContext();
  const cluster = clustersService.findCluster(clusterUri);
  const refAbortCtrl = useRef<AbortController>(null);
  const loggedInUserName =
    props.prefill.username || cluster.loggedInUser?.name || null;
  const [ssoPrompt, setSsoPrompt] = useState<SsoPrompt>('no-prompt');
  const [passwordlessLoginState, setPasswordlessLoginState] =
    useState<PasswordlessLoginState>();

  const [initAttempt, init] = useAsync(() => {
    return Promise.all([
      tshd.getAuthSettings({ clusterUri }).then(({ response }) => response),
      // checkForAppUpdates doesn't return a rejected promise, errors are
      // surfaced in app updates widget and details view.
      mainProcessClient.checkForAppUpdates(),
    ]).then(([authSettings]) => authSettings);
  });

  const [loginAttempt, login, setAttempt] = useAsync((params: LoginParams) => {
    refAbortCtrl.current = new AbortController();
    const signal = cloneAbortSignal(refAbortCtrl.current.signal);
    switch (params.kind) {
      case 'local':
        return loginLocal(ctx, params, signal);
      case 'passwordless':
        return loginPasswordless(ctx, params, signal);
      case 'sso':
        return loginSso(ctx, params, setSsoPrompt, signal);
      default:
        params satisfies never;
    }
  });

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
      onPromptCallback: (prompt: PasswordlessLoginPrompt) => {
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

  const onLoginWithSso = (provider: AuthProvider) => {
    setSsoPrompt('follow-browser-steps');
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
      setSsoPrompt('no-prompt');
    }

    if (loginAttempt.status === 'success') {
      onSuccess?.();
    }
  }, [loginAttempt.status]);

  //TODO(gzdunek): We should have a way to listen to config service changes.
  //A workaround for is to update the state, which triggers a re-render.
  const [shouldSkipVersionCheck, setShouldSkipVersionCheck] = useState(
    () => configService.get('skipVersionCheck').value
  );
  function disableVersionCheck() {
    configService.set('skipVersionCheck', true);
    setShouldSkipVersionCheck(true);
  }
  const { platform } = mainProcessClient.getRuntimeSettings();

  return {
    ssoPrompt,
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
    shouldSkipVersionCheck,
    disableVersionCheck,
    platform,
    appUpdateEvent: appUpdaterContext.updateEvent,
    downloadAppUpdate: mainProcessClient.downloadAppUpdate,
    cancelAppUpdateDownload: mainProcessClient.cancelAppUpdateDownload,
    checkForAppUpdates: mainProcessClient.checkForAppUpdates,
    quitAndInstallAppUpdate: mainProcessClient.quitAndInstallAppUpdate,
    changeAppUpdatesManagingCluster:
      mainProcessClient.changeAppUpdatesManagingCluster,
    clusterGetter: {
      findCluster: (clusterUri: RootClusterUri) =>
        clustersService.findCluster(clusterUri),
    },
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
  prompt: PasswordlessLoginPrompt['type'];
  processing?: boolean;
  loginUsernames?: string[];
  onUserResponse?(val: number | string): void;
};

/*
 * Login functions
 */

export type LoginParams =
  | LoginLocalParams
  | LoginPasswordlessParams
  | LoginSsoParams;

export interface LoginLocalParams {
  kind: 'local';
  clusterUri: uri.RootClusterUri;
  username: string;
  password: string;
  token?: string;
}

async function loginLocal(
  { tshd, clustersService, usageService }: IAppContext,
  params: LoginLocalParams,
  abortSignal: CloneableAbortSignal
) {
  await tshd.login(
    {
      clusterUri: params.clusterUri,
      params: {
        oneofKind: 'local',
        local: {
          user: params.username,
          password: params.password,
          token: params.token,
        },
      },
    },
    { abort: abortSignal }
  );
  // We explicitly use the `andCatchErrors` variant here. If loginLocal succeeds but syncing the
  // cluster fails, we don't want to stop the user on the failed modal – we want to open the
  // workspace and show an error state within the workspace.
  await clustersService.syncAndWatchRootClusterWithErrorHandling(
    params.clusterUri
  );
  usageService.captureUserLogin(params.clusterUri, 'local');
}

export interface LoginSsoParams {
  kind: 'sso';
  clusterUri: uri.RootClusterUri;
  providerType: string;
  providerName: string;
}

async function loginSso(
  { tshd, clustersService, usageService, mainProcessClient }: IAppContext,
  params: LoginSsoParams,
  setSsoPrompt: (prompt: SsoPrompt) => void,
  abortSignal: CloneableAbortSignal
) {
  await tshd.login(
    {
      clusterUri: params.clusterUri,
      params: {
        oneofKind: 'sso',
        sso: {
          providerType: params.providerType,
          providerName: params.providerName,
        },
      },
    },
    { abort: abortSignal }
  );
  setSsoPrompt('wait-for-sync');

  // Force once login finishes but before we await the cluster sync. This way the focus will go back
  // to the app ASAP. The login modal will be shown until the cluster sync finishes.
  void mainProcessClient.forceFocusWindow();

  await clustersService.syncAndWatchRootClusterWithErrorHandling(
    params.clusterUri
  );
  usageService.captureUserLogin(params.clusterUri, params.providerType);
}

export interface LoginPasswordlessParams {
  kind: 'passwordless';
  clusterUri: uri.RootClusterUri;
  onPromptCallback(res: PasswordlessLoginPrompt): void;
}

export type PasswordlessLoginPrompt =
  | { type: 'tap' }
  | { type: 'retap' }
  | { type: 'pin'; onUserResponse(pin: string): void }
  | {
      type: 'credential';
      data: { credentials: CredentialInfo[] };
      onUserResponse(index: number): void;
    };

async function loginPasswordless(
  { tshd, clustersService, usageService }: IAppContext,
  params: LoginPasswordlessParams,
  abortSignal: CloneableAbortSignal
) {
  await new Promise<void>((resolve, reject) => {
    const stream = tshd.loginPasswordless({
      abort: abortSignal,
    });

    let hasDeviceBeenTapped = false;

    // Init the stream.
    stream.requests.send({
      request: {
        oneofKind: 'init',
        init: {
          clusterUri: params.clusterUri,
        },
      },
    });

    stream.responses.onMessage(function (response) {
      switch (response.prompt) {
        case PasswordlessPrompt.PIN:
          const pinResponse = (pin: string) => {
            stream.requests.send({
              request: {
                oneofKind: 'pin',
                pin: { pin },
              },
            });
          };

          params.onPromptCallback({
            type: 'pin',
            onUserResponse: pinResponse,
          });
          return;

        case PasswordlessPrompt.CREDENTIAL:
          const credResponse = (index: number) => {
            stream.requests.send({
              request: {
                oneofKind: 'credential',
                credential: { index: BigInt(index) },
              },
            });
          };

          params.onPromptCallback({
            type: 'credential',
            onUserResponse: credResponse,
            data: { credentials: response.credentials || [] },
          });
          return;

        case PasswordlessPrompt.TAP:
          if (hasDeviceBeenTapped) {
            params.onPromptCallback({ type: 'retap' });
          } else {
            hasDeviceBeenTapped = true;
            params.onPromptCallback({ type: 'tap' });
          }
          return;

        // Following cases should never happen but just in case?
        case PasswordlessPrompt.UNSPECIFIED:
          stream.requests.complete();
          return reject(new Error('no passwordless prompt was specified'));

        default:
          stream.requests.complete();
          return reject(
            new Error(`passwordless prompt '${response.prompt}' not supported`)
          );
      }
    });

    stream.responses.onComplete(function () {
      resolve();
    });

    stream.responses.onError(function (err: Error) {
      reject(err);
    });
  });

  await clustersService.syncAndWatchRootClusterWithErrorHandling(
    params.clusterUri
  );
  usageService.captureUserLogin(params.clusterUri, 'passwordless');
}
