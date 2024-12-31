/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useState } from 'react';

import { Alert, OutlineDanger } from 'design/Alert/Alert';
import Box from 'design/Box';
import { ButtonSecondary, ButtonWarning } from 'design/Button';
import Dialog from 'design/Dialog';
import Flex from 'design/Flex';
import Indicator from 'design/Indicator';
import { StepComponentProps, StepHeader, StepSlider } from 'design/StepSlider';
import useAttempt from 'shared/hooks/useAttemptNext';

import useReAuthenticate from 'teleport/components/ReAuthenticate/useReAuthenticate';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import { MfaDevice } from 'teleport/services/mfa';
import useTeleport from 'teleport/useTeleport';

import {
  ReauthenticateStep,
  ReauthenticateStepProps,
} from './ReauthenticateStep';

interface DeleteAuthDeviceWizardProps {
  /** Device to be removed. */
  deviceToDelete: MfaDevice;
  onClose(): void;
  onSuccess(): void;
}

/** A wizard for deleting MFA and passkey devices. */
export function DeleteAuthDeviceWizard({
  deviceToDelete,
  onClose,
  onSuccess,
}: DeleteAuthDeviceWizardProps) {
  const [privilegeToken, setPrivilegeToken] = useState('');

  const reauthState = useReAuthenticate({
    challengeScope: MfaChallengeScope.MANAGE_DEVICES,
    onMfaResponse: mfaResponse =>
      // TODO(Joerger): v19.0.0
      // Devices can be deleted with an MFA response, so exchanging it for a
      // privilege token adds an unnecessary API call. The device deletion
      // endpoint requires a token, but the new endpoint "DELETE: /webapi/mfa/devices"
      // does not and can be used in v19 backwards compatibly.
      auth.createPrivilegeToken(mfaResponse).then(setPrivilegeToken),
  });

  // Handle potential error states first.
  switch (reauthState.initAttempt.status) {
    case 'processing':
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    case 'error':
      return <Alert children={reauthState.initAttempt.statusText} />;
    case 'success':
      break;
    default:
      return null;
  }

  return (
    <Dialog
      open={true}
      disableEscapeKeyDown={false}
      dialogCss={() => ({ width: '650px' })}
      onClose={onClose}
    >
      <StepSlider
        flows={wizardFlows}
        currFlow="default"
        // Step properties
        reauthState={reauthState}
        deviceToDelete={deviceToDelete}
        privilegeToken={privilegeToken}
        onClose={onClose}
        onSuccess={onSuccess}
      />
    </Dialog>
  );
}

const wizardFlows = {
  default: [ReauthenticateForDeleteStep, DeleteDeviceStep],
};

function ReauthenticateForDeleteStep(props: DeleteAuthDeviceWizardStepProps) {
  return <ReauthenticateStep {...props} />;
}

export type DeleteAuthDeviceWizardStepProps = StepComponentProps &
  ReauthenticateStepProps &
  DeleteDeviceStepProps;

type DeleteDeviceStepProps = StepComponentProps & {
  deviceToDelete: MfaDevice;
  privilegeToken: string;
  onClose(): void;
  onSuccess(): void;
};

export function DeleteDeviceStep({
  refCallback,
  stepIndex,
  flowLength,
  deviceToDelete,
  privilegeToken,
  onClose,
  onSuccess,
}: DeleteAuthDeviceWizardStepProps) {
  const ctx = useTeleport();
  const { run, attempt } = useAttempt();
  const onDelete = () => {
    run(async () => {
      await ctx.mfaService.removeDevice(privilegeToken, deviceToDelete.name);
      onSuccess();
    });
  };

  const message =
    deviceToDelete.usage === 'passwordless'
      ? `Are you sure you want to delete your "${deviceToDelete.name}" passkey?`
      : `Are you sure you want to delete your "${deviceToDelete.name}" MFA method?`;
  const title =
    deviceToDelete.usage === 'passwordless'
      ? 'Delete Passkey'
      : 'Delete MFA Method';

  return (
    <div ref={refCallback} data-testid="delete-step">
      <Box mb={4}>
        <StepHeader
          stepIndex={stepIndex}
          flowLength={flowLength}
          title={title}
        />
      </Box>
      {attempt.status === 'failed' && (
        <OutlineDanger>{attempt.statusText}</OutlineDanger>
      )}
      <Box mb={4}>{message}</Box>
      <Flex gap={2}>
        <ButtonWarning block={true} size="large" onClick={onDelete}>
          Delete
        </ButtonWarning>
        <ButtonSecondary block={true} size="large" onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </Flex>
    </div>
  );
}
