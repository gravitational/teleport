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

import {
  Box,
  ButtonSecondary,
  CloseButton,
  Code,
  ComposedAlert,
  ComposedDialog,
  Dialog,
  P2,
} from '@gravitational/design-system';
import { useRef, useState } from 'react';

import { Image } from 'design';
import { Attempt } from 'shared/hooks/useAsync';

import svgHardwareKey from 'teleterm/ui/ClusterConnect/ClusterLogin/FormLogin/PromptPasswordless/hardware.svg';
import { LinearProgress } from 'teleterm/ui/components/LinearProgress';
import { RootClusterUri, routing } from 'teleterm/ui/uri';

export type HeadlessPromptProps = {
  rootClusterUri: RootClusterUri;
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
  hidden?: boolean;
};

export function HeadlessPrompt({
  rootClusterUri,
  clientIp,
  skipConfirm,
  onApprove,
  abortApproval,
  onReject,
  headlessAuthenticationId,
  updateHeadlessStateAttempt,
  onCancel,
  hidden,
}: HeadlessPromptProps) {
  // skipConfirm automatically attempts to approve a headless auth attempt,
  // so let's show waitForMfa from the very beginning in that case.
  const [waitForMfa, setWaitForMfa] = useState(skipConfirm);
  const approveButtonRef = useRef<HTMLButtonElement>(null);

  return (
    <ComposedDialog
      open={!hidden}
      unmountOnExit={false}
      onOpenChange={({ open }) => {
        if (!open) {
          abortApproval();
          onCancel();
        }
      }}
      skipAnimationOnMount
      initialFocusEl={() => approveButtonRef.current}
      closeOnInteractOutside={false}
      contentProps={{ maxW: '540px', w: '100%' }}
    >
      <Dialog.Header>
        <Dialog.Title>
          {waitForMfa ? 'Verify your identity' : 'Headless authentication'}
        </Dialog.Title>
        <Dialog.CloseTrigger asChild>
          <CloseButton />
        </Dialog.CloseTrigger>
      </Dialog.Header>
      <Dialog.Body gap={4}>
        {updateHeadlessStateAttempt.status === 'error' && (
          <ComposedAlert
            title="Could not update the request"
            description={updateHeadlessStateAttempt.statusText}
          />
        )}

        <P2 textWrap="pretty">
          Someone initiated a one-time tsh command on{' '}
          <strong>{routing.parseClusterName(rootClusterUri)}</strong> from{' '}
          <strong>{clientIp}</strong>.
        </P2>

        <P2>Confirm the Request ID below matches the one shown by tsh:</P2>

        <Code size="md" variant="outline" px={3} py={2}>
          {headlessAuthenticationId}
        </Code>

        <P2 textWrap="pretty">
          If you didn&#x27;t start this request or the ID doesn&#x27;t match,
          click Reject and contact your administrator.
        </P2>
        {waitForMfa && (
          <>
            <Image width="200px" src={svgHardwareKey} mx="auto" my={2} />
            <Box textAlign="center" style={{ position: 'relative' }}>
              <P2 fontWeight="bold">Insert your security key and tap it.</P2>
              <LinearProgress />
            </Box>
          </>
        )}
      </Dialog.Body>
      <Dialog.Footer>
        {waitForMfa ? (
          <>
            <Dialog.CloseTrigger asChild unstyled>
              <ButtonSecondary>Cancel</ButtonSecondary>
            </Dialog.CloseTrigger>
            {/*
              The Reject button is there so that if skipping confirmation is enabled (see
              HeadlessAuthenticationService) then the user still has the ability to reject the
              request from the screen that prompts for key touch.
            */}
            <ButtonSecondary
              onClick={() => {
                abortApproval();
                onReject();
              }}
            >
              Reject
            </ButtonSecondary>
          </>
        ) : (
          <>
            <ButtonSecondary
              onClick={e => {
                e.preventDefault();
                void onReject();
              }}
            >
              Reject
            </ButtonSecondary>
            <ButtonSecondary
              ref={approveButtonRef}
              onClick={() => {
                setWaitForMfa(true);
                void onApprove();
              }}
            >
              Approve
            </ButtonSecondary>
          </>
        )}
      </Dialog.Footer>
    </ComposedDialog>
  );
}
