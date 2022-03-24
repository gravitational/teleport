/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useState, useEffect, useRef } from 'react';
import * as types from 'teleterm/ui/services/clusters/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import useAsync from 'teleterm/ui/useAsync';

export default function useClusterLogin(props: Props) {
  const { onClose, onSuccess, clusterUri } = props;
  const { clustersService } = useAppContext();
  const cluster = clustersService.findCluster(clusterUri);
  const refAbortCtrl = useRef<types.tsh.TshAbortController>(null);
  const [shouldPromptSsoStatus, promptSsoStatus] = useState(false);
  const [shouldPromptHardwareKey, promptHardwareKey] = useState(false);

  const [initAttempt, init] = useAsync(() => {
    return clustersService.getAuthSettings(clusterUri);
  });

  const [loginAttempt, login] = useAsync((opts: types.LoginParams) => {
    refAbortCtrl.current = clustersService.client.createAbortController();
    return clustersService.login(opts, refAbortCtrl.current.signal);
  });

  const onLoginWithLocal = (
    username: '',
    password: '',
    token: '',
    authType?: types.Auth2faType
  ) => {
    promptHardwareKey(authType === 'webauthn' || authType === 'u2f');
    login({
      clusterUri,
      local: {
        username,
        password,
        token,
      },
    });
  };

  const onLoginWithSso = (provider: types.AuthProvider) => {
    promptSsoStatus(true);
    login({
      clusterUri,
      sso: {
        providerName: provider.name,
        providerType: provider.type,
      },
    });
  };

  const onAbort = () => {
    refAbortCtrl.current?.abort();
  };

  const onCloseDialog = () => {
    onAbort();
    props?.onClose();
  };

  useEffect(() => {
    init();
  }, []);

  useEffect(() => {
    if (loginAttempt.status !== 'processing') {
      promptHardwareKey(false);
      promptSsoStatus(false);
    }

    if (loginAttempt.status === 'success') {
      onClose();
      onSuccess?.();
    }
  }, [loginAttempt.status]);

  return {
    shouldPromptSsoStatus,
    shouldPromptHardwareKey,
    title: cluster.name,
    onLoginWithLocal,
    onLoginWithSso,
    onCloseDialog,
    onAbort,
    loginAttempt,
    initAttempt,
  };
}

export type State = ReturnType<typeof useClusterLogin>;

export type Props = {
  clusterUri: string;
  onClose(): void;
  onSuccess?(): void;
};
