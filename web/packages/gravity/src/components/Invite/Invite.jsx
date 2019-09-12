/*
Copyright 2019 Gravitational, Inc.

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
import cfg from 'gravity/config';
import auth from 'gravity/services/auth';
import history from 'gravity/services/history';
import Logger from 'shared/libs/logger';
import InviteForm, {
  Expired,
} from 'shared/components/FormInvite';
import LogoHero from './../LogoHero';

const logger = Logger.create('components/Invite');

export default function Base(props) {
  const {
    onSubmit,
    onSubmitWithU2f,
    submitBtnText,
    title,
    auth2faType,
    fetchAttempt,
    userToken,
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

  const { userName, qrCode } = userToken;
  return (
    <>
      <LogoHero />
      <InviteForm
        submitBtnText={submitBtnText}
        title={title}
        user={userName}
        qr={qrCode}
        auth2faType={auth2faType}
        attempt={submitAttempt}
        userToken={userToken}
        onSubmitWithU2f={onSubmitWithU2f}
        onSubmit={onSubmit}
      />
    </>
  );
}

function mapState(props) {
  const [fetchAttempt, fetchAttemptActions] = useAttempt();
  const [userToken, setUserToken] = React.useState();
  const [submitAttempt, submitAttemptActions] = useAttempt();
  const { tokenId, auth2faType } = props;

  React.useEffect(() => {
    fetchAttemptActions.do(() => {
      return props.onFetch(tokenId).then(userToken => setUserToken(userToken));
    });
  }, []);

  function redirect() {
    history.push(cfg.routes.app, true);
  }

  function handleSubmitError(err) {
    logger.error(err);
    submitAttemptActions.error(err);
  }

  function onSubmit(password, token) {
    submitAttemptActions.start();
    props
      .onSubmit(password, token, tokenId)
      .then(redirect)
      .fail(handleSubmitError);
  }

  function onSubmitWithU2f(password) {
    submitAttemptActions.start();
    props
      .onSubmitWithU2f(password, tokenId)
      .then(redirect)
      .fail(handleSubmitError);
  }

  return {
    fetchAttempt,
    submitAttempt,
    auth2faType,
    onSubmitWithU2f,
    onSubmit,
    tokenId,
    userToken,
  };
}

function mapInviteState(props) {
  const tokenId = props.match.params.token;
  return {
    tokenId,
    auth2faType: cfg.getAuth2faType(),
    onFetch: auth.fetchToken.bind(auth),
    onSubmitWithU2f: auth.registerWithU2F.bind(auth),
    onSubmit: auth.registerWith2FA.bind(auth),
    submitBtnText: 'Create Account',
  };
}

function mapResetState(props) {
  const tokenId = props.match.params.token;
  return {
    tokenId,
    auth2faType: cfg.getAuth2faType(),
    onFetch: auth.fetchToken.bind(auth),
    onSubmitWithU2f: auth.resetPasswordWithU2F.bind(auth),
    onSubmit: auth.resetPasswordWith2FA.bind(auth),
    submitBtnText: 'Change Password',
  };
}

const BaseWithState = withState(mapState)(Base);
const Invite = withState(mapInviteState)(BaseWithState);
const PasswordReset = withState(mapResetState)(BaseWithState);

export { Invite, PasswordReset };
