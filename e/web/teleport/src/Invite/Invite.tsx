import React from 'react';
import { useParams } from 'react-router';
import LogoHero from 'teleport/components/LogoHero';
import { Invite as InviteOSS } from 'teleport/Invite/Invite';
import RecoveryCodes from 'e-teleport/components/RecoveryCodes';
import useInvite, { State } from './useInvite';

export default function Container({ passwordResetMode = false }) {
  const { tokenId } = useParams<{ tokenId: string }>();
  const state = useInvite(tokenId);
  return <Invite {...state} passwordResetMode={passwordResetMode} />;
}

export function Invite(props: State & Props) {
  const { passwordResetMode, recoveryCodes, redirect } = props;

  if (recoveryCodes) {
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

  return <InviteOSS {...props} />;
}

export type Props = {
  passwordResetMode: boolean;
};

export const ResetPassword = () => <Container passwordResetMode={true} />;
