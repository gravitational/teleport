import React from 'react';
import { useParams } from 'react-router';
import InviteForm, { Expired } from 'shared/components/FormInvite';
import LogoHero from 'teleport/components/LogoHero';
import RecoveryCodes from 'e-teleport/components/RecoveryCodes';
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
    recoveryCodes,
    redirect,
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

  if (recoveryCodes?.length > 0) {
    return (
      <>
        <LogoHero />
        <RecoveryCodes
          recoveryCodes={recoveryCodes}
          redirect={redirect}
          isNewCodes={passwordResetMode}
        />
      </>
    );
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
