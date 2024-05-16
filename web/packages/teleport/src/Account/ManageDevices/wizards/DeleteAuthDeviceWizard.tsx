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
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Dialog from 'design/Dialog';
import Flex from 'design/Flex';
import { StepComponentProps, StepSlider } from 'design/StepSlider';
import React, { useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Auth2faType } from 'shared/services';

import Box from 'design/Box';

import { StepHeader } from 'design/StepSlider';

import useTeleport from 'teleport/useTeleport';

import { MfaDevice } from 'teleport/services/mfa';

import {
  ReauthenticateStep,
  ReauthenticateStepProps,
} from './ReauthenticateStep';

interface DeleteAuthDeviceWizardProps {
  /** MFA type setting, as configured in the cluster's configuration. */
  auth2faType: Auth2faType;
  /** Device to be removed. */
  device: MfaDevice;
  onClose(): void;
  onSuccess(): void;
}

/** A wizard for deleting MFA and passkey devices. */
export function DeleteAuthDeviceWizard({
  auth2faType,
  device,
  onClose,
  onSuccess,
}: DeleteAuthDeviceWizardProps) {
  const [privilegeToken, setPrivilegeToken] = useState('');

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
        device={device}
        auth2faType={auth2faType}
        privilegeToken={privilegeToken}
        onClose={onClose}
        onAuthenticated={setPrivilegeToken}
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
  device: MfaDevice;
  privilegeToken: string;
  onClose(): void;
  onSuccess(): void;
};

export function DeleteDeviceStep({
  refCallback,
  stepIndex,
  flowLength,
  device,
  privilegeToken,
  onClose,
  onSuccess,
}: DeleteAuthDeviceWizardStepProps) {
  const ctx = useTeleport();
  const { run, attempt } = useAttempt();
  const onDelete = () => {
    run(async () => {
      await ctx.mfaService.removeDevice(privilegeToken, device.name);
      onSuccess();
    });
  };

  const message =
    device.usage === 'passwordless'
      ? `Are you sure you want to delete your "${device.name}" passkey?`
      : `Are you sure you want to delete your "${device.name}" MFA method?`;
  const title =
    device.usage === 'passwordless' ? 'Delete Passkey' : 'Delete MFA Method';

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
        <ButtonPrimary block={true} onClick={onDelete}>
          Delete
        </ButtonPrimary>
        <ButtonSecondary block={true} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </Flex>
    </div>
  );
}
