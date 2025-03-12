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

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import { HeadlessPrompt } from './HeadlessPrompt';

export default {
  title: 'Teleterm/ModalsHost/HeadlessPrompt',
};

export const Story = () => (
  <HeadlessPrompt
    cluster={makeRootCluster()}
    clientIp="localhost"
    skipConfirm={false}
    onApprove={async () => {}}
    abortApproval={() => {}}
    onReject={async () => {}}
    updateHeadlessStateAttempt={makeEmptyAttempt<void>()}
    onCancel={() => {}}
    headlessAuthenticationId="85fa45fa-57f4-5a9d-9ba8-b3cbf76d5ea2"
  />
);
