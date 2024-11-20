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

import { OutlineDanger } from 'design/Alert/Alert';
import { ButtonSecondary, ButtonWarning } from 'design/Button';
import Dialog from 'design/Dialog';
import Flex from 'design/Flex';
import { StepComponentProps, StepSlider } from 'design/StepSlider';
import React, { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import Box from 'design/Box';

import { StepHeader } from 'design/StepSlider';

import useTeleport from 'teleport/useTeleport';

import { MfaDevice } from 'teleport/services/mfa';

import {
  ReauthenticateStep,
  ReauthenticateStepProps,
} from './ReauthenticateStep';
import { MfaChallengeResponse } from 'teleport/services/auth';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import { useAsync } from 'shared/hooks/useAsync';

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
  const [mfaResponse, setMfaResponse] = useState<MfaChallengeResponse>(null);

  // Get an MFA challenge for an existing device.
  const [challenge, getChallenge] = useAsync(async () => {
    return auth.getChallenge({ scope: MfaChallengeScope.MANAGE_DEVICES });
  });

  useEffect(() => {
    getChallenge();
  }, []);
  
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
        deviceToDelete={deviceToDelete}
        mfaChallenge={challenge.data}
        existingMfaResponse={mfaResponse}
        onMfaResponse={setMfaResponse}
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
  existingMfaResponse: MfaChallengeResponse;
  onClose(): void;
  onSuccess(): void;
};

export function DeleteDeviceStep({
  refCallback,
  stepIndex,
  flowLength,
  deviceToDelete,
  existingMfaResponse,
  onClose,
  onSuccess,
}: DeleteAuthDeviceWizardStepProps) {
  const ctx = useTeleport();
  const { run, attempt } = useAttempt();
  const onDelete = () => {
    run(async () => {
      await ctx.mfaService.removeDevice(deviceToDelete.name, existingMfaResponse);
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
