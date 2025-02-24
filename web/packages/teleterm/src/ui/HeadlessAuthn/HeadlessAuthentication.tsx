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

import { useEffect, useRef } from 'react';

import { HeadlessAuthenticationState } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import { useAsync } from 'shared/hooks/useAsync';

import { cloneAbortSignal } from 'teleterm/services/tshd/cloneableClient';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { RootClusterUri } from 'teleterm/ui/uri';

import { HeadlessPrompt } from './HeadlessPrompt';

interface HeadlessAuthenticationProps {
  rootClusterUri: RootClusterUri;
  headlessAuthenticationId: string;
  clientIp: string;
  skipConfirm: boolean;
  onCancel(): void;
  onSuccess(): void;
  hidden?: boolean;
}

export function HeadlessAuthentication(props: HeadlessAuthenticationProps) {
  const { headlessAuthenticationService, clustersService } = useAppContext();
  const refAbortCtrl = useRef(new AbortController());
  const cluster = clustersService.findCluster(props.rootClusterUri);

  const [updateHeadlessStateAttempt, updateHeadlessState] = useAsync(
    (state: HeadlessAuthenticationState) =>
      headlessAuthenticationService.updateHeadlessAuthenticationState(
        {
          rootClusterUri: props.rootClusterUri,
          headlessAuthenticationId: props.headlessAuthenticationId,
          state: state,
        },
        cloneAbortSignal(refAbortCtrl.current.signal)
      )
  );

  async function handleHeadlessApprove(): Promise<void> {
    const [, error] = await updateHeadlessState(
      HeadlessAuthenticationState.APPROVED
    );
    if (!error) {
      props.onSuccess();
    }
  }

  async function handleHeadlessReject(): Promise<void> {
    const [, error] = await updateHeadlessState(
      HeadlessAuthenticationState.DENIED
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
      hidden={props.hidden}
      cluster={cluster}
      clientIp={props.clientIp}
      skipConfirm={props.skipConfirm}
      onApprove={handleHeadlessApprove}
      abortApproval={() => refAbortCtrl.current.abort()}
      onReject={handleHeadlessReject}
      headlessAuthenticationId={props.headlessAuthenticationId}
      updateHeadlessStateAttempt={updateHeadlessStateAttempt}
      onCancel={props.onCancel}
    />
  );
}
