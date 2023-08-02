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

import React, { useState } from 'react';
import * as Alerts from 'design/Alert';
import { ButtonIcon, Text, ButtonSecondary } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogHeader,
  DialogFooter,
} from 'design/DialogConfirmation';
import { Attempt } from 'shared/hooks/useAsync';
import * as Icons from 'design/Icon';

import { PromptWebauthn } from '../../ClusterConnect/ClusterLogin/FormLogin/PromptWebauthn';

import type * as tsh from 'teleterm/services/tshd/types';

export type HeadlessPromptProps = {
  cluster: tsh.Cluster;
  clientIp: string;
  onApprove(): Promise<void>;
  onReject(): Promise<void>;
  headlessAuthenticationId: string;
  updateHeadlessStateAttempt: Attempt<void>;
  onCancel(): void;
};

export function HeadlessPrompt({
  cluster,
  clientIp,
  onApprove,
  onReject,
  headlessAuthenticationId,
  updateHeadlessStateAttempt,
  onCancel,
}: HeadlessPromptProps) {
  const [waitForMfa, setWaitForMfa] = useState(false);

  return (
    <DialogConfirmation
      dialogCss={() => ({
        maxWidth: '480px',
        width: '100%',
      })}
      disableEscapeKeyDown={false}
      open={true}
    >
      <DialogHeader justifyContent="space-between" mb={0} alignItems="baseline">
        <Text typography="h4">
          Headless command on <b>{cluster.name}</b>
        </Text>
        <ButtonIcon type="button" onClick={onCancel} color="text.slightlyMuted">
          <Icons.Close fontSize={5} />
        </ButtonIcon>
      </DialogHeader>
      {updateHeadlessStateAttempt.status === 'error' && (
        <Alerts.Danger mb={0}>
          {updateHeadlessStateAttempt.statusText}
        </Alerts.Danger>
      )}

      {!waitForMfa && (
        <>
          <DialogContent>
            <Text color="text.slightlyMuted">
              Someone initiated a headless command from <b>{clientIp}</b>.
              <br />
              If it was not you, click Reject and contact your administrator.
            </Text>
            <Text color="text.muted" mt={1} fontSize="12px">
              Request ID: {headlessAuthenticationId}
            </Text>
          </DialogContent>
          <DialogFooter>
            <ButtonSecondary
              autoFocus
              mr={3}
              type="submit"
              onClick={e => {
                e.preventDefault();
                setWaitForMfa(true);
                onApprove();
              }}
            >
              Approve
            </ButtonSecondary>
            <ButtonSecondary
              type="button"
              onClick={e => {
                e.preventDefault();
                onReject();
              }}
            >
              Reject
            </ButtonSecondary>
          </DialogFooter>
        </>
      )}
      {waitForMfa && (
        <DialogContent mb={2}>
          <Text color="text.slightlyMuted">
            Complete MFA verification to approve the Headless Login.
          </Text>
          <PromptWebauthn prompt={'tap'} onCancel={onCancel} />
        </DialogContent>
      )}
    </DialogConfirmation>
  );
}
