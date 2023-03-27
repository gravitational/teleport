/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import { Spinner } from 'design/Icon';
import { Box, Flex } from 'design';

import auth from 'teleport/services/auth';
import { useParams } from 'teleport/components/Router';
import HeadlessRequestDialog from 'teleport/components/HeadlessRequestDialog/HeadlessRequestDialog';
import { CardAccept, CardDenied } from 'teleport/HeadlessRequest/Cards';

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
  animation: spin 1s linear infinite;
  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }
`;
