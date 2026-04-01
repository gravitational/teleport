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

import React, { useState } from 'react';

import {
  Box,
  ButtonIcon,
  ButtonPrimary,
  Flex,
  H2,
  Indicator,
  P2,
  StepSlider,
  Text,
} from 'design';
import * as Alerts from 'design/Alert';
import { DialogContent, DialogHeader } from 'design/Dialog';
import * as Icons from 'design/Icon';
import { ArrowBack } from 'design/Icon';
import type { StepComponentProps } from 'design/StepSlider';
import { AuthSettings } from 'gen-proto-ts/teleport/lib/teleterm/v1/auth_settings_pb';
import { PrimaryAuthType } from 'shared/services';

import { publicAddrWithTargetPort } from 'teleterm/services/tshd/app';
import { getTargetNameFromUri } from 'teleterm/services/tshd/gateway';
import { DetailsView } from 'teleterm/ui/AppUpdater';
import { ClusterConnectReason } from 'teleterm/ui/services/modals';

import { outermostPadding } from '../spacing';
import LoginForm from './FormLogin';
import { Props, State, useClusterLogin } from './useClusterLogin';

export function ClusterLogin(props: Props & { reason: ClusterConnectReason }) {
  const { reason, ...otherProps } = props;
  const state = useClusterLogin(otherProps);
  return <ClusterLoginPresentation {...state} reason={reason} />;
}

export const ClusterLoginPresentation = (
  props: ClusterLoginPresentationProps
) => {
  const [motdAcknowledged, setMotdAcknowledged] = useState(false);
  const showMotd =
    props.initAttempt.status === 'success' &&
    !!props.initAttempt.data.messageOfTheDay &&
    !motdAcknowledged;

  if (
    props.initAttempt.status === '' ||
    props.initAttempt.status === 'processing'
  ) {
    return (
      <>
        <LoginHeader cluster={props.title} onClose={props.onCloseDialog} />
        <Box px={outermostPadding} textAlign="center">
          <Indicator delay="none" />
        </Box>
      </>
    );
  }

  if (props.initAttempt.status === 'error') {
    return (
      <>
        <LoginHeader cluster={props.title} onClose={props.onCloseDialog} />
        <Flex px={outermostPadding} flexDirection="column" gap={3}>
          <Alerts.Danger
            details={props.initAttempt.statusText}
            margin={0}
            width="100%"
          >
            Unable to retrieve cluster auth preferences
          </Alerts.Danger>
          <ButtonPrimary autoFocus={true} size="large" onClick={props.init}>
            Retry
          </ButtonPrimary>
        </Flex>
      </>
    );
  }

  if (showMotd) {
    return (
      <>
        <LoginHeader cluster={props.title} onClose={props.onCloseDialog} />
        <MessageOfTheDay
          message={props.initAttempt.data.messageOfTheDay}
          onAcknowledge={() => setMotdAcknowledged(true)}
        />
      </>
    );
  }

  return (
    <StepSlider
      flows={loginViews}
      currFlow="default"
      css={`
        // Prevents displaying a scrollbar by the slider.
        // Instead, the entire dialog should be scrollable.
        flex-shrink: 0;
      `}
      authSettings={props.initAttempt.data}
      {...props}
    />
  );
};

export type ClusterLoginPresentationProps = State & {
  reason: ClusterConnectReason;
};

export type ClusterLoginFormProps = Omit<State, 'initAttempt' | 'init'> &
  StepComponentProps & {
    reason: ClusterConnectReason;
    authSettings: AuthSettings;
  };

function ClusterLoginForm({
  title,
  authSettings,
  loginAttempt,
  clearLoginAttempt,
  onLoginWithLocal,
  onLoginWithPasswordless,
  onLoginWithSso,
  onCloseDialog,
  onAbort,
  loggedInUserName,
  ssoPrompt,
  passwordlessLoginState,
  reason,
  shouldSkipVersionCheck,
  disableVersionCheck,
  platform,
  next,
  refCallback,
  changeAppUpdatesManagingCluster,
  appUpdateEvent,
  cancelAppUpdateDownload,
  quitAndInstallAppUpdate,
  downloadAppUpdate,
  checkForAppUpdates,
}: ClusterLoginFormProps) {
  return (
    <Flex ref={refCallback} flexDirection="column">
      <LoginHeader cluster={title} onClose={onCloseDialog} />
      <DialogContent mb={0} gap={2}>
        {reason && (
          <Box px={outermostPadding}>
            <Reason reason={reason} />
          </Box>
        )}
        <LoginForm
          authSettings={authSettings}
          primaryAuthType={getPrimaryAuthType(authSettings)}
          loggedInUserName={loggedInUserName}
          onLoginWithSso={onLoginWithSso}
          onLoginWithPasswordless={onLoginWithPasswordless}
          onLogin={onLoginWithLocal}
          onAbort={onAbort}
          loginAttempt={loginAttempt}
          clearLoginAttempt={clearLoginAttempt}
          ssoPrompt={ssoPrompt}
          passwordlessLoginState={passwordlessLoginState}
          shouldSkipVersionCheck={shouldSkipVersionCheck}
          disableVersionCheck={disableVersionCheck}
          platform={platform}
          checkForAppUpdates={checkForAppUpdates}
          changeAppUpdatesManagingCluster={changeAppUpdatesManagingCluster}
          appUpdateEvent={appUpdateEvent}
          cancelAppUpdateDownload={cancelAppUpdateDownload}
          downloadAppUpdate={downloadAppUpdate}
          quitAndInstallAppUpdate={quitAndInstallAppUpdate}
          switchToAppUpdateDetails={next}
        />
      </DialogContent>
    </Flex>
  );
}

