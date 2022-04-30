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
import RecoveryCodes from 'teleport/components/RecoveryCodes';
import Form from './FormNewCredentials';
import Expired from './FormNewCredentials/Expired';
import useToken, { State } from './useToken';

export default function Container({
  tokenId = '',
  title = '',
  submitBtnText = '',
  resetMode = false,
}) {
  const state = useToken(tokenId);
  return (
    <NewCredentials
      {...state}
      title={title}
      submitBtnText={submitBtnText}
      resetMode={resetMode}
    />
  );
}

export function NewCredentials(props: Props) {
  const {
    submitAttempt,
    fetchAttempt,
    passwordToken,
    recoveryCodes,
    resetMode,
    redirect,
    ...rest
  } = props;

  if (fetchAttempt.status === 'failed') {
    return <Expired resetMode={resetMode} />;
  }

  if (fetchAttempt.status !== 'success') {
    return null;
  }

  if (recoveryCodes) {
    return (
      <RecoveryCodes
        recoveryCodes={recoveryCodes}
        redirect={redirect}
        isNewCodes={resetMode}
      />
    );
  }

  const { user, qrCode } = passwordToken;
  return <Form user={user} qr={qrCode} attempt={submitAttempt} {...rest} />;
}

export type Props = State & {
  submitBtnText: string;
  title: string;
  resetMode?: boolean;
};
