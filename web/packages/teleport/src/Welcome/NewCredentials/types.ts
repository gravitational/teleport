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

import { ReactElement } from 'react';

import { NewFlow, StepComponentProps } from 'design/StepSlider';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { Auth2faType, PrimaryAuthType } from 'shared/services';

import { RecoveryCodesProps } from 'teleport/components/RecoveryCodes';
import { RecoveryCodes, ResetToken } from 'teleport/services/auth';
import { DeviceUsage } from 'teleport/services/mfa';

export type UseTokenState = {
  auth2faType: Auth2faType;
  primaryAuthType: PrimaryAuthType;
  isPasswordlessEnabled: boolean;
  fetchAttempt: Attempt;
  submitAttempt: Attempt;
  credential?: Credential;
  clearSubmitAttempt: () => void;
  onSubmit: (password: string, otpCode?: string, deviceName?: string) => void;
  createNewWebAuthnDevice: (usage: DeviceUsage) => void;
  onSubmitWithWebauthn: (password?: string, deviceName?: string) => void;
  resetToken: ResetToken;
  recoveryCodes: RecoveryCodes;
  redirect: () => void;
  success: boolean;
  finishedRegister: () => void;
};

// Note: QuestionnaireProps is duplicated in Enterprise (e-teleport/Welcome/Questionnaire/Questionnaire)
export type QuestionnaireProps = {
  onboard: boolean;
  username?: string;
  onSubmit?: () => void;
};

// Note: InviteCollaboratorsCardProps is duplicated in Enterprise
// (e-teleport/Welcome/InviteCollaborators/InviteCollaborators)
export type InviteCollaboratorsCardProps = {
  onSubmit: () => void;
};

export type NewCredentialsProps = UseTokenState & {
  resetMode?: boolean;
  isDashboard: boolean;

  // support E questionnaire
  displayOnboardingQuestionnaire?: boolean;
  setDisplayOnboardingQuestionnaire?: (bool: boolean) => void;
  Questionnaire?: ({
    onboard,
    username,
    onSubmit,
  }: QuestionnaireProps) => ReactElement;

  // support for E's invite collaborators at onboarding
  displayInviteCollaborators?: boolean;
  setDisplayInviteCollaborators?: (bool: boolean) => void;
  InviteCollaborators?: ({
    onSubmit,
  }: InviteCollaboratorsCardProps) => ReactElement;

  RecoveryCodes?: React.ComponentType<RecoveryCodesProps>;
};

export type RegisterSuccessProps = {
  redirect(): void;
  resetMode: boolean;
  username?: string;
  isDashboard: boolean;
};

export type LoginFlow = Extract<PrimaryAuthType, 'passwordless' | 'local'>;
export type SliderProps = StepComponentProps & {
  changeFlow(f: NewFlow<LoginFlow>): void;
};

export type NewCredentialsContainerProps = {
  tokenId?: string;
  resetMode?: boolean;
};
