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

import { useEffect, useState } from 'react';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import useAsync from 'teleterm/ui/useAsync';
import { GatewayProtocol } from 'teleterm/ui/services/clusters/types';

export default function useGatewayCreate({
  targetUri,
  onClose,
  onSuccess,
}: Props) {
  const ctx = useAppContext();
  const db = ctx.clustersService.findDb(targetUri);

  const [port, setPort] = useState('');
  const [user, setUser] = useState('');

  const [createAttempt, createGateway] = useAsync(
    (params: { port: string; user?: string }) => {
      return ctx.clustersService.createGateway({ targetUri, ...params });
    }
  );

  useEffect(() => {
    if (createAttempt.status === 'success') {
      onClose();
      if (onSuccess) {
        onSuccess(createAttempt.data.uri);
      }
    }
  }, [createAttempt.status]);

  return {
    createAttempt,
    createGateway,
    port,
    user,
    userRequired: (db.protocol as GatewayProtocol) === 'mongodb',
    db,
    onClose,
    setPort,
    setUser,
  };
}

export type Props = {
  onClose(): void;
  onSuccess?(gatewayUri: string): void;
  targetUri: string;
  port?: string;
};

export type State = ReturnType<typeof useGatewayCreate>;
