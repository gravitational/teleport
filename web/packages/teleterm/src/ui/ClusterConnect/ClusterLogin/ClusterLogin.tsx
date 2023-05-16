/*
Copyright 2019-2022 Gravitational, Inc.

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
import * as Alerts from 'design/Alert';
import { ButtonIcon, Text, Indicator, Box } from 'design';
import * as Icons from 'design/Icon';
import { DialogHeader, DialogContent } from 'design/Dialog';
import { PrimaryAuthType } from 'shared/services';

import { AuthSettings } from 'teleterm/ui/services/clusters/types';
import { ClusterConnectReason } from 'teleterm/ui/services/modals';
import { routing } from 'teleterm/ui/uri';

import LoginForm from './FormLogin';
import useClusterLogin, { State, Props } from './useClusterLogin';

export function ClusterLogin(props: Props & { reason: ClusterConnectReason }) {
  const { reason, ...otherProps } = props;
  const state = useClusterLogin(otherProps);
  return <ClusterLoginPresentation {...state} reason={reason} />;
}

export type ClusterLoginPresentationProps = State & {
  reason: ClusterConnectReason;
};

export function ClusterLoginPresentation({
  title,
  initAttempt,
  loginAttempt,
  clearLoginAttempt,
  onLoginWithLocal,
  onLoginWithPasswordless,
  onLoginWithSso,
  onCloseDialog,
  onAbort,
  loggedInUserName,
  shouldPromptSsoStatus,
  webauthnLogin,
  reason,
}: ClusterLoginPresentationProps) {
  return (
    <>
      <DialogHeader px={4} pt={4} mb={0}>
        <Text typography="h4">
          Login to <b>{title}</b>
        </Text>
        <ButtonIcon ml="auto" p={3} onClick={onCloseDialog} aria-label="Close">
          <Icons.Close fontSize="20px" />
        </ButtonIcon>
      </DialogHeader>
      <DialogContent mb={0}>
        {reason && <Reason reason={reason} />}

        {initAttempt.status === 'error' && (
          <Alerts.Danger m={4}>
            Unable to retrieve cluster auth preferences,{' '}
            {initAttempt.statusText}
          </Alerts.Danger>
        )}
        {initAttempt.status === 'processing' && (
          <Box textAlign="center" m={4}>
            <Indicator delay="none" />
          </Box>
        )}
        {initAttempt.status === 'success' && (
          <LoginForm
            {...initAttempt.data}
            primaryAuthType={getPrimaryAuthType(initAttempt.data)}
            loggedInUserName={loggedInUserName}
            onLoginWithSso={onLoginWithSso}
            onLoginWithPasswordless={onLoginWithPasswordless}
            onLogin={onLoginWithLocal}
            onAbort={onAbort}
            loginAttempt={loginAttempt}
            clearLoginAttempt={clearLoginAttempt}
            shouldPromptSsoStatus={shouldPromptSsoStatus}
            webauthnLogin={webauthnLogin}
          />
        )}
      </DialogContent>
    </>
  );
}

function getPrimaryAuthType(auth: AuthSettings): PrimaryAuthType {
  if (auth.localConnectorName === 'passwordless') {
    return 'passwordless';
  }

  const { authType } = auth;
  if (authType === 'github' || authType === 'oidc' || authType === 'saml') {
    return 'sso';
  }

  return 'local';
}

function Reason({ reason }: { reason: ClusterConnectReason }) {
  switch (reason.kind) {
    case 'reason.gateway-cert-expired': {
      const { gateway, targetUri } = reason;
      let $targetDesc: React.ReactFragment;
      if (gateway) {
        $targetDesc = (
          <>
            <strong>{gateway.targetName}</strong> as{' '}
            <strong>{gateway.targetUser}</strong>
          </>
        );
      } else {
        const targetName = routing.parseDbUri(targetUri)?.params['dbId'];

        if (targetName) {
          $targetDesc = <strong>{targetName}</strong>;
        } else {
          $targetDesc = (
            <>
              a database server under <code>{targetUri}</code>
            </>
          );
        }
      }

      return (
        <Text px={4} pt={2} mb={0}>
          You tried to connect to {$targetDesc} but your session has expired.
          Please log in to refresh the session.
        </Text>
      );
    }
    default: {
      return;
    }
  }
}
