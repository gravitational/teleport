/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { useParams } from 'react-router';
import InviteForm, { Expired } from 'shared/components/FormInvite';
import LogoHero from 'teleport/components/LogoHero';
import useInvite, { State } from './useInvite';

export default function Container({ passwordResetMode = false }) {
  const { tokenId } = useParams<{ tokenId: string }>();
  const state = useInvite(tokenId);
  return <Invite {...state} passwordResetMode={passwordResetMode} />;
}

export function Invite(props: State & Props) {
  const {
    passwordResetMode,
    auth2faType,
    fetchAttempt,
    submitAttempt,
    clearSubmitAttempt,
    onSubmit,
    onSubmitWithU2f,
    passwordToken,
  } = props;

  if (fetchAttempt.status === 'failed') {
    return (
      <>
        <LogoHero />
        <Expired />
      </>
    );
  }

  if (fetchAttempt.status !== 'success') {
    return null;
  }

  const { user, qrCode } = passwordToken;
  const title = passwordResetMode ? 'Reset Password' : 'Welcome to Teleport';
  const submitBtnText = passwordResetMode
    ? 'Change Password'
    : 'Create Account';

  return (
    <>
      <LogoHero />
      <InviteForm
        submitBtnText={submitBtnText}
        title={title}
        user={user}
        qr={qrCode}
        auth2faType={auth2faType}
        attempt={submitAttempt}
        clearSubmitAttempt={clearSubmitAttempt}
        onSubmitWithU2f={onSubmitWithU2f}
        onSubmit={onSubmit}
      />
    </>
  );
}

export type Props = {
  passwordResetMode: boolean;
};

export const ResetPassword = () => <Container passwordResetMode={true} />;
