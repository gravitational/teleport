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

import React, { useState } from 'react';
import * as Alerts from 'design/Alert';
import {
  ButtonIcon,
  Text,
  ButtonSecondary,
  Image,
  Flex,
  Box,
  H2,
} from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogHeader,
  DialogFooter,
} from 'design/DialogConfirmation';
import { Attempt } from 'shared/hooks/useAsync';
import * as Icons from 'design/Icon';

import { P, P3 } from 'design/Text/Text';

import LinearProgress from 'teleterm/ui/components/LinearProgress';
import svgHardwareKey from 'teleterm/ui/ClusterConnect/ClusterLogin/FormLogin/PromptWebauthn/hardware.svg';

import type * as tsh from 'teleterm/services/tshd/types';

export type HeadlessPromptProps = {
  cluster: tsh.Cluster;
  clientIp: string;
  skipConfirm: boolean;
  onApprove(): Promise<void>;
  abortApproval(): void;
  /**
   * onReject updates the state of the request by rejecting it.
   */
  onReject(): Promise<void>;
  headlessAuthenticationId: string;
  updateHeadlessStateAttempt: Attempt<void>;
  /**
   * onCancel simply closes the modal and ignores the request. The user is still able to confirm or
   * reject the request from the Web UI.
   */
  onCancel(): void;
};

export function HeadlessPrompt({
  cluster,
  clientIp,
  skipConfirm,
  onApprove,
  abortApproval,
  onReject,
  headlessAuthenticationId,
  updateHeadlessStateAttempt,
  onCancel,
}: HeadlessPromptProps) {
  // skipConfirm automatically attempts to approve a headless auth attempt,
  // so let's show waitForMfa from the very beginning in that case.
  const [waitForMfa, setWaitForMfa] = useState(skipConfirm);

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
        <H2 mb={4}>
          Headless command on <b>{cluster.name}</b>
        </H2>
        <ButtonIcon
          type="button"
          color="text.slightlyMuted"
          onClick={() => {
            abortApproval();
            onCancel();
          }}
        >
          <Icons.Cross size="medium" />
        </ButtonIcon>
      </DialogHeader>
      {updateHeadlessStateAttempt.status === 'error' && (
        <Alerts.Danger mb={0}>
          {updateHeadlessStateAttempt.statusText}
        </Alerts.Danger>
      )}
      <DialogContent>
        <P color="text.slightlyMuted">
          Someone initiated a headless command from <b>{clientIp}</b>.
        </P>
        <P>If it was not you, click Reject and contact your administrator.</P>
        <P3 color="text.muted">Request ID: {headlessAuthenticationId}</P3>
      </DialogContent>
      {waitForMfa && (
        <DialogContent mb={2}>
          <Text color="text.slightlyMuted">
            Complete MFA verification to approve the Headless Login.
          </Text>

          <Image mt={4} mb={4} width="200px" src={svgHardwareKey} mx="auto" />
          <Box textAlign="center" style={{ position: 'relative' }}>
            <Text bold>Insert your security key and tap it</Text>
            <LinearProgress />
          </Box>

          <Flex justifyContent="flex-end" mt={4} gap={3}>
            {/*
              The Reject button is there so that if skipping confirmation is enabled (see
              HeadlessAuthenticationService) then the user still has the ability to reject the
              request from the screen that prompts for key touch.
            */}
            <ButtonSecondary
              type="button"
              onClick={() => {
                abortApproval();
                onReject();
              }}
            >
              Reject
            </ButtonSecondary>
            <ButtonSecondary
              type="button"
              onClick={() => {
                abortApproval();
                onCancel();
              }}
            >
              Cancel
            </ButtonSecondary>
          </Flex>
        </DialogContent>
      )}
      {!waitForMfa && (
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
      )}
    </DialogConfirmation>
  );
}
