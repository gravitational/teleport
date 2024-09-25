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

import React, { createContext, PropsWithChildren } from 'react';
import {
  createConnectTransport,
  createGrpcWebTransport,
} from '@connectrpc/connect-web';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TransportProvider } from '@connectrpc/connect-query';

import { getAuthHeaders } from 'teleport/services/api';

const RpcContext = createContext<RpcContextProps>(null);

const RpcContextProvider: React.FC<PropsWithChildren> = props => {
  const rootClusterClient = new QueryClient();
  const transport = createConnectTransport({
    useBinaryFormat: true,
    interceptors: [
      next => request => {
        const headers = getAuthHeaders();
        Object.keys(headers).forEach(k => request.header.append(k, headers[k]));
        // Add your headers here
        return next(request);
      },
    ],
    credentials: 'include',
    baseUrl: '/v1/webapi/sites/teleport.zarquon.sh/authapi/v1/grpc/',
  });
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={rootClusterClient}>
        <RpcContext.Provider
          value={{
            rootClusterClient,
          }}
          children={props.children}
        />
        ;
      </QueryClientProvider>
    </TransportProvider>
  );
};

export default RpcContextProvider;

type RpcContextProps = {
  rootClusterClient: QueryClient;
};
