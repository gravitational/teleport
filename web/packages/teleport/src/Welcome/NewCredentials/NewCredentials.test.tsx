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

import { Attempt } from 'shared/hooks/useAttemptNext';

import { render, screen } from 'design/utils/testing';
import React from 'react';

import { RecoveryCodes, ResetToken } from 'teleport/services/auth';
import { NewCredentialsProps } from 'teleport/Welcome/NewCredentials/types';
import { NewCredentials } from 'teleport/Welcome/NewCredentials/NewCredentials';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';

const attempt: Attempt = { status: '' };
const failedAttempt: Attempt = { status: 'failed' };
const processingAttempt: Attempt = { status: 'processing' };

const resetToken: ResetToken = {
  tokenId: 'tokenId',
  qrCode: 'qrCode',
  user: 'user',
};
const recoveryCodes: RecoveryCodes = {
  createdDate: new Date(),
};

const makeProps = (): NewCredentialsProps => {
  return {
    auth2faType: 'off',
    primaryAuthType: 'sso',
    isPasswordlessEnabled: false,
    fetchAttempt: attempt,
    submitAttempt: attempt,
    clearSubmitAttempt: () => {},
    onSubmit: () => {},
    createNewWebAuthnDevice: () => {},
    onSubmitWithWebauthn: () => {},
    resetToken: resetToken,
    recoveryCodes: recoveryCodes,
    redirect: () => {},
    success: false,
    finishedRegister: () => {},
    resetMode: false,
    isDashboard: false,
  };
};

test('renders expired for failed fetch attempt', () => {
  const props = makeProps();
  props.fetchAttempt = failedAttempt;
  render(<NewCredentials {...props} />);

  expect(screen.getByText(/Invitation Code Expired/i)).toBeInTheDocument();
});

const nullCases: {
  attempt: Attempt;
}[] = [{ attempt: processingAttempt }, { attempt: attempt }];

test.each(nullCases)('renders $attempt as null', testCase => {
  const props = makeProps();
  props.fetchAttempt = testCase.attempt;
  const { container } = render(<NewCredentials {...props} />);

  expect(container).toBeEmptyDOMElement();
});

test('renders Register Success on success', () => {
  const props = makeProps();
  props.fetchAttempt = { status: 'success' };
  props.recoveryCodes = undefined;
  props.success = true;
  render(<NewCredentials {...props} />);

  expect(
    screen.getByText(/Proceed to access your account./i)
  ).toBeInTheDocument();
  expect(screen.getByText(/Go to Cluster/i)).toBeInTheDocument();
});

test('renders recovery codes', () => {
  const props = makeProps();
  props.fetchAttempt = { status: 'success' };
  props.success = false;
  props.recoveryCodes = {
    codes: ['foo', 'bar'],
    createdDate: new Date(),
  };
  render(<NewCredentials {...props} />);

  expect(screen.getByText(/Backup & Recovery Codes/i)).toBeInTheDocument();
});

test('renders credential flow for passwordless', () => {
  const props = makeProps();
  props.fetchAttempt = { status: 'success' };
  props.success = false;
  props.recoveryCodes = undefined;
  props.primaryAuthType = 'passwordless';
  render(<NewCredentials {...props} />);

  expect(
    screen.getByText(/Set up Passwordless Authentication/i)
  ).toBeInTheDocument();
});

test('renders credential flow for local', () => {
  const props = makeProps();
  props.fetchAttempt = { status: 'success' };
  props.success = false;
  props.recoveryCodes = undefined;
  props.primaryAuthType = 'local';
  render(<NewCredentials {...props} />);

  expect(screen.getByText(/Set A Password/i)).toBeInTheDocument();
});

test('renders credential flow for sso', () => {
  const props = makeProps();
  props.fetchAttempt = { status: 'success' };
  props.success = false;
  props.recoveryCodes = undefined;
  props.primaryAuthType = 'sso';
  render(<NewCredentials {...props} />);

  expect(screen.getByText(/Set A Password/i)).toBeInTheDocument();
});

test('renders questionnaire', () => {
  mockUserContextProviderWith(makeTestUserContext());

  const props = makeProps();
  props.fetchAttempt = { status: 'success' };
  props.success = true;
  props.recoveryCodes = undefined;
  props.displayOnboardingQuestionnaire = true;
  props.setDisplayOnboardingQuestionnaire = () => {};
  props.Questionnaire = () => <div>Passed Component!</div>;
  render(<NewCredentials {...props} />);

  expect(screen.getByText(/Passed Component!/i)).toBeInTheDocument();
});

test('renders invite collaborators', () => {
  mockUserContextProviderWith(makeTestUserContext());

  const props = makeProps();
  props.fetchAttempt = { status: 'success' };
  props.success = true;
  props.recoveryCodes = undefined;
  props.displayInviteCollaborators = true;
  props.setDisplayInviteCollaborators = () => {};
  props.InviteCollaborators = () => <div>Passed Component!</div>;
  render(<NewCredentials {...props} />);

  expect(screen.getByText(/Passed Component!/i)).toBeInTheDocument();
});
