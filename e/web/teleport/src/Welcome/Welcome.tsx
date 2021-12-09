import React from 'react';
import WelcomeOSS from 'teleport/Welcome';
import {
  NewCredentials as NewCredentialsOSS,
  Props as PropsOSS,
} from 'teleport/Welcome/NewCredentials';
import RecoveryCodes from 'e-teleport/components/RecoveryCodes';
import useTokenE, { State } from './useToken';

export default function WelcomeE() {
  return <WelcomeOSS CustomForm={Container} />;
}

function Container({ tokenId = '', ...rest }: ContainerProps) {
  const state = useTokenE(tokenId);
  return <NewCredentialsE {...state} {...rest} />;
}

function NewCredentialsE(props: State & PropsOSS) {
  const { recoveryCodes, resetMode, redirect, ...rest } = props;
  if (recoveryCodes) {
    return (
      <RecoveryCodes
        recoveryCodes={recoveryCodes}
        redirect={redirect}
        isNewCodes={resetMode}
      />
    );
  }

  return <NewCredentialsOSS {...rest} />;
}

type ContainerProps = PropsOSS & {
  tokenId: string;
};
