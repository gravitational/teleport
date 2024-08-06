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

import {
  makeRootCluster,
  makeLeafCluster,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

import { ReAuthenticate } from './ReAuthenticate';

export default {
  title: 'Teleterm/ModalsHost/ReAuthenticate',
};

const promptMfaRequest = {
  reason: 'MFA is required to access Kubernetes cluster "minikube"',
  clusterUri: makeRootCluster().uri,
  webauthn: false,
  totp: false,
};

export const WithWebauthn = () => (
  <MockAppContextProvider>
    <ReAuthenticate
      promptMfaRequest={{ ...promptMfaRequest, webauthn: true }}
      onCancel={() => {}}
      onSuccess={() => {
        window.alert(
          'You somehow submitted a form while only Webauthn was available.'
        );
      }}
    />
  </MockAppContextProvider>
);

const showToken = (otpToken: string) =>
  window.alert(`Submitted form with token: ${otpToken}`);

export const WithTotp = () => (
  <MockAppContextProvider>
    <ReAuthenticate
      promptMfaRequest={{ ...promptMfaRequest, totp: true }}
      onCancel={() => {}}
      onSuccess={showToken}
    />
  </MockAppContextProvider>
);

export const WithWebauthnAndTotp = () => (
  <MockAppContextProvider>
    <ReAuthenticate
      promptMfaRequest={{ ...promptMfaRequest, webauthn: true, totp: true }}
      onCancel={() => {}}
      onSuccess={showToken}
    />
  </MockAppContextProvider>
);

export const MultilineTitle = () => (
  <MockAppContextProvider>
    <ReAuthenticate
      promptMfaRequest={{
        ...promptMfaRequest,
        webauthn: true,
        totp: true,
        clusterUri: '/clusters/lorem.cloud.gravitational.io',
      }}
      onCancel={() => {}}
      onSuccess={showToken}
    />
  </MockAppContextProvider>
);

export const ForLeafCluster = () => (
  <MockAppContextProvider>
    <ReAuthenticate
      promptMfaRequest={{
        ...promptMfaRequest,
        webauthn: true,
        totp: true,
        clusterUri: makeLeafCluster().uri,
      }}
      onCancel={() => {}}
      onSuccess={showToken}
    />
  </MockAppContextProvider>
);
