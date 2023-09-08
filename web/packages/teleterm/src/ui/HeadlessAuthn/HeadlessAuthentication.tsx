/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useRef, useEffect } from 'react';

import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { RootClusterUri } from 'teleterm/ui/uri';

import { HeadlessAuthenticationState } from 'teleterm/services/tshd/types';

import { HeadlessPrompt } from './HeadlessPrompt';

interface HeadlessAuthenticationProps {
  rootClusterUri: RootClusterUri;
  headlessAuthenticationId: string;
  clientIp: string;
  skipConfirm: boolean;
  onCancel(): void;
  onSuccess(): void;
}

export function HeadlessAuthentication(props: HeadlessAuthenticationProps) {
  const { headlessAuthenticationService, clustersService } = useAppContext();
  const refAbortCtrl = useRef(clustersService.client.createAbortController());
  const cluster = clustersService.findCluster(props.rootClusterUri);

  const [updateHeadlessStateAttempt, updateHeadlessState] = useAsync(
    (state: HeadlessAuthenticationState) =>
      headlessAuthenticationService.updateHeadlessAuthenticationState(
        {
          rootClusterUri: props.rootClusterUri,
          headlessAuthenticationId: props.headlessAuthenticationId,
          state: state,
        },
        refAbortCtrl.current.signal
      )
  );

  async function handleHeadlessApprove(): Promise<void> {
    const [, error] = await updateHeadlessState(
      HeadlessAuthenticationState.HEADLESS_AUTHENTICATION_STATE_APPROVED
    );
    if (!error) {
      props.onSuccess();
    }
  }

  async function handleHeadlessReject(): Promise<void> {
    const [, error] = await updateHeadlessState(
      HeadlessAuthenticationState.HEADLESS_AUTHENTICATION_STATE_DENIED
    );
    if (!error) {
      props.onSuccess();
    }
  }

  useEffect(() => {
    if (props.skipConfirm && updateHeadlessStateAttempt.status === '') {
      handleHeadlessApprove();
    }
  }, []);

  return (
    <HeadlessPrompt
      cluster={cluster}
      clientIp={props.clientIp}
      skipConfirm={props.skipConfirm}
      onApprove={handleHeadlessApprove}
      abortApproval={refAbortCtrl.current.abort}
      onReject={handleHeadlessReject}
      headlessAuthenticationId={props.headlessAuthenticationId}
      updateHeadlessStateAttempt={updateHeadlessStateAttempt}
      onCancel={props.onCancel}
    />
  );
}
