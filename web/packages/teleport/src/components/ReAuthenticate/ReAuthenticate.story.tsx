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

import { makeEmptyAttempt } from 'shared/hooks/useAsync';

import {
  MFA_OPTION_SSO_DEFAULT,
  MFA_OPTION_TOTP,
  MFA_OPTION_WEBAUTHN,
} from 'teleport/services/mfa';

import { ReAuthenticate, State } from './ReAuthenticate';
import { ReauthState } from './useReAuthenticate';

export default {
  title: 'Teleport/ReAuthenticate',
};

export const Loaded = () => <ReAuthenticate {...props} />;

export const Processing = () => (
  <ReAuthenticate
    {...props}
    reauthState={{
      ...props.reauthState,
      submitAttempt: { status: 'processing', data: null, statusText: '' },
    }}
  />
);

export const Failed = () => (
  <ReAuthenticate
    {...props}
    reauthState={{
      ...props.reauthState,
      submitAttempt: {
        status: 'error',
        data: null,
        error: new Error('an error has occurred'),
        statusText: 'an error has occurred',
      },
    }}
  />
);

const props: State = {
  reauthState: {
    initAttempt: { status: 'success' },
    mfaOptions: [MFA_OPTION_WEBAUTHN, MFA_OPTION_TOTP, MFA_OPTION_SSO_DEFAULT],
    submitWithMfa: async () => null,
    submitAttempt: makeEmptyAttempt(),
    clearSubmitAttempt: () => {},
  } as ReauthState,

  onClose: () => null,
};
