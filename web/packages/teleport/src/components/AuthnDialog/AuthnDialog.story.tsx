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

import { makeDefaultMfaState } from 'teleport/lib/useMfa';

import AuthnDialog, { Props } from './AuthnDialog';

export default {
  title: 'Teleport/AuthnDialog',
};

export const LoadedWithMultipleOptions = () => {
  const props: Props = {
    mfaState: {
      ...makeDefaultMfaState(),
      attempt: {
        status: 'processing',
        statusText: '',
        data: null,
      },
      challenge: {
        ssoChallenge: {
          redirectUrl: 'hi',
          requestId: '123',
          channelId: '123',
          device: {
            connectorId: '123',
            connectorType: 'saml',
            displayName: 'Okta',
          },
        },
        webauthnPublicKey: {
          challenge: new ArrayBuffer(1),
        },
      },
    },
  };
  return <AuthnDialog {...props} />;
};

export const LoadedWithSingleOption = () => {
  const props: Props = {
    mfaState: {
      ...makeDefaultMfaState(),
      attempt: {
        status: 'processing',
        statusText: '',
        data: null,
      },
      challenge: {
        webauthnPublicKey: {
          challenge: new ArrayBuffer(1),
        },
      },
    },
  };
  return <AuthnDialog {...props} />;
};

export const LoadedWithError = () => {
  const err = new Error('Something went wrong');
  const props: Props = {
    mfaState: {
      ...makeDefaultMfaState(),
      attempt: {
        status: 'error',
        statusText: err.message,
        error: err,
        data: null,
      },
    },
  };
  return <AuthnDialog {...props} />;
};
