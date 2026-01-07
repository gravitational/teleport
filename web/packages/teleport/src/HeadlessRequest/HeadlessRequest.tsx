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

import { useEffect, useState } from 'react';
import styled from 'styled-components';

import { Box, Flex, rotate360 } from 'design';
import { Spinner } from 'design/Icon';

import AuthnDialog from 'teleport/components/AuthnDialog';
import HeadlessRequestDialog from 'teleport/components/HeadlessRequestDialog/HeadlessRequestDialog';
import { useParams } from 'teleport/components/Router';
import { CardAccept, CardDenied } from 'teleport/HeadlessRequest/Cards';
import { shouldShowMfaPrompt, useMfa } from 'teleport/lib/useMfa';
import auth from 'teleport/services/auth';
import { MfaChallengeScope } from 'teleport/services/auth/auth';
import { getActionFromAuthType, getHeadlessAuthTypeLabel, HeadlessAuthenticationType } from './types';
import { capitalizeFirstLetter } from 'shared/utils/text';
import { HeadlessAuthenticationState } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';

export function HeadlessRequest() {
  const { requestId } = useParams<{ requestId: string }>();

  const [state, setState] = useState({
    ipAddress: '',
    requestStatus: 'pending',
    errorText: '',
    publicKey: null as PublicKeyCredentialRequestOptions,
    authType: HeadlessAuthenticationType.UNSPECIFIED,
    state: HeadlessAuthenticationState.UNSPECIFIED,
  });
  const [loading, setLoading] = useState(true);
  const [browserIpAddress, setBrowserIpAddress] = useState('');

  useEffect(() => {
    const setAuthState = (response: { clientIpAddress: string, authType: number, state: number }) => {
      console.log(response);
      setState({
        ...state,
        requestStatus: 'loaded',
        ipAddress: response.clientIpAddress,
        authType: response.authType,
        state: response.state
      });
    };

    auth
      .headlessSsoGet(requestId)
      .then(setAuthState)
      .catch(e => {
        setState({
          ...state,
          errorText: e.toString(),
        });
      })
      .finally(() => setLoading(false));;
  }, [requestId]);

  useEffect(() => {
    if (
      state.authType === HeadlessAuthenticationType.BROWSER ||
      state.authType === HeadlessAuthenticationType.SESSION
    ) {
      auth.getClientIp().then(setBrowserIpAddress).catch((err) => {
        console.error('Failed to fetch browser IP address:', err);
      });
    }
  }, [state.authType])

  const mfa = useMfa({
    req: {
      scope: MfaChallengeScope.HEADLESS_LOGIN,
    },
    isMfaRequired: true,
  });

  const setSuccess = () => {
    setState({ ...state, state: HeadlessAuthenticationState.APPROVED });
  };

  const setRejected = () => {
    setState({ ...state, state: HeadlessAuthenticationState.DENIED });
  };

  if (loading) {
    return (
      <Flex
        css={`
          align-content: center;
          flex-wrap: wrap;
          justify-content: center;
          min-height: 100vh;
        `}
      >
        <Spin>
          <Spinner />
        </Spin>
      </Flex>
    );
  }

  if (state.state == HeadlessAuthenticationState.APPROVED) {
    return (
      <CardAccept title={`${capitalizeFirstLetter(getActionFromAuthType(state.authType))} has been approved`}>
        You can now return to your terminal.
      </CardAccept>
    );
  }

  if (state.state == HeadlessAuthenticationState.DENIED) {
    return (
      <CardDenied title="Request has been rejected">
        The request has been rejected.
      </CardDenied>
    );
  }

  return (
    <>
      {
        /* Show only one dialog at a time because too many dialogs can be confusing */
        shouldShowMfaPrompt(mfa) ? (
          <AuthnDialog mfaState={mfa} />
        ) : (
          <HeadlessRequestDialog
            ipAddress={state.ipAddress}
            browserIpAddress={browserIpAddress}
            onAccept={() => {
              auth
                .headlessSsoAccept(mfa, requestId)
                .then(setSuccess)
                .catch(e => {
                  setState({
                    ...state,
                    errorText: e.toString(),
                  });
                });
            }}
            onReject={() => {
              auth
                .headlessSSOReject(requestId)
                .then(setRejected)
                .catch(e => {
                  setState({
                    ...state,
                    errorText: e.toString(),
                  });
                });
            }}
            errorText={state.errorText}
            action={getActionFromAuthType(state.authType)}
          />
        )
      }
    </>
  );
}

const Spin = styled(Box)`
  line-height: 12px;
  font-size: 24px;
  animation: ${rotate360} 1s linear infinite;
`;

