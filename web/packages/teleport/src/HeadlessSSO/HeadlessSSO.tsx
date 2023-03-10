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

import React, {useEffect, useState} from 'react';
import auth from 'teleport/services/auth';
import { useParams } from 'teleport/components/Router';
import HeadlessSsoDialog from 'teleport/components/HeadlessSsoDialog/HeadlessSsoDialog';
import {CardAccept, CardDenied} from "teleport/HeadlessSSO/Cards";


export function HeadlessSSO() {
  const { requestId } = useParams<{ requestId: string }>();

  const [state, setState] = useState({
    ipAddress: '',
    status: '',
    errorText: '',
    publicKey: null as PublicKeyCredentialRequestOptions,
  });

  const setIpAddress = (response: { clientIpAddress: string }) => {
    setState({
      ...state,
      status: 'loaded',
      ipAddress: response.clientIpAddress,
    });
  };

  useEffect(() => {
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
  }, []);

  const setSuccess = () => {
    setState({ ...state, status: 'success' });
  };

  const setRejected = () => {
    setState({ ...state, status: 'rejected' });
  };

  if (state.status == '') {
    return <></>;
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
    <HeadlessSsoDialog
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
