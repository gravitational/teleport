/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { ArgTypes } from '@storybook/react';
import { FC, PropsWithChildren } from 'react';

import Dialog from 'design/Dialog';
import { ClientVersionStatus } from 'gen-proto-ts/teleport/lib/teleterm/v1/auth_settings_pb';

import { makeAuthSettings } from 'teleterm/services/tshd/testHelpers';

import { dialogCss } from '../spacing';
import { ClusterLoginPresentationProps } from './ClusterLogin';

export const TestContainer: FC<PropsWithChildren> = ({ children }) => (
  <Dialog dialogCss={dialogCss} open>
    {children}
  </Dialog>
);

export interface StoryProps {
  compatibility: 'compatible' | 'client-too-old' | 'client-too-new';
}

export const compatibilityArgType: ArgTypes<StoryProps> = {
  compatibility: {
    control: { type: 'radio' },
    options: ['compatible', 'client-too-old', 'client-too-new'],
    description: 'Client compatibility',
  },
};

export function makeProps(
  storyProps: StoryProps
): ClusterLoginPresentationProps {
  const props: ClusterLoginPresentationProps = {
    shouldPromptSsoStatus: false,
    title: 'localhost',
    loginAttempt: {
      status: '',
      statusText: '',
      data: undefined,
    },
    init: () => null,
    initAttempt: {
      status: 'success',
      statusText: '',
      data: makeAuthSettings(),
    },

    loggedInUserName: null,
    onCloseDialog: () => null,
    onAbort: () => null,
    onLoginWithLocal: () => Promise.resolve<[void, Error]>([null, null]),
    onLoginWithPasswordless: () => Promise.resolve<[void, Error]>([null, null]),
    onLoginWithSso: () => null,
    clearLoginAttempt: () => null,
    passwordlessLoginState: null,
    reason: undefined,
    shouldSkipVersionCheck: false,
    disableVersionCheck: () => {},
    platform: 'darwin',
  };

  switch (storyProps.compatibility) {
    case 'client-too-old':
      {
        props.initAttempt.data.clientVersionStatus =
          ClientVersionStatus.TOO_OLD;
        props.initAttempt.data.versions = {
          client: '16.0.0-dev',
          minClient: '17.0.0',
          server: '18.2.7',
        };
      }
      break;
    case 'client-too-new': {
      props.initAttempt.data.clientVersionStatus = ClientVersionStatus.TOO_NEW;
      props.initAttempt.data.versions = {
        client: '18.0.0-dev',
        minClient: '16.0.0',
        server: '17.0.0',
      };
    }
  }

  return props;
}
