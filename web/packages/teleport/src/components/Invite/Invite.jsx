/*
Copyright 2020 Gravitational, Inc.

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
import { useAttempt, withState } from 'shared/hooks';
import cfg from 'teleport/config';
import auth from 'teleport/services/auth';
import history from 'teleport/services/history';
import { useParams } from 'react-router';
import Logger from 'shared/libs/logger';
import InviteForm, { Expired } from 'shared/components/FormInvite';
import LogoHero from './../LogoHero';

const logger = Logger.create('components/Invite');

export function Invite(props) {
  const {
    passwordResetMode = false,
    auth2faType,
    fetchAttempt,
    onSubmit,
    onSubmitWithU2f,
    passwordToken,
    submitAttempt,
  } = props;

  if (fetchAttempt.isFailed) {
    return (
      <>
        <LogoHero />
        <Expired />
      </>
    );
  }

  if (!fetchAttempt.isSuccess) {
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
        onSubmitWithU2f={onSubmitWithU2f}
        onSubmit={onSubmit}
      />
    </>
  );
}

function mapState() {
  const [fetchAttempt, fetchAttemptActions] = useAttempt();
  const [passwordToken, setPswToken] = React.useState();
  const [submitAttempt, submitAttemptActions] = useAttempt();
  const auth2faType = cfg.getAuth2faType();
  const { tokenId } = useParams();

  React.useEffect(() => {
    fetchAttemptActions.do(() => {
      return auth
        .fetchPasswordToken(tokenId)
        .then(resetToken => setPswToken(resetToken));
    });
  }, []);

  function redirect() {
    history.push(cfg.routes.app, true);
  }

  function handleSubmitError(err) {
    logger.error(err);
    submitAttemptActions.error(err);
  }

  function onSubmit(password, otpToken) {
    submitAttemptActions.start();
    auth
      .resetPassword(tokenId, password, otpToken)
      .then(redirect)
      .catch(handleSubmitError);
  }

  function onSubmitWithU2f(password) {
    submitAttemptActions.start();
    auth
      .resetPasswordWithU2f(tokenId, password)
      .then(redirect)
      .catch(handleSubmitError);
  }

  return {
    auth2faType,
    fetchAttempt,
    onSubmit,
    onSubmitWithU2f,
    passwordToken,
    submitAttempt,
    tokenId,
  };
}

const InviteWithState = withState(mapState)(Invite);

export default InviteWithState;
export const ResetPassword = () => <InviteWithState passwordResetMode={true} />;
