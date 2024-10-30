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

import { FC, PropsWithChildren } from 'react';
import Dialog from 'design/Dialog';
import { Attempt } from 'shared/hooks/useAsync';

import * as types from 'teleterm/ui/services/clusters/types';

import { dialogCss } from '../ClusterConnect';

import { ClusterLoginPresentationProps } from './ClusterLogin';

export const TestContainer: FC<PropsWithChildren> = ({ children }) => (
  <Dialog dialogCss={dialogCss} open={true}>
    {children}
  </Dialog>
);

export function makeProps(): ClusterLoginPresentationProps {
  return {
    shouldPromptSsoStatus: false,
    title: 'localhost',
    loginAttempt: {
      status: '',
      statusText: '',
    } as Attempt<void>,
    init: () => null,
    initAttempt: {
      status: 'success',
      statusText: '',
      data: {
        localAuthEnabled: true,
        authProviders: [],
        type: '',
        hasMessageOfTheDay: false,
        allowPasswordless: true,
        localConnectorName: '',
        authType: 'local',
      } as types.AuthSettings,
    } as const,

    loggedInUserName: null,
    onCloseDialog: () => null,
    onAbort: () => null,
    onLoginWithLocal: () => Promise.resolve<[void, Error]>([null, null]),
    onLoginWithPasswordless: () => Promise.resolve<[void, Error]>([null, null]),
    onLoginWithSso: () => null,
    clearLoginAttempt: () => null,
    passwordlessLoginState: null,
    reason: undefined,
  };
}
