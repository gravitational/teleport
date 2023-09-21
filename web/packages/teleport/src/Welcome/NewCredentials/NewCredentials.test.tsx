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
const successAttempt: Attempt = { status: 'success', statusText: 'hey' };

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
    onSubmitWithWebauthn: () => {},
    resetToken: resetToken,
    recoveryCodes: recoveryCodes,
    redirect: () => {},
    success: false,
    finishedRegister: () => {},
    privateKeyPolicyEnabled: false,
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

test('renders Reset Complete for success and private key policy enabled during reset', () => {
  const props = makeProps();
  props.fetchAttempt = successAttempt;
  props.success = true;
  props.privateKeyPolicyEnabled = true;
  props.resetMode = true;
  render(<NewCredentials {...props} />);

  expect(screen.getByText(/Reset Complete/i)).toBeInTheDocument();
});

test('renders Registration Complete for success and private key policy enabled during registration', () => {
  const props = makeProps();
  props.fetchAttempt = { status: 'success' };
  props.success = true;
  props.privateKeyPolicyEnabled = true;
  props.resetMode = false;
  render(<NewCredentials {...props} />);

  expect(screen.getByText(/Registration Complete/i)).toBeInTheDocument();
});

test('renders Register Success on success', () => {
  const props = makeProps();
  props.fetchAttempt = { status: 'success' };
  props.privateKeyPolicyEnabled = false;
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

  expect(screen.getByText(/Set A Passwordless Device/i)).toBeInTheDocument();
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