const AppUpdateDetails = ({
  refCallback,
  platform,
  currentVersion,
  downloadAppUpdate,
  checkForAppUpdates,
  cancelAppUpdateDownload,
  quitAndInstallAppUpdate,
  changeAppUpdatesManagingCluster,
  appUpdateEvent,
  onCloseDialog,
  prev,
}: ClusterLoginPresentationProps & StepComponentProps) => {
  return (
    <Flex ref={refCallback} flexDirection="column">
      <DialogHeader px={outermostPadding}>
        <Flex alignItems="center" gap={1}>
          <ButtonIcon title="Go Back" onClick={prev}>
            <ArrowBack />
          </ButtonIcon>
          <H2>App Updates</H2>
        </Flex>
        <ButtonIcon ml="auto" p={3} onClick={onCloseDialog} aria-label="Close">
          <Icons.Cross size="medium" />
        </ButtonIcon>
      </DialogHeader>
      <Flex px={4}>
        <DetailsView
          onInstall={() => quitAndInstallAppUpdate()}
          platform={platform}
          currentVersion={currentVersion}
          changeManagingCluster={clusterUri =>
            changeAppUpdatesManagingCluster(clusterUri)
          }
          updateEvent={appUpdateEvent}
          onDownload={() => downloadAppUpdate()}
          onCancelDownload={() => cancelAppUpdateDownload()}
          onCheckForUpdates={() => checkForAppUpdates()}
        />
      </Flex>
    </Flex>
  );
};

const loginViews = { default: [ClusterLoginForm, AppUpdateDetails] };

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
  const $targetDesc = getTargetDesc(reason);

  return (
    <Text>
      You tried to connect to {$targetDesc} but your session has expired. Please
      log in to refresh the session.
    </Text>
  );
}

const getTargetDesc = (reason: ClusterConnectReason): React.ReactNode => {
  switch (reason.kind) {
    case 'reason.gateway-cert-expired': {
      const { gateway, targetUri } = reason;
      if (gateway) {
        return (
          <>
            <strong>{gateway.targetName}</strong>
            {gateway.targetUser && (
              <>
                {' '}
                as <strong>{gateway.targetUser}</strong>
              </>
            )}
          </>
        );
      } else {
        return <strong>{getTargetNameFromUri(targetUri)}</strong>;
      }
    }
    case 'reason.vnet-cert-expired': {
      if (reason.routeToApp) {
        return <strong>{publicAddrWithTargetPort(reason.routeToApp)}</strong>;
      }
      return <strong>{getTargetNameFromUri(reason.targetUri)}</strong>;
    }
    default: {
      reason satisfies never;
      return;
    }
  }
};

function LoginHeader(props: { cluster: string; onClose(): void }) {
  return (
    <DialogHeader px={outermostPadding}>
      <H2>
        Log in to <b>{props.cluster}</b>
      </H2>
      <ButtonIcon ml="auto" p={3} onClick={props.onClose} aria-label="Close">
        <Icons.Cross size="medium" />
      </ButtonIcon>
    </DialogHeader>
  );
}

function MessageOfTheDay(props: { message: string; onAcknowledge(): void }) {
  return (
    <>
      {/* Make the internal container scrollable, so that the acknowledge button is always visible. */}
      <Box mb={3} maxHeight="400px" overflow="auto">
        <P2 whiteSpace="pre-wrap" px={outermostPadding}>
          {props.message}
        </P2>
      </Box>
      <ButtonPrimary
        size="large"
        mx={outermostPadding}
        autoFocus
        onClick={props.onAcknowledge}
      >
        Acknowledge
      </ButtonPrimary>
    </>
  );
}
