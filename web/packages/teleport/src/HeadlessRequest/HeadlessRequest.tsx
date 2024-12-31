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

import HeadlessRequestDialog from 'teleport/components/HeadlessRequestDialog/HeadlessRequestDialog';
import { useParams } from 'teleport/components/Router';
import { CardAccept, CardDenied } from 'teleport/HeadlessRequest/Cards';
import auth from 'teleport/services/auth';

export function HeadlessRequest() {
  const { requestId } = useParams<{ requestId: string }>();

  const [state, setState] = useState({
    ipAddress: '',
    status: 'pending',
    errorText: '',
    publicKey: null as PublicKeyCredentialRequestOptions,
  });

  useEffect(() => {
    const setIpAddress = (response: { clientIpAddress: string }) => {
      setState({
        ...state,
        status: 'loaded',
        ipAddress: response.clientIpAddress,
      });
    };

    auth
      .headlessSSOGet(requestId)
      .then(setIpAddress)
      .catch(e => {
        setState({
          ...state,
          status: 'error',
          errorText: e.toString(),
        });
      });
  }, [requestId]);

  const setSuccess = () => {
    setState({ ...state, status: 'success' });
  };

  const setRejected = () => {
    setState({ ...state, status: 'rejected' });
  };

  if (state.status == 'pending') {
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

  if (state.status == 'success') {
    return (
      <CardAccept title="Command has been approved">
        You can now return to your terminal.
      </CardAccept>
    );
  }

  if (state.status == 'rejected') {
    return (
      <CardDenied title="Request has been rejected">
        The request has been rejected.
      </CardDenied>
    );
  }

  return (
    <HeadlessRequestDialog
      ipAddress={state.ipAddress}
      onAccept={() => {
        setState({ ...state, status: 'in-progress' });

        auth
          .headlessSSOAccept(requestId)
          .then(setSuccess)
          .catch(e => {
            setState({
              ...state,
              status: 'error',
              errorText: e.toString(),
            });
          });
      }}
      onReject={() => {
        setState({ ...state, status: 'in-progress' });

        auth
          .headlessSSOReject(requestId)
          .then(setRejected)
          .catch(e => {
            setState({
              ...state,
              status: 'error',
              errorText: e.toString(),
            });
          });
      }}
      errorText={state.errorText}
    />
  );
}

const Spin = styled(Box)`
  line-height: 12px;
  font-size: 24px;
  animation: ${rotate360} 1s linear infinite;
`;
